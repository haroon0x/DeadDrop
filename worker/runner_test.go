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

func TestBuildSummaryRejectsZeroExitOutputWhenReceiptMissing(t *testing.T) {
	summary, receiptJSON, ok := buildSummary("custom", "demo", 0, "plain output")
	if ok {
		t.Fatal("expected missing receipt to be invalid")
	}
	if receiptJSON != "" {
		t.Fatalf("expected no structured receipt json, got %q", receiptJSON)
	}
	if !strings.Contains(summary, "Agent output tail:\nplain output") {
		t.Fatalf("expected output tail, got %q", summary)
	}
}

func TestBuildSummaryReportsMissingReceiptOnNonZeroExit(t *testing.T) {
	_, _, ok := buildSummary("custom", "demo", 1, "plain output")
	if ok {
		t.Fatal("expected non-zero output without receipt to remain invalid")
	}
}

func TestBuildSummaryExtractsStructuredReceiptJSON(t *testing.T) {
	output := `noise
DEADDROP_RECEIPT_JSON
{"status":"completed","summary":"Fixed bug","changed_files":["app.py"],"verification":[{"command":"pytest","status":"passed","summary":"1 passed"}],"blockers":[],"notes":"ok"}
DEADDROP_RECEIPT_JSON_END`
	summary, receiptJSON, ok := buildSummary("gemini", "default", 0, output)
	if !ok {
		t.Fatal("expected structured receipt")
	}
	if !strings.Contains(summary, "Fixed bug") {
		t.Fatalf("expected summary text, got %q", summary)
	}
	if !strings.Contains(receiptJSON, `"changed_files":["app.py"]`) {
		t.Fatalf("expected normalized receipt json, got %q", receiptJSON)
	}
}

func TestBuildSummaryRejectsInvalidStructuredReceiptJSON(t *testing.T) {
	output := `DEADDROP_RECEIPT_JSON
{"status":
DEADDROP_RECEIPT_JSON_END`
	summary, receiptJSON, ok := buildSummary("gemini", "default", 0, output)
	if ok {
		t.Fatal("expected invalid structured receipt to fail")
	}
	if receiptJSON != "" {
		t.Fatalf("expected no receipt json, got %q", receiptJSON)
	}
	if !strings.Contains(summary, "Invalid structured receipt JSON") {
		t.Fatalf("expected invalid json message, got %q", summary)
	}
}

func TestMockReceiptReportsWhetherCodeChanged(t *testing.T) {
	changed := mockReceipt(true)
	if !strings.Contains(changed, "Changed app.py") {
		t.Fatalf("expected changed receipt, got %q", changed)
	}
	unchanged := mockReceipt(false)
	if !strings.Contains(unchanged, "No deterministic app.py change was needed") {
		t.Fatalf("expected unchanged receipt, got %q", unchanged)
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

func TestValidateWorkspaceAcceptsPlainDirAndGitSubdir(t *testing.T) {
	dir := t.TempDir()
	runTestCommand(t, dir, "git", "init")
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := validateWorkspace(subdir); err != nil {
		t.Fatalf("expected git subdir workspace to pass, got %v", err)
	}
	plain := t.TempDir()
	if err := validateWorkspace(plain); err != nil {
		t.Fatalf("expected plain directory workspace to pass, got %v", err)
	}
	file := filepath.Join(plain, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateWorkspace(file); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected file workspace to fail, got %v", err)
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
