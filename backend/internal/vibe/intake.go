package vibe

import "strings"

// A starter selects an intake path; it is not a task brief. Keep this check
// before admission so older clients cannot spend a request drafting its label.
func starterQuestion(content string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(content), " "))
	normalized = strings.ReplaceAll(normalized, "’", "'")
	normalized = strings.TrimRight(normalized, ".!?")
	switch normalized {
	case "help me build an agent":
		return "What should your agent help with? Describe the task and who it is for."
	case "i have an agent that needs testing":
		return "What does your agent do, how does it run, and what can you share for testing?"
	case "i'm figuring out what ai could do for us":
		return "What task would you like to make easier? Describe one part of your work."
	default:
		return ""
	}
}

func briefQuestion(doc Document, content string) string {
	question := starterQuestion(content)
	if question == "" || len(doc.Artifacts) > 0 || len(doc.Requirements) > 0 {
		return ""
	}
	for _, message := range doc.Messages {
		if message.Role == "user" && message.Origin != "playground" && strings.TrimSpace(message.Content) != "" && starterQuestion(message.Content) == "" {
			return ""
		}
	}
	return question
}
