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
	if !isGitRepo(repo.Path) {
		return finish(cfg, c, job.ID, 1, "configured repo is not a git repo", "configured repo is not a git repo")
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
		tmpl := cfg.CommandTemplate
		if tmpl == "" {
			tmpl = `gemini --skip-trust --approval-mode yolo --output-format text -p "{{prompt}}"`
		}
		command, err = runTemplate(cfg, repo, c, job.ID, tmpl, prompt, job.Prompt)
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

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	return cmd.Run() == nil
}

func buildPrompt(repo, alias, task string) string {
	return fmt.Sprintf(`You are running inside DeadDrop, a local-first mission queue for coding agents.

Trusted local workspace repo:
- repo_alias: %s
- repo_path: %s

User task:
%s

Rules:
- Work only inside the current repository.
- Do not commit changes.
- Do not run git commit, git push, or destructive cleanup.
- Prefer the smallest useful change.
- Do not delete unrelated files.
- If you need to run tests, run the smallest relevant test first.
- If the task is a question or asks you to inspect files, answer it directly; code edits are not required.
- If the task asks for specific lines or content, provide exactly the requested information.
- If the task is ambiguous, make the safest minimal change or explain what is missing.
- The human will review the diff and can accept or reject later.
- Keep final answer concise and operational.

Your final answer can be any format that best satisfies the user task. The only required protocol is:
- Wrap your final answer exactly once between DEADDROP_RECEIPT and DEADDROP_RECEIPT_END.
- Do not print DEADDROP_RECEIPT until you are ready to give the final answer.
- If blocked, put the blocker and any useful findings inside the receipt.

DEADDROP_RECEIPT
<your final answer, audit, findings, or requested information>
DEADDROP_RECEIPT_END
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
	c.Log(jobID, "system", "Running agent command: "+command)
	if cfg.DryRun {
		return CommandResult{ExitCode: 0, Output: "Dry run: command not executed"}, nil
	}
	return streamCommand(cfg.AgentTimeout, repo.Path, c, jobID, "sh", "-c", command)
}

func logCommand(dir string, c Client, jobID int, name string, args ...string) (CommandResult, error) {
	c.Log(jobID, "system", "Running: "+name+" "+strings.Join(args, " "))
	return streamCommand(2*time.Minute, dir, c, jobID, name, args...)
}

func streamCommand(timeout time.Duration, dir string, c Client, jobID int, name string, args ...string) (CommandResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
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
	err := cmd.Wait()
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
		receipt = "Agent output tail:\n" + tail(output, 3000)
	}
	return fmt.Sprintf("%s\n\nWorker receipt:\nAgent mode: %s\nRepo alias: %s\nExit code: %d\nNo commit was created by DeadDrop. Review git diff before accepting changes locally.", receipt, agent, alias, exitCode), hasReceipt
}

func extractReceipt(output string) string {
	start := strings.Index(output, "DEADDROP_RECEIPT\n")
	end := strings.LastIndex(output, "DEADDROP_RECEIPT_END")
	if start == -1 || end == -1 || end <= start {
		return ""
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
