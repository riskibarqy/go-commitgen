package prompt

import "fmt"

// Review builds the prompt for lightweight code review.
func Review(diff string) string {
	return fmt.Sprintf(`You are a meticulous senior engineer.
Review the following git diff and highlight any potential issues.

Return plain text following this format:
- If you see problems: list each on its own line starting with "- " and keep each finding under 160 characters.
- If the changes look good: respond with "No blocking issues found."

Focus on correctness, security, performance, tests, and edge cases. Do not mention formatting unless it hides a bug.

Diff:
%s
`, diff)
}
