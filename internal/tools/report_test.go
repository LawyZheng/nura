package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

func newTestStoreWithPatient(t *testing.T) (*store.Store, int) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	pid, err := s.CreatePatientProfile(&model.PatientProfile{Name: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	return s, pid
}

func TestReportTool_Interface(t *testing.T) {
	s, _ := newTestStoreWithPatient(t)
	llm := &providers.MockProvider{}
	rt := NewReportTool(llm, s, t.TempDir())

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

	var s2 map[string]any
	if err := json.Unmarshal(schema, &s2); err != nil {
		t.Errorf("schema is not valid JSON: %v", err)
	}
}

func TestReportTool_Execute(t *testing.T) {
	s, pid := newTestStoreWithPatient(t)
	llm := &providers.MockProvider{}
	rt := NewReportTool(llm, s, t.TempDir())

	input, _ := json.Marshal(ReportIngestInput{
		PatientID:  pid,
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
	s, _ := newTestStoreWithPatient(t)
	llm := &providers.MockProvider{}
	rt := NewReportTool(llm, s, t.TempDir())

	_, err := rt.Execute(context.Background(), json.RawMessage(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}
