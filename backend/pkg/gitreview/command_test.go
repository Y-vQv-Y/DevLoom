package gitreview

import "testing"

func TestParseMention(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    Command
		detail  string
		ok      bool
	}{
		{name: "bare mention", comment: "Please @DevLoom", want: CommandReview, ok: true},
		{name: "fix command", comment: "@devloom /fix the failing test", want: CommandFix, detail: "the failing test", ok: true},
		{name: "explain command", comment: "@devloom explain the auth flow", want: CommandExplain, detail: "the auth flow", ok: true},
		{name: "ordinary comment", comment: "@devloom-helper please review", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, detail, ok := ParseMention(tt.comment, "@devloom")
			if ok != tt.ok || command != tt.want || detail != tt.detail {
				t.Fatalf("ParseMention() = %q, %q, %v; want %q, %q, %v", command, detail, ok, tt.want, tt.detail, tt.ok)
			}
		})
	}
}

func TestPromptKeepsReviewSubjectAndSafetyRules(t *testing.T) {
	prompt := Prompt(CommandFix, "https://gitlab.example/group/repo/-/merge_requests/2", "repair the nil check")
	for _, want := range []string{"Fix the requested issues", "isolated task branch", "protected base branch", "repair the nil check"} {
		if !contains(prompt, want) {
			t.Fatalf("prompt %q does not contain %q", prompt, want)
		}
	}
}

func contains(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
