package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LawyZheng/nura/internal/pipeline"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/store"
)

// ReportIngestInput is the input for the report ingestion tool.
type ReportIngestInput struct {
	RawText    string `json:"raw_text"`
	ReportDate string `json:"report_date"`
	SourceType string `json:"source_type,omitempty"`
}

// ReportTool wraps the report ingestion pipeline as an agent tool.
type ReportTool struct {
	pipeline *pipeline.Pipeline
}

func NewReportTool(llm providers.LLMProvider, s *store.Store) *ReportTool {
	return &ReportTool{
		pipeline: pipeline.NewPipeline(llm, s),
	}
}

func (rt *ReportTool) Name() string        { return "report_ingest" }
func (rt *ReportTool) Description() string { return "Ingest a medical report: classify, extract, normalize, merge, and explain" }

func (rt *ReportTool) Schema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"raw_text":    {"type": "string", "description": "Raw text content of the medical report"},
			"report_date": {"type": "string", "description": "Date of the report (YYYY-MM-DD)"},
			"source_type": {"type": "string", "description": "Source type: photo or pdf"}
		},
		"required": ["raw_text", "report_date"]
	}`)
}

func (rt *ReportTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var in ReportIngestInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ToolResult{}, fmt.Errorf("unmarshal input: %w", err)
	}

	result, err := rt.pipeline.Run(ctx, in.RawText, in.ReportDate)
	if err != nil {
		return ToolResult{Error: err.Error()}, err
	}

	data, _ := json.Marshal(result)
	return ToolResult{
		Data:    data,
		Message: result.Explanation,
	}, nil
}
