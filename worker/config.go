package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type RepoConfig struct {
	Alias  string   `json:"alias"`
	Path   string   `json:"path"`
	Name   string   `json:"name"`
	Verify []string `json:"verify"`
}

type Config struct {
	Server          string
	Token           string
	Worker          string
	Repo            string
	RepoAlias       string
	Manifest        string
	Repos           map[string]RepoConfig
	Agent           string
	PollInterval    time.Duration
	AgentTimeout    time.Duration
	DryRun          bool
	RunOnce         bool
	CommandTemplate string
	VerifyCommands  []string
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	return ""
}

func (values *stringListFlag) Set(value string) error {
	if value == "" {
		return fmt.Errorf("verification command cannot be empty")
	}
	*values = append(*values, value)
	return nil
}

func parseConfig(args []string) (Config, error) {
	if len(args) == 0 || args[0] != "run" {
		return Config{}, fmt.Errorf("usage: deaddrop-worker run --server URL --token TOKEN --worker local --repo /path --repo-alias default --agent mock")
	}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	cfg := Config{}
	poll := fs.Int("poll-interval", 3, "poll interval in seconds")
	agentTimeout := fs.Int("agent-timeout", 900, "agent command timeout in seconds")
	fs.StringVar(&cfg.Server, "server", "", "server URL")
	fs.StringVar(&cfg.Token, "token", "", "worker token")
	fs.StringVar(&cfg.Worker, "worker", "local", "worker name")
	fs.StringVar(&cfg.Repo, "repo", "", "local repo path")
	fs.StringVar(&cfg.RepoAlias, "repo-alias", "default", "repo alias accepted by this worker")
	fs.StringVar(&cfg.Manifest, "manifest", "", "workspace manifest JSON path")
	fs.StringVar(&cfg.Agent, "agent", "gemini", "agent mode: mock, gemini, custom, or a preset (claude, codex, aider, cursor, opencode)")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "log command without running it")
	fs.BoolVar(&cfg.RunOnce, "run-once", false, "poll once, process at most one job, then exit")
	fs.StringVar(&cfg.CommandTemplate, "command-template", "", "custom command template")
	fs.Var((*stringListFlag)(&cfg.VerifyCommands), "verify", "verification command; repeat for multiple commands")
	if err := fs.Parse(args[1:]); err != nil {
		return Config{}, err
	}
	cfg.PollInterval = time.Duration(*poll) * time.Second
	cfg.AgentTimeout = time.Duration(*agentTimeout) * time.Second
	if cfg.Server == "" || cfg.Token == "" {
		return Config{}, fmt.Errorf("--server and --token are required")
	}
	if err := cfg.loadRepos(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg *Config) loadRepos() error {
	cfg.Repos = map[string]RepoConfig{}
	if cfg.Manifest != "" {
		data, err := os.ReadFile(cfg.Manifest)
		if err != nil {
			return fmt.Errorf("read manifest: %w", err)
		}
		var manifest struct {
			Repos []RepoConfig `json:"repos"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("parse manifest: %w", err)
		}
		base := filepath.Dir(cfg.Manifest)
		for _, repo := range manifest.Repos {
			if repo.Alias == "" || repo.Path == "" {
				return fmt.Errorf("manifest repos need alias and path")
			}
			if !filepath.IsAbs(repo.Path) {
				repo.Path = filepath.Clean(filepath.Join(base, repo.Path))
			}
			if repo.Name == "" {
				repo.Name = repo.Alias
			}
			if _, err := os.Stat(repo.Path); err != nil {
				return fmt.Errorf("repo %s path invalid: %w", repo.Alias, err)
			}
			cfg.Repos[repo.Alias] = repo
		}
	}
	if cfg.Repo != "" {
		if _, err := os.Stat(cfg.Repo); err != nil {
			return fmt.Errorf("repo path invalid: %w", err)
		}
		cfg.Repos[cfg.RepoAlias] = RepoConfig{Alias: cfg.RepoAlias, Path: cfg.Repo, Name: cfg.RepoAlias, Verify: cfg.VerifyCommands}
	}
	if len(cfg.Repos) == 0 {
		return fmt.Errorf("--manifest or --repo is required")
	}
	return nil
}
