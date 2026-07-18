package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClientBatchesLogsAndReplaysPendingResult(t *testing.T) {
	var acceptResult atomic.Bool
	var resultRequests atomic.Int32
	var logRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/worker/jobs/7/logs":
			logRequests.Add(1)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["content"] != "first\nsecond" {
				t.Errorf("unexpected batched log %q", body["content"])
			}
			w.WriteHeader(http.StatusOK)
		case "/api/worker/jobs/7/complete":
			resultRequests.Add(1)
			if !acceptResult.Load() {
				http.Error(w, "temporary", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	client.spoolDir = t.TempDir()
	client.setAttempt(7, "00000000-0000-0000-0000-000000000007")
	client.Log(7, "stdout", "first")
	client.Log(7, "stdout", "second")
	if err := client.Complete(7, 0, "done", "", "diff", "abc123"); err == nil || !strings.Contains(err.Error(), "result saved") {
		t.Fatalf("expected spooled result error, got %v", err)
	}
	if logRequests.Load() != 1 {
		t.Fatalf("expected one batched log request, got %d", logRequests.Load())
	}
	if resultRequests.Load() != requestAttempts {
		t.Fatalf("expected %d result attempts, got %d", requestAttempts, resultRequests.Load())
	}
	attemptID := "00000000-0000-0000-0000-000000000007"
	if _, err := os.Stat(client.pendingPath(7, attemptID, "complete")); err != nil {
		t.Fatal(err)
	}

	acceptResult.Store(true)
	if err := client.ReplayPending(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(client.pendingPath(7, attemptID, "complete")); !os.IsNotExist(err) {
		t.Fatalf("expected delivered result spool to be removed, got %v", err)
	}
}
