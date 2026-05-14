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
	if err := validateGitRepoRoot(repo.Path); err != nil {
		return finish(cfg, c, job.ID, 1, err.Error(), err.Error())
	}
	c.Log(job.ID, "system", "Inspecting repo")
	logCommand(repo.Path, c, job.ID, "git", "status", "--short")
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

	diff := capture(repo.Path, "git", "diff")
	status := capture(repo.Path, "git", "status", "--short")
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

func validateGitRepoRoot(path string) error {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("configured repo is not a git repo")
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("configured repo root is invalid: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("configured repo path is invalid: %w", err)
	}
	configured, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return fmt.Errorf("configured repo path is invalid: %w", err)
	}
	if top != configured {
		return fmt.Errorf("configured repo path must be the git worktree root: %s", top)
	}
	return nil
}

func buildPrompt(repo, alias, task string) string {
	return fmt.Sprintf(`# DeadDrop Worker Task

You are Gemini CLI running inside one trusted local git workspace.

## Workspace
- Repo alias: %s
- Repo path: %s

## User Task
%s

## Operating Rules
1. Work only inside current repository.
2. Do not commit, push, reset, or delete unrelated files.
3. For inspection/question tasks, answer directly; code edits are not required.
4. For change tasks, make smallest useful change and run smallest relevant verification.
5. If blocked or ambiguous, explain blocker and what you checked.
6. Human reviews diff later, so keep receipt concise and operational.

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
	if strings.Contains(string(data), old) {
		c.Log(jobID, "system", "Mock agent: applying deterministic add() fix")
		if !cfg.DryRun {
			next := strings.Replace(string(data), old, "return a + b", 1)
			if err := os.WriteFile(appPath, []byte(next), 0644); err != nil {
				return CommandResult{ExitCode: 1}, err
			}
		}
	}
	c.Log(jobID, "system", "Mock agent: running verification")
	verify, err := runPythonModule(repo.Path, c, jobID, "pytest")
	verify.Output += "\n" + mockReceipt()
	return verify, err
}

func runPythonModule(dir string, c Client, jobID int, module string) (CommandResult, error) {
	if _, err := exec.LookPath("python"); err == nil {
		return logCommand(dir, c, jobID, "python", "-m", module)
	}
	return logCommand(dir, c, jobID, "python3", "-m", module)
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

func mockReceipt() string {
	return `DEADDROP_RECEIPT
Completed.

Changed app.py so add(a, b) returns a + b instead of a - b.
Ran pytest before and after the change.
Review git diff before committing.
DEADDROP_RECEIPT_END`
}
