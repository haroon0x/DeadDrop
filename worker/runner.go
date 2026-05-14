package main

import (
	"bufio"
	"bytes"
	"context"
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
	ExitCode int
	Summary  string
	Diff     string
	Err      error
}

type CommandResult struct {
	ExitCode int
	Output   string
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
	summary, hasReceipt := buildSummary(cfg.Agent, repo.Alias, command.ExitCode, command.Output)
	if !hasReceipt && err == nil {
		err = fmt.Errorf("agent completed but did not emit DeadDrop receipt markers")
		command.ExitCode = 2
		c.Log(job.ID, "system", err.Error())
	}
	if err != nil {
		return RunResult{ExitCode: command.ExitCode, Summary: summary, Diff: diff, Err: err}
	}
	return RunResult{ExitCode: command.ExitCode, Summary: summary, Diff: diff}
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
	return fmt.Sprintf(`# DeadDrop Worker Task

You are Gemini CLI running inside one trusted local workspace directory.

## Workspace
- Repo alias: %s
- Repo path: %s

## User Task
%s

## Operating Rules
1. Work only inside current workspace directory.
2. Do not commit, push, reset, or delete unrelated files.
3. For inspection/question tasks, answer directly; code edits are not required.
4. For change tasks, make smallest useful change and run smallest relevant verification.
5. If blocked or ambiguous, explain blocker and what you checked.
6. Human reviews git diff later if workspace is in a git repo, so keep receipt concise and operational.

## Required Final Output
At very end, print exactly this structure:

START LINE:
DEADDROP_RECEIPT

THEN:
Short result, changed files if any, verification run if any, blockers if any.

END LINE:
DEADDROP_RECEIPT_END

Do not print DEADDROP_RECEIPT until final answer.
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
	args := []string{"--skip-trust", "--approval-mode", "yolo", "--output-format", "text", "-p", prompt}
	c.Log(jobID, "system", "Running agent command: gemini --skip-trust --approval-mode yolo --output-format text -p <prompt redacted>")
	if cfg.DryRun {
		return CommandResult{ExitCode: 0, Output: "Dry run: gemini not executed"}, nil
	}
	return streamCommand(cfg.AgentTimeout, repo.Path, c, jobID, "gemini", args...)
}

func redactedCommandForLog(tmpl string, repo RepoConfig) string {
	command := strings.ReplaceAll(tmpl, "{{prompt}}", "<prompt redacted>")
	command = strings.ReplaceAll(command, "{{task}}", "<task redacted>")
	command = strings.ReplaceAll(command, "{{repo}}", repo.Path)
	return command
}

func logCommand(dir string, c Client, jobID int, name string, args ...string) (CommandResult, error) {
	c.Log(jobID, "system", "Running: "+name+" "+strings.Join(args, " "))
	return streamCommand(2*time.Minute, dir, c, jobID, name, args...)
}

func streamCommand(timeout time.Duration, dir string, c Client, jobID int, name string, args ...string) (CommandResult, error) {
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
		c.Log(jobID, "stdout", s)
		outputMu.Lock()
		output.WriteString(s + "\n")
		outputMu.Unlock()
	}, done)
	go scanPipe(stderr, func(s string) {
		c.Log(jobID, "stderr", s)
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
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		msg := fmt.Sprintf("command timed out after %s", timeout)
		c.Log(jobID, "stderr", msg)
		return CommandResult{ExitCode: 124, Output: output.String()}, fmt.Errorf("%s", msg)
	}
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return CommandResult{ExitCode: exit.ExitCode(), Output: output.String()}, err
		}
		return CommandResult{ExitCode: 1, Output: output.String()}, err
	}
	return CommandResult{ExitCode: 0, Output: output.String()}, nil
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

func buildSummary(agent, alias string, exitCode int, output string) (string, bool) {
	receipt := extractReceipt(output)
	hasReceipt := receipt != ""
	if receipt == "" {
		body := tail(output, 3000)
		if strings.TrimSpace(body) != "" && exitCode == 0 {
			receipt = "DEADDROP_RECEIPT\n" + body + "\nDEADDROP_RECEIPT_END"
			hasReceipt = true
		} else {
			receipt = "Agent output tail:\n" + body
		}
	}
	return fmt.Sprintf("%s\n\nWorker receipt:\nAgent mode: %s\nRepo alias: %s\nExit code: %d\nNo commit was created by DeadDrop. Review git diff before accepting changes locally.", receipt, agent, alias, exitCode), hasReceipt
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
	if changed {
		changeLine = "Changed app.py so add(a, b) returns a + b instead of a - b."
	}
	return `DEADDROP_RECEIPT
Completed.

` + changeLine + `
Ran pytest before and after the change.
Review git diff before committing.
DEADDROP_RECEIPT_END`
}
