package biz

import (
	"context"
	"strings"
	"testing"

	"github.com/Y-vQv-Y/DevLoom/backend/consts"
)

func TestOpenSourceTaskHookProvidesDeliveryPrompts(t *testing.T) {
	hook := &taskhook{}
	tests := []struct {
		name     string
		taskType consts.TaskType
		subType  consts.TaskSubType
		want     string
	}{
		{name: "requirement", taskType: consts.TaskTypeDesign, subType: consts.TaskSubTypeGenerateRequirement, want: "acceptance criteria"},
		{name: "design", taskType: consts.TaskTypeDesign, subType: consts.TaskSubTypeGenerateDesign, want: "rollback"},
		{name: "task list", taskType: consts.TaskTypeDesign, subType: consts.TaskSubTypeGenerateTasklist, want: "dependencies"},
		{name: "implementation", taskType: consts.TaskTypeDevelop, subType: consts.TaskSubTypeExecuteTask, want: "isolated work branch"},
		{name: "general development", taskType: consts.TaskTypeDevelop, want: "protected base branch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := hook.GetSystemPrompt(context.Background(), tt.taskType, tt.subType)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(prompt, tt.want) {
				t.Fatalf("prompt %q does not contain %q", prompt, tt.want)
			}
		})
	}
}
