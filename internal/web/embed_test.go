package web

import (
	"bytes"
	"strings"
	"testing"
)

func TestLayout_ContainsThemeToggle(t *testing.T) {
	var buf bytes.Buffer
	err := Render(&buf, "index.html", map[string]any{
		"Patients": []any{},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, `id="theme-toggle"`) {
		t.Error("layout missing theme-toggle button")
	}
	if !strings.Contains(html, `id="font-toggle"`) {
		t.Error("layout missing font-toggle button")
	}
	if !strings.Contains(html, "v0.4") {
		t.Error("layout should show version v0.4")
	}
}
