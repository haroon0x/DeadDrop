package main

import "testing"

func TestExtractReceiptFreeForm(t *testing.T) {
	output := "noise\nDEADDROP_RECEIPT\nline 4 says hello\nline 5 says world\nDEADDROP_RECEIPT_END\nmore noise"
	got := extractReceipt(output)
	want := "DEADDROP_RECEIPT\nline 4 says hello\nline 5 says world\nDEADDROP_RECEIPT_END"
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
