package main

import (
	"strings"
	"testing"
)

func TestEnterpriseSourcesCoverRequiredAcceptanceSurface(t *testing.T) {
	checks := map[string]string{
		"sqlite persistence":  "sqlite3",
		"summary metrics":     "/api/summary",
		"work item API":       "/api/work-items",
		"audit API":           "/api/audit",
		"health API":          "/healthz",
		"responsive frontend": "@media(max-width:900px)",
	}
	combined := backendSource + frontendSource + testSource + acceptanceSource
	for name, expected := range checks {
		if !strings.Contains(combined, expected) {
			t.Errorf("%s missing %q", name, expected)
		}
	}
	if strings.Contains(frontendSource, "fetch('/api/") {
		t.Fatal("frontend uses a root-relative API path that bypasses the preview prefix")
	}
}

func TestEnterpriseToolSequenceBuildsTestsAndPublishes(t *testing.T) {
	want := []string{"write_file", "write_file", "write_file", "write_file", "write_file", "run_command", "run_command", "run_command", "run_command", "publish_port"}
	for step, toolName := range want {
		message := nextMessage(step)
		calls, ok := message["tool_calls"].([]map[string]any)
		if !ok || len(calls) != 1 {
			t.Fatalf("step %d tool calls = %#v", step, message["tool_calls"])
		}
		function := calls[0]["function"].(map[string]any)
		if function["name"] != toolName {
			t.Fatalf("step %d tool = %v, want %s", step, function["name"], toolName)
		}
	}
	if nextMessage(len(want))["tool_calls"] != nil {
		t.Fatal("final response unexpectedly contains tool calls")
	}
}

func TestAcceptanceExercisesCreateUpdateFilterAndAudit(t *testing.T) {
	for _, expected := range []string{"-X POST", "-X PATCH", "priority=critical", "work_item.created", "work_item.updated"} {
		if !strings.Contains(acceptanceSource, expected) {
			t.Errorf("acceptance source missing %q", expected)
		}
	}
}
