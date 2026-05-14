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
	client := NewClient(cfg.Server, cfg.Token)
	if err := registerRepos(cfg, client); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		job, err := client.Next(cfg.Worker)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
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
		code := handleJob(cfg, client, *job)
		if cfg.RunOnce {
			os.Exit(code)
		}
	}
}

func handleJob(cfg Config, client Client, job Job) int {
	result := runJob(cfg, client, job)
	if result.Err != nil || result.ExitCode != 0 {
		msg := ""
		if result.Err != nil {
			msg = result.Err.Error()
		}
		if err := client.Fail(job.ID, result.ExitCode, msg, result.Summary, result.ReceiptJSON, result.Diff); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if result.ExitCode != 0 {
			return result.ExitCode
		}
		return 1
	}
	if err := client.Complete(job.ID, result.ExitCode, result.Summary, result.ReceiptJSON, result.Diff); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func registerRepos(cfg Config, client Client) error {
	repos := make([]RepoRegistration, 0, len(cfg.Repos))
	for _, repo := range cfg.Repos {
		repos = append(repos, RepoRegistration{RepoAlias: repo.Alias, DisplayName: repo.Name})
	}
	return client.Register(cfg.Worker, repos)
}
