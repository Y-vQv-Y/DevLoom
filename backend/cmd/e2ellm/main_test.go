package main

import (
	"strings"
	"testing"
)

func TestFrontendSourceUsesPreviewRelativeAPIPaths(t *testing.T) {
	if strings.Contains(frontendSource, "fetch('/api/") {
		t.Fatal("frontendSource uses a root-relative API path that bypasses the preview prefix")
	}
	if got := strings.Count(frontendSource, "fetch('api/todos'"); got != 2 {
		t.Fatalf("relative todo API calls = %d, want 2", got)
	}
}
