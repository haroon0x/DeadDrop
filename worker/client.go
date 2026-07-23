package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	requestTimeout     = 10 * time.Second
	requestAttempts    = 3
	maxResponseBytes   = 1 << 20
	maxLogContentRunes = 16000
	heartbeatInterval  = 5 * time.Second
)

type Client struct {
	base     string
	token    string
	http     *http.Client
	logs     *logBuffer
	attempts *attemptStore
	spoolDir string
}

type Job struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Prompt     string `json:"prompt"`
	RepoAlias  string `json:"repo_alias"`
	WorkerName string `json:"worker_name"`
	Status     string `json:"status"`
	AttemptID  string `json:"attempt_id"`
	Agent      string `json:"agent"`
}

type RepoRegistration struct {
	RepoAlias   string `json:"repo_alias"`
	DisplayName string `json:"display_name"`
}

type logEntry struct {
	JobID     int
	AttemptID string
	Stream    string
	Content   string
}

type logBuffer struct {
	mu      sync.Mutex
	flushMu sync.Mutex
	entries []logEntry
	runes   int
}

type pendingResult struct {
	Version   int             `json:"version"`
	JobID     int             `json:"job_id"`
	AttemptID string          `json:"attempt_id"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
}

type attemptState struct {
	AttemptID       string
	CancelRequested bool
	LeaseLost       bool
}

type attemptStore struct {
	mu     sync.Mutex
	states map[int]attemptState
}

func NewClient(base, token string) Client {
	base = strings.TrimRight(base, "/")
	return Client{
		base:     base,
		token:    token,
		http:     &http.Client{Timeout: requestTimeout},
		logs:     &logBuffer{},
		attempts: &attemptStore{states: map[int]attemptState{}},
		spoolDir: resultSpoolDir(base),
	}
}

func resultSpoolDir(base string) string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	digest := sha256.Sum256([]byte(base))
	serverKey := hex.EncodeToString(digest[:8])
	return filepath.Join(configDir, "deaddrop", "pending-results", serverKey)
}

func (c Client) request(method, path string, body any, retry bool) (int, []byte, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("encode request: %w", err)
		}
	}
	attempts := 1
	if retry {
		attempts = requestAttempts
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		req, requestErr := http.NewRequest(method, c.base+path, bytes.NewReader(payload))
		if requestErr != nil {
			return 0, nil, fmt.Errorf("create request: %w", requestErr)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, requestErr := c.http.Do(req)
		if requestErr != nil {
			lastErr = requestErr
		} else {
			data, responseErr := readResponse(res)
			if responseErr == nil && (!retry || !retryableStatus(res.StatusCode) || attempt == attempts) {
				return res.StatusCode, data, nil
			}
			if responseErr != nil {
				lastErr = responseErr
			} else {
				lastErr = fmt.Errorf("server returned %s: %s", res.Status, strings.TrimSpace(string(data)))
			}
		}
		if attempt < attempts {
			time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
		}
	}
	return 0, nil, fmt.Errorf("request failed after %d attempts: %w", attempts, lastErr)
}

func readResponse(res *http.Response) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	closeErr := res.Body.Close()
	if len(data) > maxResponseBytes {
		readErr = fmt.Errorf("response exceeded %d bytes", maxResponseBytes)
	}
	return data, errors.Join(readErr, closeErr)
}

func retryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func statusError(operation string, status int, data []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}
	return fmt.Errorf("%s failed: HTTP %d: %s", operation, status, strings.TrimSpace(string(data)))
}

func (c Client) Next(worker string) (*Job, error) {
	status, data, err := c.request("GET", "/api/worker/next?worker_name="+url.QueryEscape(worker), nil, false)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNoContent {
		return nil, nil
	}
	if err := statusError("next", status, data); err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("decode next job: %w", err)
	}
	if job.AttemptID == "" {
		return nil, fmt.Errorf("next job response is missing attempt_id")
	}
	c.setAttempt(job.ID, job.AttemptID)
	return &job, nil
}

func (c Client) Register(worker string, repos []RepoRegistration) error {
	status, data, err := c.request("POST", "/api/worker/register", map[string]any{"worker_name": worker, "repos": repos}, true)
	if err != nil {
		return err
	}
	return statusError("register", status, data)
}

func (c Client) Log(jobID int, stream, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	attemptID, err := c.attemptID(jobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log delivery skipped: %v\n", err)
		return
	}
	c.logs.mu.Lock()
	c.logs.entries = append(c.logs.entries, logEntry{JobID: jobID, AttemptID: attemptID, Stream: stream, Content: content})
	c.logs.runes += len([]rune(content))
	flush := stream == "system" || c.logs.runes >= maxLogContentRunes
	c.logs.mu.Unlock()
	if flush {
		if err := c.FlushLogs(jobID); err != nil {
			fmt.Fprintf(os.Stderr, "log delivery delayed: %v\n", err)
		}
	}
}

func (c Client) FlushLogs(jobID int) error {
	c.logs.flushMu.Lock()
	defer c.logs.flushMu.Unlock()
	entries := c.takeLogs(jobID)
	if len(entries) == 0 {
		return nil
	}
	batches := batchLogs(entries)
	for index, entry := range batches {
		status, data, err := c.request("POST", fmt.Sprintf("/api/worker/jobs/%d/logs", jobID), map[string]string{"attempt_id": entry.AttemptID, "stream": entry.Stream, "content": entry.Content}, true)
		if err == nil {
			err = statusError("log", status, data)
		}
		if err != nil {
			c.prependLogs(batches[index:])
			return err
		}
	}
	return nil
}

func (c Client) takeLogs(jobID int) []logEntry {
	c.logs.mu.Lock()
	defer c.logs.mu.Unlock()
	selected := make([]logEntry, 0, len(c.logs.entries))
	remaining := make([]logEntry, 0, len(c.logs.entries))
	remainingRunes := 0
	for _, entry := range c.logs.entries {
		if entry.JobID == jobID {
			selected = append(selected, entry)
		} else {
			remaining = append(remaining, entry)
			remainingRunes += len([]rune(entry.Content))
		}
	}
	c.logs.entries = remaining
	c.logs.runes = remainingRunes
	return selected
}

func (c Client) prependLogs(entries []logEntry) {
	c.logs.mu.Lock()
	defer c.logs.mu.Unlock()
	next := make([]logEntry, 0, len(entries)+len(c.logs.entries))
	next = append(next, entries...)
	next = append(next, c.logs.entries...)
	c.logs.entries = next
	for _, entry := range entries {
		c.logs.runes += len([]rune(entry.Content))
	}
}

func batchLogs(entries []logEntry) []logEntry {
	var batches []logEntry
	for _, entry := range entries {
		runes := []rune(entry.Content)
		for len(runes) > 0 {
			limit := len(runes)
			if limit > maxLogContentRunes {
				limit = maxLogContentRunes
			}
			chunk := string(runes[:limit])
			runes = runes[limit:]
			last := len(batches) - 1
			if last >= 0 && batches[last].JobID == entry.JobID && batches[last].AttemptID == entry.AttemptID && batches[last].Stream == entry.Stream {
				combined := []rune(batches[last].Content + "\n" + chunk)
				if len(combined) <= maxLogContentRunes {
					batches[last].Content = string(combined)
					continue
				}
			}
			batches = append(batches, logEntry{JobID: entry.JobID, AttemptID: entry.AttemptID, Stream: entry.Stream, Content: chunk})
		}
	}
	return batches
}

func (c Client) Complete(jobID, exitCode int, summary, receiptJSON, diff, baseCommit string) error {
	attemptID, err := c.attemptID(jobID)
	if err != nil {
		return err
	}
	payload := map[string]any{"attempt_id": attemptID, "exit_code": exitCode, "final_summary": summary, "receipt_json": receiptJSON, "git_diff": diff, "baseline_commit": baseCommit}
	return c.finishJob(jobID, "complete", payload)
}

func (c Client) Fail(jobID, exitCode int, message, summary, receiptJSON, diff, baseCommit string) error {
	attemptID, err := c.attemptID(jobID)
	if err != nil {
		return err
	}
	payload := map[string]any{"attempt_id": attemptID, "exit_code": exitCode, "error_message": message, "final_summary": summary, "receipt_json": receiptJSON, "git_diff": diff, "baseline_commit": baseCommit}
	return c.finishJob(jobID, "fail", payload)
}

func (c Client) Cancelled(jobID, exitCode int, summary, receiptJSON, diff, baseCommit string) error {
	attemptID, err := c.attemptID(jobID)
	if err != nil {
		return err
	}
	payload := map[string]any{"attempt_id": attemptID, "exit_code": exitCode, "final_summary": summary, "receipt_json": receiptJSON, "git_diff": diff, "baseline_commit": baseCommit}
	return c.finishJob(jobID, "cancelled", payload)
}

func (c Client) finishJob(jobID int, kind string, payload any) error {
	attemptID, attemptErr := payloadAttemptID(payload)
	if attemptErr != nil {
		return attemptErr
	}
	if err := c.FlushLogs(jobID); err != nil {
		fmt.Fprintf(os.Stderr, "final log delivery failed: %v\n", err)
	}
	path := fmt.Sprintf("/api/worker/jobs/%d/%s", jobID, kind)
	status, data, err := c.request("POST", path, payload, true)
	if err == nil {
		err = statusError(kind, status, data)
	}
	if err == nil {
		if removeErr := c.removePending(jobID, attemptID, kind); removeErr != nil {
			return removeErr
		}
		c.clearAttempt(jobID)
		return nil
	}
	spoolPath, spoolErr := c.spoolResult(jobID, kind, payload)
	if spoolErr != nil {
		return errors.Join(err, fmt.Errorf("save pending result: %w", spoolErr))
	}
	if permanentResultStatus(status) {
		rejectedPath, rejectErr := rejectPending(spoolPath)
		if rejectErr != nil {
			return errors.Join(err, fmt.Errorf("preserve rejected result: %w", rejectErr))
		}
		c.clearAttempt(jobID)
		return fmt.Errorf("%w; rejected result preserved at %s", err, rejectedPath)
	}
	return fmt.Errorf("%w; result saved to %s", err, spoolPath)
}

func (c Client) spoolResult(jobID int, kind string, payload any) (string, error) {
	if c.spoolDir == "" {
		return "", fmt.Errorf("user config directory is unavailable")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	attemptID, err := payloadAttemptID(payload)
	if err != nil {
		return "", err
	}
	result := pendingResult{Version: 1, JobID: jobID, AttemptID: attemptID, Kind: kind, Payload: raw}
	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(c.spoolDir, 0700); err != nil {
		return "", err
	}
	path := c.pendingPath(jobID, attemptID, kind)
	temp, err := os.CreateTemp(c.spoolDir, ".pending-*")
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return "", err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return "", err
	}
	ok = true
	return path, nil
}

func (c Client) ReplayPending() error {
	if c.spoolDir == "" {
		return nil
	}
	files, err := os.ReadDir(c.spoolDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pending results: %w", err)
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		path := filepath.Join(c.spoolDir, file.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read pending result %s: %w", path, err)
		}
		var result pendingResult
		if err := json.Unmarshal(data, &result); err != nil {
			return fmt.Errorf("decode pending result %s: %w", path, err)
		}
		if result.Version != 1 || result.JobID <= 0 || result.AttemptID == "" || (result.Kind != "complete" && result.Kind != "fail" && result.Kind != "cancelled") || !json.Valid(result.Payload) {
			return fmt.Errorf("pending result %s is invalid", path)
		}
		payloadAttempt, attemptErr := payloadAttemptID(result.Payload)
		if attemptErr != nil || payloadAttempt != result.AttemptID {
			return fmt.Errorf("pending result %s has inconsistent attempt identity", path)
		}
		status, response, err := c.request("POST", fmt.Sprintf("/api/worker/jobs/%d/%s", result.JobID, result.Kind), result.Payload, true)
		if err == nil {
			err = statusError("replay "+result.Kind, status, response)
		}
		if err != nil && permanentResultStatus(status) {
			rejectedPath, rejectErr := rejectPending(path)
			if rejectErr != nil {
				return fmt.Errorf("preserve rejected result %s: %w", path, rejectErr)
			}
			fmt.Fprintf(os.Stderr, "pending result rejected by server and preserved at %s: %v\n", rejectedPath, err)
			continue
		}
		if err != nil {
			return fmt.Errorf("replay pending result %s: %w", path, err)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove delivered result %s: %w", path, err)
		}
	}
	return nil
}

func (c Client) pendingPath(jobID int, attemptID, kind string) string {
	return filepath.Join(c.spoolDir, fmt.Sprintf("job-%d-%s-%s.json", jobID, attemptID, kind))
}

func (c Client) removePending(jobID int, attemptID, kind string) error {
	if c.spoolDir == "" {
		return nil
	}
	err := os.Remove(c.pendingPath(jobID, attemptID, kind))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func payloadAttemptID(payload any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode result attempt: %w", err)
	}
	var body struct {
		AttemptID string `json:"attempt_id"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return "", fmt.Errorf("decode result attempt: %w", err)
	}
	if body.AttemptID == "" {
		return "", fmt.Errorf("result payload is missing attempt_id")
	}
	return body.AttemptID, nil
}

func permanentResultStatus(status int) bool {
	if status < 400 || status >= 500 {
		return false
	}
	return status != http.StatusUnauthorized && status != http.StatusForbidden && status != http.StatusRequestTimeout && status != http.StatusTooManyRequests
}

func rejectPending(path string) (string, error) {
	rejectedPath := strings.TrimSuffix(path, ".json") + ".rejected"
	if err := os.Rename(path, rejectedPath); err != nil {
		return "", err
	}
	return rejectedPath, nil
}

func (c Client) setAttempt(jobID int, attemptID string) {
	c.attempts.mu.Lock()
	defer c.attempts.mu.Unlock()
	c.attempts.states[jobID] = attemptState{AttemptID: attemptID}
}

func (c Client) attemptID(jobID int) (string, error) {
	c.attempts.mu.Lock()
	defer c.attempts.mu.Unlock()
	state, ok := c.attempts.states[jobID]
	if !ok || state.AttemptID == "" {
		return "", fmt.Errorf("job %d has no active attempt", jobID)
	}
	return state.AttemptID, nil
}

func (c Client) clearAttempt(jobID int) {
	c.attempts.mu.Lock()
	defer c.attempts.mu.Unlock()
	delete(c.attempts.states, jobID)
}

func (c Client) updateAttempt(jobID int, update func(*attemptState)) {
	c.attempts.mu.Lock()
	defer c.attempts.mu.Unlock()
	state, ok := c.attempts.states[jobID]
	if !ok {
		return
	}
	update(&state)
	c.attempts.states[jobID] = state
}

func (c Client) CancelRequested(jobID int) bool {
	c.attempts.mu.Lock()
	defer c.attempts.mu.Unlock()
	return c.attempts.states[jobID].CancelRequested
}

func (c Client) StartHeartbeat(jobID int, cancel context.CancelFunc) func() {
	ctx, stop := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		failures := 0
		for {
			cancelRequested, err := c.heartbeat(jobID)
			if err != nil {
				failures++
				fmt.Fprintf(os.Stderr, "heartbeat failed for job %d: %v\n", jobID, err)
				if failures >= 3 {
					c.updateAttempt(jobID, func(state *attemptState) { state.LeaseLost = true })
					cancel()
					return
				}
			} else {
				failures = 0
				if cancelRequested {
					c.updateAttempt(jobID, func(state *attemptState) { state.CancelRequested = true })
					cancel()
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return stop
}

func (c Client) heartbeat(jobID int) (bool, error) {
	attemptID, err := c.attemptID(jobID)
	if err != nil {
		return false, err
	}
	status, data, err := c.request("POST", fmt.Sprintf("/api/worker/jobs/%d/heartbeat", jobID), map[string]string{"attempt_id": attemptID}, false)
	if err != nil {
		return false, err
	}
	if err := statusError("heartbeat", status, data); err != nil {
		return false, err
	}
	var response struct {
		CancelRequested bool `json:"cancel_requested"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return false, fmt.Errorf("decode heartbeat: %w", err)
	}
	return response.CancelRequested, nil
}
