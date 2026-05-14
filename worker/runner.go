package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type RunResult struct {
	ExitCode    int
	Summary     string
	ReceiptJSON string
	Diff        string
	Err         error
}

type CommandResult struct {
	ExitCode int
	Output   string
}

type GeminiJSONOutput struct {
	Response string          `json:"response"`
	Stats    json.RawMessage `json:"stats"`
	Error    json.RawMessage `json:"error"`
}

func runJob(cfg Config, c Client, job Job) RunResult {
	c.Log(job.ID, "system", "Picked up job")
	repo, ok := cfg.Repos[job.RepoAlias]
	if !ok {
		msg := fmt.Sprintf("job repo_alias %q is not in worker manifest", job.RepoAlias)
		return finish(cfg, c, job.ID, 1, msg, msg)
	}
	if err := validateWorkspace(repo.Path); err != nil {
		return finish(cfg, c, job.ID, 1, err.Error(), err.Error())
	}
	c.Log(job.ID, "system", "Inspecting workspace")
	logGitStatus(repo.Path, c, job.ID)
	prompt := buildPrompt(repo.Path, repo.Alias, job.Prompt)

	var command CommandResult
	var err error
	switch cfg.Agent {
	case "mock":
		command, err = runMock(cfg, repo, c, job.ID)
	case "gemini":
		if cfg.CommandTemplate == "" {
			command, err = runGemini(cfg, repo, c, job.ID, prompt)
		} else {
			command, err = runTemplate(cfg, repo, c, job.ID, cfg.CommandTemplate, prompt, job.Prompt)
		}
	case "custom":
		if cfg.CommandTemplate == "" {
			return finish(cfg, c, job.ID, 1, "--command-template is required for custom agent", "")
		}
		command, err = runTemplate(cfg, repo, c, job.ID, cfg.CommandTemplate, prompt, job.Prompt)
	default:
		return finish(cfg, c, job.ID, 1, "unknown agent mode: "+cfg.Agent, "")
	}

	diff := captureGitDiff(repo.Path)
	status := captureGitStatus(repo.Path)
	c.Log(job.ID, "system", "Final git status:\n"+status)
	receiptSource := command.Output
	if cfg.Agent == "gemini" && cfg.CommandTemplate == "" {
		var parseErr error
		receiptSource, parseErr = geminiResponseText(command.Output)
		if parseErr != nil && err == nil {
			err = parseErr
			command.ExitCode = 2
			c.Log(job.ID, "system", parseErr.Error())
		}
	}
	summary, receiptJSON, hasReceipt := buildSummary(cfg.Agent, repo.Alias, command.ExitCode, receiptSource)
	if !hasReceipt && err == nil {
		err = fmt.Errorf("agent command exited 0, but final structured receipt JSON was missing or invalid")
		command.ExitCode = 2
		c.Log(job.ID, "system", err.Error())
	}
	if err != nil {
		return RunResult{ExitCode: command.ExitCode, Summary: summary, ReceiptJSON: receiptJSON, Diff: diff, Err: err}
	}
	return RunResult{ExitCode: command.ExitCode, Summary: summary, ReceiptJSON: receiptJSON, Diff: diff}
}

func finish(cfg Config, c Client, jobID, code int, message, summary string) RunResult {
	c.Log(jobID, "system", message)
	return RunResult{ExitCode: code, Summary: summary, Diff: "", Err: fmt.Errorf("%s", message)}
}

func validateWorkspace(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("configured workspace path is invalid: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("configured workspace path is not a directory")
	}
	_, err = filepath.Abs(path)
	return err
}

func buildPrompt(repo, alias, task string) string {
	return fmt.Sprintf(`# DeadDrop Agent Task

You are the coding agent for DeadDrop. You are running in a local terminal with the workspace directory as your current working directory. Complete the user's task, then return one structured receipt.

## Workspace
- Repo alias: %s
- Workspace path: %s

## User Task
%s

## Work Rules
1. Stay inside the current workspace. Do not read or edit files outside it unless the user explicitly asks and it is necessary.
2. Do not run git commit, git push, destructive resets, or broad delete commands.
3. If the task is a question or inspection request, answer it directly. Do not edit files unless the user asked for edits.
4. If the task asks for changes, make the smallest useful change and preserve existing style.
5. Run focused verification when practical. Prefer project-local tests, type checks, linters, or a direct command that proves the change.
6. If verification is not possible, say why in the receipt and set verification status to not_run.
7. If blocked by missing files, missing credentials, unclear instructions, or unsafe scope, stop and report blockers. Do not invent success.
8. Keep terminal output useful: run commands normally so stdout/stderr can stream to DeadDrop logs.
9. Human reviews any captured diff later, so do not include huge logs or full diffs in the receipt.

## Required Final Output
At the very end, print exactly this structure and nothing after it:

START LINE:
DEADDROP_RECEIPT_JSON

THEN:
A single valid JSON object, no markdown fences, matching this schema:
{
  "status": "completed" | "failed" | "blocked",
  "summary": "One concise plain-English result or answer.",
  "changed_files": ["relative/path.ext"],
  "verification": [
    {"command": "pytest", "status": "passed" | "failed" | "not_run", "summary": "short outcome"}
  ],
  "blockers": ["short blocker if any"],
  "notes": "Optional concise notes."
}

END LINE:
DEADDROP_RECEIPT_JSON_END

Rules:
- Output JSON only between markers.
- Use [] for no changed files, no verification, or no blockers.
- Use workspace-relative paths in changed_files.
- If command output matters, summarize it in verification or notes; raw output is already in DeadDrop logs.
- If status is "completed", summary must state what was completed or answered.
- Do not print DEADDROP_RECEIPT_JSON until final answer.
`, alias, repo, task)
}

func runMock(cfg Config, repo RepoConfig, c Client, jobID int) (CommandResult, error) {
	c.Log(jobID, "system", "Mock agent: inspecting repo")
	if _, err := os.Stat(filepath.Join(repo.Path, "test_app.py")); err == nil {
		c.Log(jobID, "system", "Mock agent: running tests")
		runPythonModule(repo.Path, c, jobID, "pytest")
	}
	appPath := filepath.Join(repo.Path, "app.py")
	data, err := os.ReadFile(appPath)
	if err != nil {
		return CommandResult{ExitCode: 1}, err
	}
	old := "return a - b"
	changed := false
	if strings.Contains(string(data), old) {
		c.Log(jobID, "system", "Mock agent: applying deterministic add() fix")
		if !cfg.DryRun {
			next := strings.Replace(string(data), old, "return a + b", 1)
			if err := os.WriteFile(appPath, []byte(next), 0644); err != nil {
				return CommandResult{ExitCode: 1}, err
			}
			changed = true
		}
	}
	c.Log(jobID, "system", "Mock agent: running verification")
	verify, err := runPythonModule(repo.Path, c, jobID, "pytest")
	verify.Output += "\n" + mockReceipt(changed)
	return verify, err
}

func runPythonModule(dir string, c Client, jobID int, module string) (CommandResult, error) {
	if _, err := exec.LookPath("python"); err == nil {
		return logCommand(dir, c, jobID, "python", "-m", module)
	}
	return logCommand(dir, c, jobID, "python3", "-m", module)
}

func logGitStatus(dir string, c Client, jobID int) {
	if !isGitWorktree(dir) {
		c.Log(jobID, "system", "Workspace is not inside a git worktree; git status/diff capture disabled")
		return
	}
	logCommand(dir, c, jobID, "git", "status", "--short", "--", ".")
}

func runTemplate(cfg Config, repo RepoConfig, c Client, jobID int, tmpl, prompt, task string) (CommandResult, error) {
	command := replaceTemplate(tmpl, "prompt", prompt)
	command = replaceTemplate(command, "task", task)
	command = replaceTemplate(command, "repo", repo.Path)
	c.Log(jobID, "system", "Running agent command: "+redactedCommandForLog(tmpl, repo))
	if cfg.DryRun {
		return CommandResult{ExitCode: 0, Output: "Dry run: command not executed"}, nil
	}
	return streamCommand(cfg.AgentTimeout, repo.Path, c, jobID, "sh", "-c", command)
}

func runGemini(cfg Config, repo RepoConfig, c Client, jobID int, prompt string) (CommandResult, error) {
	args := []string{"--skip-trust", "--approval-mode", "yolo", "--output-format", "json", "-p", prompt}
	c.Log(jobID, "system", "Running agent command: gemini --skip-trust --approval-mode yolo --output-format json -p <prompt redacted>")
	if cfg.DryRun {
		return CommandResult{ExitCode: 0, Output: "Dry run: gemini not executed"}, nil
	}
	return streamCommandWithOptions(cfg.AgentTimeout, repo.Path, c, jobID, false, true, "gemini", args...)
}

func geminiResponseText(output string) (string, error) {
	var parsed GeminiJSONOutput
	jsonBody := extractTrailingJSONObject(output)
	if err := json.Unmarshal([]byte(jsonBody), &parsed); err != nil {
		return output, fmt.Errorf("gemini returned non-JSON output despite --output-format json: %w", err)
	}
	if len(parsed.Error) > 0 && string(parsed.Error) != "null" {
		return parsed.Response, fmt.Errorf("gemini returned error: %s", string(parsed.Error))
	}
	if strings.TrimSpace(parsed.Response) == "" {
		return parsed.Response, fmt.Errorf("gemini JSON response was empty")
	}
	return parsed.Response, nil
}

func extractTrailingJSONObject(output string) string {
	body := strings.TrimSpace(output)
	if json.Valid([]byte(body)) {
		return body
	}
	start := strings.LastIndex(body, "\n{")
	if start == -1 {
		start = strings.Index(body, "{")
	} else {
		start++
	}
	if start == -1 {
		return body
	}
	return strings.TrimSpace(body[start:])
}

func redactedCommandForLog(tmpl string, repo RepoConfig) string {
	command := strings.ReplaceAll(tmpl, "{{prompt}}", "<prompt redacted>")
	command = strings.ReplaceAll(command, "{{task}}", "<task redacted>")
	command = strings.ReplaceAll(command, "{{repo}}", repo.Path)
	return command
}

func logCommand(dir string, c Client, jobID int, name string, args ...string) (CommandResult, error) {
	c.Log(jobID, "system", "Running in "+dir+": "+name+" "+strings.Join(args, " "))
	logLocal("job id=%d running in %s: %s %s", jobID, dir, name, strings.Join(args, " "))
	return streamCommand(2*time.Minute, dir, c, jobID, name, args...)
}

func streamCommand(timeout time.Duration, dir string, c Client, jobID int, name string, args ...string) (CommandResult, error) {
	return streamCommandWithOptions(timeout, dir, c, jobID, true, false, name, args...)
}

func streamCommandWithOptions(timeout time.Duration, dir string, c Client, jobID int, logStdout, filterGeminiNoise bool, name string, args ...string) (CommandResult, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		c.Log(jobID, "stderr", err.Error())
		return CommandResult{ExitCode: 1}, err
	}
	var output bytes.Buffer
	var outputMu sync.Mutex
	done := make(chan struct{}, 2)
	go scanPipe(stdout, func(s string) {
		if logStdout {
			c.Log(jobID, "stdout", s)
			logLocal("job id=%d stdout: %s", jobID, s)
		}
		outputMu.Lock()
		output.WriteString(s + "\n")
		outputMu.Unlock()
	}, done)
	go scanPipe(stderr, func(s string) {
		if filterGeminiNoise && isGeminiStartupNoise(s) {
			outputMu.Lock()
			output.WriteString(s + "\n")
			outputMu.Unlock()
			return
		}
		c.Log(jobID, "stderr", s)
		logLocal("job id=%d stderr: %s", jobID, s)
		outputMu.Lock()
		output.WriteString(s + "\n")
		outputMu.Unlock()
	}, done)
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	var err error
	select {
	case err = <-waitDone:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		err = <-waitDone
	}
	<-done
	<-done
	elapsed := time.Since(start).Round(time.Second)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		msg := fmt.Sprintf("command timed out after %s", timeout)
		c.Log(jobID, "stderr", msg)
		c.Log(jobID, "system", fmt.Sprintf("Command finished: exit code 124 after %s", elapsed))
		logLocal("job id=%d command finished: exit code 124 after %s", jobID, elapsed)
		return CommandResult{ExitCode: 124, Output: output.String()}, fmt.Errorf("%s", msg)
	}
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			c.Log(jobID, "system", fmt.Sprintf("Command finished: exit code %d after %s", exit.ExitCode(), elapsed))
			logLocal("job id=%d command finished: exit code %d after %s", jobID, exit.ExitCode(), elapsed)
			return CommandResult{ExitCode: exit.ExitCode(), Output: output.String()}, err
		}
		c.Log(jobID, "system", fmt.Sprintf("Command finished: exit code 1 after %s", elapsed))
		logLocal("job id=%d command finished: exit code 1 after %s", jobID, elapsed)
		return CommandResult{ExitCode: 1, Output: output.String()}, err
	}
	if !logStdout && strings.TrimSpace(output.String()) != "" {
		c.Log(jobID, "system", "Command stdout captured for structured parsing")
		logLocal("job id=%d stdout captured for structured parsing (%d bytes)", jobID, output.Len())
	}
	c.Log(jobID, "system", fmt.Sprintf("Command finished: exit code 0 after %s", elapsed))
	logLocal("job id=%d command finished: exit code 0 after %s", jobID, elapsed)
	return CommandResult{ExitCode: 0, Output: output.String()}, nil
}

func isGeminiStartupNoise(line string) bool {
	return strings.Contains(line, "[ExtensionManager] Error loading agent from") ||
		strings.Contains(line, "(Local Agent) tools: Expected array, received string") ||
		strings.Contains(line, "Configuration file not found at /home/g/.gemini/extensions/") ||
		strings.Contains(line, "YOLO mode is enabled. All tool calls will be automatically approved.") ||
		strings.Contains(line, "Ripgrep is not available. Falling back to GrepTool.")
}

func scanPipe(pipe any, log func(string), done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	scanner := bufio.NewScanner(pipe.(io.Reader))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		log(scanner.Text())
	}
}

func capture(dir, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()
	return out.String()
}

func isGitWorktree(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func captureGitStatus(dir string) string {
	if !isGitWorktree(dir) {
		return "Workspace is not inside a git worktree; no git status available."
	}
	return capture(dir, "git", "status", "--short", "--", ".")
}

func captureGitDiff(dir string) string {
	if !isGitWorktree(dir) {
		return ""
	}
	return capture(dir, "git", "diff", "--", ".")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func replaceTemplate(tmpl, key, value string) string {
	quoted := shellQuote(value)
	tmpl = strings.ReplaceAll(tmpl, `"`+"{{"+key+"}}"+`"`, quoted)
	tmpl = strings.ReplaceAll(tmpl, `'{{`+key+`}}'`, quoted)
	return strings.ReplaceAll(tmpl, "{{"+key+"}}", quoted)
}

type Receipt struct {
	Status       string         `json:"status"`
	Summary      string         `json:"summary"`
	ChangedFiles []string       `json:"changed_files"`
	Verification []Verification `json:"verification"`
	Blockers     []string       `json:"blockers"`
	Notes        string         `json:"notes"`
}

type Verification struct {
	Command string `json:"command"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

func buildSummary(agent, alias string, exitCode int, output string) (string, string, bool) {
	if strings.Contains(output, "DEADDROP_RECEIPT_JSON") {
		if receiptJSON, receipt, ok := extractReceiptJSON(output); ok {
			summary := receipt.Summary
			if strings.TrimSpace(summary) == "" {
				summary = "Agent returned a structured receipt."
			}
			return fmt.Sprintf("%s\n\nWorker receipt:\nAgent mode: %s\nRepo alias: %s\nExit code: %d\nNo commit was created by DeadDrop. Review git diff before accepting changes locally.", summary, agent, alias, exitCode), receiptJSON, true
		}
		return fmt.Sprintf("Invalid structured receipt JSON.\n\nAgent output tail:\n%s", tail(output, 3000)), "", false
	}
	if receiptJSON, receipt, ok := extractBareReceiptJSON(output); ok {
		summary := receipt.Summary
		if strings.TrimSpace(summary) == "" {
			summary = "Agent returned a structured receipt."
		}
		return fmt.Sprintf("%s\n\nWorker receipt:\nAgent mode: %s\nRepo alias: %s\nExit code: %d\nNo commit was created by DeadDrop. Review git diff before accepting changes locally.", summary, agent, alias, exitCode), receiptJSON, true
	}
	receipt := extractReceipt(output)
	hasReceipt := receipt != ""
	if receipt == "" {
		body := tail(output, 3000)
		receipt = "Agent output tail:\n" + body
	}
	return fmt.Sprintf("%s\n\nWorker receipt:\nAgent mode: %s\nRepo alias: %s\nExit code: %d\nNo commit was created by DeadDrop. Review git diff before accepting changes locally.", receipt, agent, alias, exitCode), "", hasReceipt
}

func extractReceiptJSON(output string) (string, Receipt, bool) {
	var receipt Receipt
	start := strings.LastIndex(output, "DEADDROP_RECEIPT_JSON\n")
	end := strings.LastIndex(output, "DEADDROP_RECEIPT_JSON_END")
	if start == -1 || end == -1 || end <= start {
		return "", receipt, false
	}
	body := strings.TrimSpace(output[start+len("DEADDROP_RECEIPT_JSON\n") : end])
	if body == "" {
		return "", receipt, false
	}
	return normalizeReceiptJSON(body)
}

func extractBareReceiptJSON(output string) (string, Receipt, bool) {
	body := strings.TrimSpace(output)
	if strings.HasPrefix(body, "```") {
		lines := strings.Split(body, "\n")
		if len(lines) >= 3 {
			body = strings.Join(lines[1:len(lines)-1], "\n")
			body = strings.TrimSpace(body)
		}
	}
	return normalizeReceiptJSON(body)
}

func normalizeReceiptJSON(body string) (string, Receipt, bool) {
	var receipt Receipt
	if err := json.Unmarshal([]byte(body), &receipt); err != nil {
		return "", receipt, false
	}
	if strings.TrimSpace(receipt.Summary) == "" && len(receipt.ChangedFiles) == 0 && len(receipt.Verification) == 0 && len(receipt.Blockers) == 0 && strings.TrimSpace(receipt.Notes) == "" {
		return "", receipt, false
	}
	if strings.TrimSpace(receipt.Status) == "" {
		receipt.Status = "completed"
	}
	normalized, err := json.Marshal(receipt)
	if err != nil {
		return "", receipt, false
	}
	return string(normalized), receipt, true
}

func extractReceipt(output string) string {
	start := strings.Index(output, "DEADDROP_RECEIPT\n")
	end := strings.LastIndex(output, "DEADDROP_RECEIPT_END")
	if start == -1 {
		return ""
	}
	if end == -1 || end <= start {
		return strings.TrimSpace(output[start:])
	}
	return strings.TrimSpace(output[start : end+len("DEADDROP_RECEIPT_END")])
}

func tail(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func mockReceipt(changed bool) string {
	changeLine := "No deterministic app.py change was needed."
	files := "[]"
	if changed {
		changeLine = "Changed app.py so add(a, b) returns a + b instead of a - b."
		files = `["app.py"]`
	}
	return `DEADDROP_RECEIPT_JSON
{
  "status": "completed",
  "summary": "` + changeLine + `",
  "changed_files": ` + files + `,
  "verification": [
    {"command": "pytest", "status": "passed", "summary": "Ran before and after the change."}
  ],
  "blockers": [],
  "notes": "No commit was created."
}
DEADDROP_RECEIPT_JSON_END`
}
