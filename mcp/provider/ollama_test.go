package provider

import (
	"strings"
	"testing"
)

func TestTruncateOllamaUserPrompt(t *testing.T) {
	system := strings.Repeat("s", 100)
	user := strings.Repeat("u", 50000)
	out := truncateOllamaUserPrompt(system, user, 8192, 1024)
	if !strings.HasPrefix(out, ollamaPromptTruncateNote) {
		t.Fatalf("expected truncation note prefix")
	}
	if utf8Count(out) >= utf8Count(user) {
		t.Fatalf("expected truncated prompt to be shorter than original")
	}
}

func utf8Count(s string) int {
	return len([]rune(s))
}
