package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	if os.Getuid() == 0 {
		fmt.Fprintln(os.Stderr, "Error: For security, DeadDrop worker must NOT be run as root.")
		os.Exit(1)
	}
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	logLocal("starting worker server=%s worker=%s agent=%s poll=%s run_once=%t", cfg.Server, cfg.Worker, cfg.Agent, cfg.PollInterval, cfg.RunOnce)
	for alias, repo := range cfg.Repos {
		logLocal("workspace alias=%s path=%s", alias, repo.Path)
	}
	client := NewClient(cfg.Server, cfg.Token)
	logLocal("registering workspace aliases")
	if err := registerRepos(cfg, client); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logLocal("registered; waiting for jobs")
	for {
		job, err := client.Next(cfg.Worker)
		if err != nil {
			logLocal("worker API error: %v", err)
			if cfg.RunOnce {
				os.Exit(1)
			}
			time.Sleep(cfg.PollInterval)
			continue
		}
		if job == nil {
			if cfg.RunOnce {
				return
			}
			time.Sleep(cfg.PollInterval)
			continue
		}
		logLocal("claimed job id=%d title=%q repo_alias=%s", job.ID, job.Title, job.RepoAlias)
		code := handleJob(cfg, client, *job)
		logLocal("job id=%d finished with worker exit code %d", job.ID, code)
		if cfg.RunOnce {
			os.Exit(code)
		}
	}
}

func handleJob(cfg Config, client Client, job Job) int {
	logLocal("running job id=%d", job.ID)
	result := runJob(cfg, client, job)
	if result.Err != nil || result.ExitCode != 0 {
		msg := ""
		if result.Err != nil {
			msg = result.Err.Error()
		}
		logLocal("reporting failed job id=%d exit_code=%d error=%q", job.ID, result.ExitCode, msg)
		client.Log(job.ID, "system", fmt.Sprintf("Reporting failed result to server: exit code %d", result.ExitCode))
		if err := client.Fail(job.ID, result.ExitCode, msg, result.Summary, result.ReceiptJSON, result.Diff); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		client.Log(job.ID, "system", "Server accepted failed result")
		if result.ExitCode != 0 {
			return result.ExitCode
		}
		return 1
	}
	logLocal("reporting completed job id=%d exit_code=%d", job.ID, result.ExitCode)
	client.Log(job.ID, "system", fmt.Sprintf("Reporting completed result to server: exit code %d", result.ExitCode))
	if err := client.Complete(job.ID, result.ExitCode, result.Summary, result.ReceiptJSON, result.Diff); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	client.Log(job.ID, "system", "Server accepted completed result")
	return 0
}

func logLocal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%s] "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}

func registerRepos(cfg Config, client Client) error {
	repos := make([]RepoRegistration, 0, len(cfg.Repos))
	for _, repo := range cfg.Repos {
		repos = append(repos, RepoRegistration{RepoAlias: repo.Alias, DisplayName: repo.Name})
	}
	return client.Register(cfg.Worker, repos)
}
