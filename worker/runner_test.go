package main

import (
	"encoding/json"
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
	authoritative, err := authoritativeReceiptJSON(receiptJSON, 0, []string{"actual.go"}, []Verification{{Command: "go test ./...", Status: "passed", Summary: "Worker observed exit code 0"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(authoritative, "app.py") || !strings.Contains(authoritative, `"changed_files":["actual.go"]`) || !strings.Contains(authoritative, `"command":"go test ./..."`) {
		t.Fatalf("expected worker evidence to replace agent claims, got %q", authoritative)
	}
}

func TestBuildSummaryUsesLastStructuredReceiptJSON(t *testing.T) {
	output := `START LINE:
DEADDROP_RECEIPT_JSON

THEN:
A single valid JSON object

END LINE:
DEADDROP_RECEIPT_JSON_END

real final answer
DEADDROP_RECEIPT_JSON
{
  "status": "completed",
  "summary": "Identified multiple hardcoded values and fixed tests.",
  "changed_files": [
    "memanto/app/services/session_service.py",
    "tests/test_api.py",
    "tests/test_unit.py"
  ],
  "verification": [
    {
      "command": "pytest tests/test_unit.py",
      "status": "passed",
      "summary": "14 unit tests passed."
    }
  ],
  "blockers": [],
  "notes": "No commit was created."
}
DEADDROP_RECEIPT_JSON_END`
	summary, receiptJSON, ok := buildSummary("gemini", "default", 0, output)
	if !ok {
		t.Fatal("expected final structured receipt")
	}
	if !strings.Contains(summary, "Identified multiple hardcoded values") {
		t.Fatalf("expected final receipt summary, got %q", summary)
	}
	if !strings.Contains(receiptJSON, `"memanto/app/services/session_service.py"`) {
		t.Fatalf("expected normalized final receipt json, got %q", receiptJSON)
	}
}

func TestBuildSummaryAcceptsBareReceiptJSON(t *testing.T) {
	output := `{"status":"completed","summary":"Answered question","changed_files":[],"verification":[],"blockers":[],"notes":"No files changed."}`
	summary, receiptJSON, ok := buildSummary("gemini", "default", 0, output)
	if !ok {
		t.Fatal("expected bare structured receipt")
	}
	if !strings.Contains(summary, "Answered question") {
		t.Fatalf("expected summary text, got %q", summary)
	}
	if !strings.Contains(receiptJSON, `"summary":"Answered question"`) {
		t.Fatalf("expected normalized receipt json, got %q", receiptJSON)
	}
}

func TestBuildSummaryAcceptsFencedBareReceiptJSON(t *testing.T) {
	output := "```json\n{\"status\":\"completed\",\"summary\":\"Done\",\"changed_files\":[],\"verification\":[],\"blockers\":[],\"notes\":\"\"}\n```"
	_, _, ok := buildSummary("gemini", "default", 0, output)
	if !ok {
		t.Fatal("expected fenced receipt json")
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

func TestGeminiResponseTextExtractsResponseFromJSONOutput(t *testing.T) {
	output := `{"response":"DEADDROP_RECEIPT_JSON\n{\"status\":\"completed\",\"summary\":\"ok\",\"changed_files\":[],\"verification\":[],\"blockers\":[],\"notes\":\"\"}\nDEADDROP_RECEIPT_JSON_END","stats":{"latency":123}}`
	response, err := geminiResponseText(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(response, "DEADDROP_RECEIPT_JSON") || !strings.Contains(response, `"summary":"ok"`) {
		t.Fatalf("expected response text, got %q", response)
	}
}

func TestGeminiResponseTextIgnoresWarningsBeforeJSONOutput(t *testing.T) {
	output := "Warning: noisy extension warning\n[WARN] another warning\n{\"response\":\"ok\",\"stats\":{\"tokens\":1}}"
	response, err := geminiResponseText(output)
	if err != nil {
		t.Fatal(err)
	}
	if response != "ok" {
		t.Fatalf("expected ok, got %q", response)
	}
}

func TestGeminiResponseTextReportsCLIError(t *testing.T) {
	_, err := geminiResponseText(`{"response":"","error":{"message":"bad key"}}`)
	if err == nil || !strings.Contains(err.Error(), "gemini returned error") {
		t.Fatalf("expected gemini error, got %v", err)
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

func TestInitManifestCreatesRunnableConfig(t *testing.T) {
	repo := t.TempDir()
	runTestCommand(t, repo, "git", "init")
	output := filepath.Join(t.TempDir(), "manifest.json")
	if err := initManifest([]string{"--repo", repo, "--output", output, "--verify", "go test ./..."}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Repos []RepoConfig `json:"repos"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Repos) != 1 || manifest.Repos[0].Alias != "default" || manifest.Repos[0].Path != repo || strings.Join(manifest.Repos[0].Verify, ",") != "go test ./..." {
		t.Fatalf("unexpected manifest %+v", manifest.Repos)
	}
}

func TestJobWorkspaceCapturesPatchWithoutChangingSource(t *testing.T) {
	dir := t.TempDir()
	runTestCommand(t, dir, "git", "init")
	runTestCommand(t, dir, "git", "config", "user.email", "test@example.com")
	runTestCommand(t, dir, "git", "config", "user.name", "Test")
	configuredPath := filepath.Join(dir, "project")
	if err := os.Mkdir(configuredPath, 0755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(configuredPath, "app.txt")
	if err := os.WriteFile(sourcePath, []byte("before\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, dir, "git", "add", "project/app.txt")
	runTestCommand(t, dir, "git", "commit", "-m", "baseline")

	workspace, err := prepareJobWorkspace(configuredPath, 42)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cleanupJobWorkspace(workspace); err != nil {
			t.Fatal(err)
		}
	})
	if err := os.WriteFile(filepath.Join(workspace.WorkspacePath, "app.txt"), []byte("after\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.WorkspacePath, "new.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatal(err)
	}

	diff, err := captureGitDiff(workspace.WorkspacePath, workspace.BaseCommit)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "+after") || !strings.Contains(diff, "new.txt") {
		t.Fatalf("expected tracked and untracked changes in patch, got %q", diff)
	}
	if !strings.HasSuffix(diff, "\n") {
		t.Fatalf("expected patch to retain its trailing newline, got %q", diff)
	}
	changedFiles, err := captureChangedFiles(workspace.WorkspacePath, workspace.BaseCommit)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(changedFiles, ",") != "app.txt,new.txt" {
		t.Fatalf("unexpected changed files %v", changedFiles)
	}
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != "before\n" {
		t.Fatalf("source workspace changed: %q", source)
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
