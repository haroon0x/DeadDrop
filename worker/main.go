package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
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
			time.Sleep(cfg.PollInterval)
			continue
		}
		if job == nil {
			time.Sleep(cfg.PollInterval)
			continue
		}
		result := runJob(cfg, client, *job)
		if result.Err != nil || result.ExitCode != 0 {
			msg := ""
			if result.Err != nil {
				msg = result.Err.Error()
			}
			if err := client.Fail(job.ID, result.ExitCode, msg, result.Summary, result.Diff); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			continue
		}
		if err := client.Complete(job.ID, result.ExitCode, result.Summary, result.Diff); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func registerRepos(cfg Config, client Client) error {
	repos := make([]RepoRegistration, 0, len(cfg.Repos))
	for _, repo := range cfg.Repos {
		repos = append(repos, RepoRegistration{RepoAlias: repo.Alias, DisplayName: repo.Name})
	}
	return client.Register(cfg.Worker, repos)
}
