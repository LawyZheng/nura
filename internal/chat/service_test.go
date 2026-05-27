package chat

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LawyZheng/nura/internal/model"
	"github.com/LawyZheng/nura/internal/policy"
	"github.com/LawyZheng/nura/internal/providers"
	"github.com/LawyZheng/nura/internal/rag"
	"github.com/LawyZheng/nura/internal/store"
)

func knowledgeDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "knowledge")
}

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	ks, err := rag.LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatalf("load knowledge: %v", err)
	}

	mock := &providers.MockProvider{}
	mock.CompleteFunc = func(_ context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
		return &providers.CompletionResponse{
			Content:    "Mock reply for: " + req.UserPrompt,
			StopReason: "end_turn",
			Usage:      providers.Usage{InputTokens: 10, OutputTokens: 20},
		}, nil
	}

	svc := NewService(mock, policy.NewEngine(), ks, s)
	return svc, s
}

func createPatient(t *testing.T, s *store.Store) int {
	t.Helper()
	// SYNTHETIC DATA - not real patient information
	id, err := s.CreatePatientProfile(&model.PatientProfile{
		Name:   "Test Patient",
		Gender: "male",
	})
	if err != nil {
		t.Fatalf("create patient: %v", err)
	}
	return id
}

func TestService_LowRisk_NoRAG(t *testing.T) {
	svc, s := newTestService(t)
	pid := createPatient(t, s)

	reply, err := svc.HandleMessage(context.Background(), pid, "HP呼气试验是什么意思")
	if err != nil {
		t.Fatal(err)
	}
	if reply.Disclaimer == "" {
		t.Error("expected disclaimer for low-risk message")
	}
	if len(reply.Sources) != 0 {
		t.Errorf("expected no sources for low-risk, got %v", reply.Sources)
	}
	if reply.RiskLevel != "low" {
		t.Errorf("risk_level = %q, want 'low'", reply.RiskLevel)
	}
}

func TestService_HighRisk_WithRAG(t *testing.T) {
	svc, s := newTestService(t)
	pid := createPatient(t, s)

	reply, err := svc.HandleMessage(context.Background(), pid, "奥美拉唑能不能停药")
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Sources) == 0 {
		t.Error("expected RAG sources for high-risk medication query")
	}
	if reply.RiskLevel != "high" {
		t.Errorf("risk_level = %q, want 'high'", reply.RiskLevel)
	}
	if reply.Disclaimer == "" {
		t.Error("expected disclaimer")
	}
}

func TestService_Emergency(t *testing.T) {
	svc, s := newTestService(t)
	pid := createPatient(t, s)

	reply, err := svc.HandleMessage(context.Background(), pid, "我吐血了")
	if err != nil {
		t.Fatal(err)
	}
	if reply.RiskLevel != "emergency" {
		t.Errorf("risk_level = %q, want 'emergency'", reply.RiskLevel)
	}
	if !strings.Contains(reply.Content, "立即就医") && !strings.Contains(reply.Content, "急救") {
		t.Errorf("expected emergency guidance in content, got: %s", reply.Content)
	}
}

func TestService_ConversationHistory(t *testing.T) {
	svc, s := newTestService(t)
	pid := createPatient(t, s)

	// Send two messages to build history.
	_, err := svc.HandleMessage(context.Background(), pid, "你好")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.HandleMessage(context.Background(), pid, "谢谢")
	if err != nil {
		t.Fatal(err)
	}

	// Verify messages are persisted.
	msgs, err := s.ListRecentChatMessages(pid, 10)
	if err != nil {
		t.Fatal(err)
	}
	// 2 user messages + 2 assistant replies = 4.
	if len(msgs) != 4 {
		t.Errorf("expected 4 persisted messages, got %d", len(msgs))
	}
}

func TestService_HistoryInjectedIntoPrompt(t *testing.T) {
	dir := t.TempDir()
	s, err := store.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ks, err := rag.LoadFromDir(knowledgeDir())
	if err != nil {
		t.Fatal(err)
	}

	var capturedSystem string
	mock := &providers.MockProvider{}
	mock.CompleteFunc = func(_ context.Context, req providers.CompletionRequest) (*providers.CompletionResponse, error) {
		capturedSystem = req.SystemPrompt
		return &providers.CompletionResponse{
			Content:    "Mock reply",
			StopReason: "end_turn",
		}, nil
	}

	svc := NewService(mock, policy.NewEngine(), ks, s)
	// SYNTHETIC DATA - not real patient information
	pid, _ := s.CreatePatientProfile(&model.PatientProfile{Name: "Test"})

	// Insert a prior message.
	s.InsertChatMessage(&model.ChatMessage{PatientID: pid, Role: "user", Content: "earlier question"})
	s.InsertChatMessage(&model.ChatMessage{PatientID: pid, Role: "assistant", Content: "earlier answer"})

	_, err = svc.HandleMessage(context.Background(), pid, "follow up")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(capturedSystem, "earlier question") {
		t.Error("expected conversation history in system prompt")
	}
	if !strings.Contains(capturedSystem, "earlier answer") {
		t.Error("expected prior assistant reply in system prompt")
	}
}
