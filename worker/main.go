package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println(version)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := initManifest(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	if runningAsRoot() {
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
		if err := client.ReplayPending(); err != nil {
			logLocal("pending result replay failed: %v", err)
			if cfg.RunOnce {
				os.Exit(1)
			}
			time.Sleep(cfg.PollInterval)
			continue
		}
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

func initManifest(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	repo := fs.String("repo", "", "Git workspace path")
	output := fs.String("output", "deaddrop.manifest.json", "manifest output path")
	name := fs.String("name", "", "workspace display name")
	var verify stringListFlag
	fs.Var(&verify, "verify", "verification command; repeat for multiple commands")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *repo == "" {
		return fmt.Errorf("--repo is required")
	}
	resolved, err := filepath.Abs(*repo)
	if err != nil {
		return fmt.Errorf("resolve repo: %w", err)
	}
	resolved, err = filepath.EvalSymlinks(resolved)
	if err != nil {
		return fmt.Errorf("resolve repo: %w", err)
	}
	if err := validateWorkspace(resolved); err != nil {
		return err
	}
	if _, err := gitRoot(resolved); err != nil {
		return fmt.Errorf("workspace must be inside a Git worktree: %w", err)
	}
	displayName := *name
	if displayName == "" {
		displayName = filepath.Base(resolved)
	}
	manifest := struct {
		Repos []RepoConfig `json:"repos"`
	}{Repos: []RepoConfig{{Alias: "default", Name: displayName, Path: resolved, Verify: verify}}}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	data = append(data, '\n')
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close manifest: %w", err)
	}
	fmt.Printf("Created %s for %s\n", *output, resolved)
	return nil
}

func handleJob(cfg Config, client Client, job Job) int {
	logLocal("running job id=%d", job.ID)
	ctx, cancel := context.WithCancel(context.Background())
	stopHeartbeat := client.StartHeartbeat(job.ID, cancel)
	result := runJob(ctx, cfg, client, job)
	stopHeartbeat()
	cancel()
	if client.CancelRequested(job.ID) {
		client.Log(job.ID, "system", "Reporting cancelled result to server")
		if err := client.Cancelled(job.ID, result.ExitCode, result.Summary, result.ReceiptJSON, result.Diff); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		logLocal("server accepted cancelled result for job id=%d", job.ID)
		return 0
	}
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
		logLocal("server accepted failed result for job id=%d", job.ID)
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
	logLocal("server accepted completed result for job id=%d", job.ID)
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
