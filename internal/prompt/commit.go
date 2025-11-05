package prompt

import "fmt"

// Commit builds the prompt sent to the model for commit generation.
func Commit(diff, branch string) string {
	return fmt.Sprintf(`You help craft git commit messages.
Analyse the staged diff and respond with a single JSON object describing the commit.

Requirements:
- "commit_type": choose the best fit from ["feat","fix","perf","refactor","docs","test","build","chore","ci"].
- "description": short imperative summary of what changed (<= 72 characters, no trailing punctuation, lower case start).
- "summary": brief reason or impact of the change (<= 100 characters).
- "body": 1-3 sentences that highlight key details or rationale (<= 300 characters). Use newline separators if listing items.
- Output only valid JSON. No prose, markdown, or backticks.

Example:
{"commit_type":"fix","description":"handle nil pointer in parser","summary":"avoid panic when schema metadata missing","body":"Add nil check before parser access to prevent runtime crash."}

Context:
- Branch: %s
- Diff:
%s
`, branch, diff)
}
