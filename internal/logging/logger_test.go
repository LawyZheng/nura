package logging

import (
	"testing"
)

func TestNewLoggerLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		logger, err := NewLogger(level, "console")
		if err != nil {
			t.Errorf("NewLogger(%q, console) returned error: %v", level, err)
			continue
		}
		if logger == nil {
			t.Errorf("NewLogger(%q, console) returned nil logger", level)
		}
	}
}

func TestNewLoggerFormats(t *testing.T) {
	for _, format := range []string{"console", "json"} {
		logger, err := NewLogger("info", format)
		if err != nil {
			t.Errorf("NewLogger(info, %q) returned error: %v", format, err)
			continue
		}
		if logger == nil {
			t.Errorf("NewLogger(info, %q) returned nil logger", format)
		}
	}
}

func TestNewLoggerInvalidLevel(t *testing.T) {
	_, err := NewLogger("invalid", "console")
	if err == nil {
		t.Error("expected error for invalid level, got nil")
	}
}

func TestNewLoggerInvalidFormat(t *testing.T) {
	_, err := NewLogger("info", "xml")
	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}
}

func TestNewNop(t *testing.T) {
	logger := NewNop()
	if logger == nil {
		t.Error("NewNop returned nil")
	}
	// Should not panic when used.
	logger.Info("this should be silently discarded")
}
