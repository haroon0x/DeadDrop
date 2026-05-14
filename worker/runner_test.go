package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractReceiptFreeForm(t *testing.T) {
	output := "noise\nDEADDROP_RECEIPT\nline 4 says hello\nline 5 says world\nDEADDROP_RECEIPT_END\nmore noise"
	got := extractReceipt(output)
	want := "DEADDROP_RECEIPT\nline 4 says hello\nline 5 says world\nDEADDROP_RECEIPT_END"
	if got != want {
		t.Fatalf("receipt mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestExtractReceiptAcceptsMissingEndMarker(t *testing.T) {
	output := "noise\nDEADDROP_RECEIPT\nlisted files\nREADME.md\n"
	got := extractReceipt(output)
	want := "DEADDROP_RECEIPT\nlisted files\nREADME.md"
	if got != want {
		t.Fatalf("receipt mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestBuildSummaryReportsMissingReceipt(t *testing.T) {
	_, ok := buildSummary("custom", "demo", 0, "plain output")
	if ok {
		t.Fatal("expected missing receipt")
	}
}

func TestRedactedCommandForLogHidesPromptAndTask(t *testing.T) {
	got := redactedCommandForLog(`agent --prompt "{{prompt}}" --task '{{task}}' --repo "{{repo}}"`, RepoConfig{
		Alias: "demo",
		Path:  "/tmp/demo",
		Name:  "Demo",
	})
	if strings.Contains(got, "{{prompt}}") || strings.Contains(got, "{{task}}") {
		t.Fatalf("template placeholders leaked: %q", got)
	}
	if !strings.Contains(got, "<prompt redacted>") || !strings.Contains(got, "<task redacted>") {
		t.Fatalf("expected redacted placeholders, got %q", got)
	}
	if !strings.Contains(got, "/tmp/demo") {
		t.Fatalf("expected repo path to remain visible, got %q", got)
	}
}

func TestValidateGitRepoRootRejectsSubdir(t *testing.T) {
	dir := t.TempDir()
	runTestCommand(t, dir, "git", "init")
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	err := validateGitRepoRoot(subdir)
	if err == nil || !strings.Contains(err.Error(), "git worktree root") {
		t.Fatalf("expected git worktree root error, got %v", err)
	}
	if err := validateGitRepoRoot(dir); err != nil {
		t.Fatalf("expected repo root to pass, got %v", err)
	}
}

func TestParseConfigRunOnce(t *testing.T) {
	dir := t.TempDir()
	runTestCommand(t, dir, "git", "init")
	cfg, err := parseConfig([]string{
		"run",
		"--server", "http://localhost:8000",
		"--token", "worker",
		"--repo", dir,
		"--run-once",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.RunOnce {
		t.Fatal("expected run-once to be enabled")
	}
}

func runTestCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
