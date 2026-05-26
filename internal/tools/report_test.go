package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

func TestReportTool_Interface(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	llm := &providers.MockProvider{}
	rt := NewReportTool(llm, s)

	// Verify it satisfies the Tool interface.
	var _ Tool = rt

	if rt.Name() != "report_ingest" {
		t.Errorf("name = %q, want 'report_ingest'", rt.Name())
	}
	if rt.Description() == "" {
		t.Error("expected non-empty description")
	}

	schema := rt.Schema()
	if len(schema) == 0 {
		t.Error("expected non-empty schema")
	}

	// Verify schema is valid JSON.
	var s2 map[string]any
	if err := json.Unmarshal(schema, &s2); err != nil {
		t.Errorf("schema is not valid JSON: %v", err)
	}
}

func TestReportTool_Execute(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	llm := &providers.MockProvider{}
	rt := NewReportTool(llm, s)

	input, _ := json.Marshal(ReportIngestInput{
		RawText:    "test gastroscopy report",
		ReportDate: "2024-03-01",
	})

	result, err := rt.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
}

func TestReportTool_InvalidInput(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	llm := &providers.MockProvider{}
	rt := NewReportTool(llm, s)

	_, err = rt.Execute(context.Background(), json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}
