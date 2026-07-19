// Package gitreview contains the provider-independent commands used by Git
// webhook review tasks.
package gitreview

import (
	"strings"
	"unicode"
)

// Command is the action requested in a pull-request comment.
type Command string

const (
	CommandReview  Command = "review"
	CommandFix     Command = "fix"
	CommandExplain Command = "explain"
	CommandPlan    Command = "plan"
)

// ParseMention extracts a supported command and the remaining instructions
// from a comment. A bare mention defaults to review, preserving the original
// automatic-review behavior.
func ParseMention(comment, mention string) (Command, string, bool) {
	comment = strings.TrimSpace(comment)
	mention = strings.TrimSpace(mention)
	if comment == "" || mention == "" {
		return "", "", false
	}

	index := indexMention(comment, mention)
	if index < 0 {
		return "", "", false
	}

	remaining := strings.TrimSpace(comment[:index] + " " + comment[index+len(mention):])
	words := strings.Fields(remaining)
	command := CommandReview
	if len(words) > 0 {
		candidate := strings.ToLower(strings.TrimLeft(words[0], "/:"))
		switch Command(candidate) {
		case CommandReview, CommandFix, CommandExplain, CommandPlan:
			command = Command(candidate)
			words = words[1:]
		}
	}

	return command, strings.TrimSpace(strings.Join(words, " ")), true
}

// Prompt turns a provider comment into an explicit instruction for the
// isolated coding environment. The URL is included so agents can identify the
// review subject even when the comment contains no additional text.
func Prompt(command Command, subjectURL, detail string) string {
	subjectURL = strings.TrimSpace(subjectURL)
	detail = strings.TrimSpace(detail)
	if command == "" {
		command = CommandReview
	}

	var instruction string
	switch command {
	case CommandFix:
		instruction = "Fix the requested issues in this pull request. Work only in the isolated task branch, run focused tests, and summarize the changes. Do not modify or merge the protected base branch."
	case CommandExplain:
		instruction = "Explain the pull request and identify correctness, security, and maintenance risks. Do not modify files or push commits."
	case CommandPlan:
		instruction = "Create an implementation plan for this pull request, including affected files, risks, tests, and rollout steps. Do not modify files or push commits."
	default:
		instruction = "Review this pull request for correctness, security, regressions, and missing tests. Report actionable findings with file and line references. Do not modify or merge the protected base branch."
	}

	if subjectURL != "" {
		instruction += "\nReview subject: " + subjectURL
	}
	if detail != "" && !strings.EqualFold(detail, subjectURL) {
		instruction += "\nAdditional instructions: " + detail
	}
	return instruction
}

func indexMention(value, mention string) int {
	lowerValue := strings.ToLower(value)
	lowerMention := strings.ToLower(mention)
	for offset := 0; ; {
		relative := strings.Index(lowerValue[offset:], lowerMention)
		if relative < 0 {
			return -1
		}
		index := offset + relative
		end := index + len(mention)
		if mentionBoundary(value, index-1) && mentionBoundary(value, end) {
			return index
		}
		offset = index + len(mention)
		if offset >= len(value) {
			return -1
		}
	}
}

func mentionBoundary(value string, index int) bool {
	if index < 0 || index >= len(value) {
		return true
	}
	r := rune(value[index])
	return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '-'
}
