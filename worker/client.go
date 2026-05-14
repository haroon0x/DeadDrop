package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	base  string
	token string
	http  *http.Client
}

type Job struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Prompt     string `json:"prompt"`
	RepoAlias  string `json:"repo_alias"`
	WorkerName string `json:"worker_name"`
	Status     string `json:"status"`
}

type RepoRegistration struct {
	RepoAlias   string `json:"repo_alias"`
	DisplayName string `json:"display_name"`
}

func NewClient(base, token string) Client {
	return Client{base: strings.TrimRight(base, "/"), token: token, http: http.DefaultClient}
}

func (c Client) request(method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func (c Client) Next(worker string) (*Job, error) {
	res, err := c.request("GET", "/api/worker/next?worker_name="+url.QueryEscape(worker), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if res.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("next failed: %s %s", res.Status, string(data))
	}
	var job Job
	return &job, json.NewDecoder(res.Body).Decode(&job)
}

func (c Client) Register(worker string, repos []RepoRegistration) error {
	res, err := c.request("POST", "/api/worker/register", map[string]any{"worker_name": worker, "repos": repos})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return fmt.Errorf("register failed: %s %s", res.Status, string(data))
	}
	return nil
}

func (c Client) Log(jobID int, stream, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	_, _ = c.request("POST", fmt.Sprintf("/api/worker/jobs/%d/logs", jobID), map[string]string{"stream": stream, "content": content})
}

func (c Client) Complete(jobID, exitCode int, summary, diff string) error {
	res, err := c.request("POST", fmt.Sprintf("/api/worker/jobs/%d/complete", jobID), map[string]any{
		"exit_code": exitCode, "final_summary": summary, "git_diff": diff,
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return fmt.Errorf("complete failed: %s %s", res.Status, string(data))
	}
	return nil
}

func (c Client) Fail(jobID, exitCode int, message, summary, diff string) error {
	res, err := c.request("POST", fmt.Sprintf("/api/worker/jobs/%d/fail", jobID), map[string]any{
		"exit_code": exitCode, "error_message": message, "final_summary": summary, "git_diff": diff,
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		data, _ := io.ReadAll(res.Body)
		return fmt.Errorf("fail failed: %s %s", res.Status, string(data))
	}
	return nil
}
