package store

import "testing"

func TestGenerateModelID(t *testing.T) {
	got := GenerateModelID("user1", "deepseek", "deepseek-v4-flash")
	want := "user1_deepseek_deepseek-v4-flash"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSlugifyModelSegment(t *testing.T) {
	if slugifyModelSegment("DeepSeek_V4.Pro") != "deepseek-v4-pro" {
		t.Fatalf("unexpected slug: %s", slugifyModelSegment("DeepSeek_V4.Pro"))
	}
}
