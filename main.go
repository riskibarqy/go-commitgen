package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	defaultEndpoint = "http://localhost:11434"
	defaultModel    = "qwen3:8b" // small, fast; change to your favorite
	defaultMaxBytes = 32000      // keep prompts snappy
)

type ollamaGenerateReq struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaGenerateChunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func main() {
	var (
		model       = flag.String("model", envOr("OLLAMA_MODEL", defaultModel), "Ollama model name")
		reviewModel = flag.String("review-model", envOr("OLLAMA_REVIEW_MODEL", ""), "Ollama model used for code review (defaults to --model)")
		endpoint    = flag.String("endpoint", envOr("OLLAMA_ENDPOINT", defaultEndpoint), "Ollama base URL")
		maxBytes    = flag.Int("max-bytes", intFromEnv("COMMITGEN_MAX_BYTES", defaultMaxBytes), "Max diff bytes to send")
		commitNow   = flag.Bool("commit", true, "Run `git commit -m` with the generated message")
		runReview   = flag.Bool("review", true, "Have the model review the diff before composing the commit message")
		hookPath    = flag.String("hook", "", "When set, write the message into the given commit-msg/prepare-commit-msg file")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	diff, err := stagedDiff(ctx)
	check(err, "failed to read staged diff")
	if strings.TrimSpace(diff) == "" {
		fail("No staged changes. Stage your changes first: `git add ...`")
	}

	diff = trimTo(diff, *maxBytes)

	branch, err := currentBranch(ctx)
	check(err, "failed to determine current branch")

	selectedReviewModel := strings.TrimSpace(*reviewModel)
	if selectedReviewModel == "" {
		selectedReviewModel = *model
	}

	if *runReview {
		review, err := generateReview(ctx, *endpoint, selectedReviewModel, diff)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ review failed: %v\n", err)
		} else if strings.TrimSpace(review) != "" {
			fmt.Println("Review findings:")
			fmt.Println(review)
			fmt.Println()
		}
	}

	raw, err := generateMessage(ctx, *endpoint, *model, diff, branch)
	check(err, "failed to generate message")

	parts, err := parseCommitParts(raw)
	if err != nil {
		parts = fallbackCommitParts(raw)
	}

	headline, body := formatCommitOutput(branch, parts)
	if strings.TrimSpace(headline) == "" {
		fail("Model returned an empty message. Consider a larger model/temp or smaller diff.")
	}

	commitMessage := headline
	if body != "" {
		commitMessage = headline + "\n\n" + body
	}

	// If used as a hook, write to the provided file
	if *hookPath != "" {
		check(os.WriteFile(*hookPath, []byte(commitMessage+"\n"), 0644), "failed to write hook message file")
		fmt.Println(commitMessage)
		return
	}

	// Print for manual use, or commit if asked
	if *commitNow {
		check(runGitCommit(ctx, headline, body), "git commit failed")
		fmt.Println(commitMessage)
		return
	}

	fmt.Println(commitMessage)
}

func stagedDiff(ctx context.Context) (string, error) {
	// Unified diff without context to reduce tokens; add -M for renames detection
	cmd := exec.CommandContext(ctx, "git", "diff", "--staged", "-U0", "-M")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff error: %v\n%s", err, out.String())
	}
	return out.String(), nil
}

func trimTo(s string, max int) string {
	if len(s) <= max {
		return s
	}
	head := s[:max]
	// try not to cut mid-line
	if idx := strings.LastIndex(head, "\n"); idx > 0 {
		head = head[:idx]
	}
	return head + "\n…[diff truncated]"
}

func generateMessage(ctx context.Context, endpoint, model, diff, branch string) (string, error) {
	return callOllama(ctx, endpoint, model, buildPrompt(diff, branch), map[string]interface{}{
		"temperature": 0.2,
		"top_p":       0.9,
		"num_predict": 120,
	})
}

func generateReview(ctx context.Context, endpoint, model, diff string) (string, error) {
	return callOllama(ctx, endpoint, model, buildReviewPrompt(diff), map[string]interface{}{
		"temperature": 0.1,
		"top_p":       0.9,
		"num_predict": 200,
	})
}

func callOllama(ctx context.Context, endpoint, model, prompt string, options map[string]interface{}) (string, error) {
	reqBody := ollamaGenerateReq{
		Model:   model,
		Prompt:  prompt,
		Stream:  true,
		Options: options,
	}

	b, _ := json.Marshal(reqBody)
	httpClient := &http.Client{
		Timeout:   35 * time.Second,
		Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext},
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(endpoint, "/")+"/api/generate", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
	}

	var out strings.Builder
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Bytes()
		var chunk ollamaGenerateChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue // ignore junk
		}
		out.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}

	return strings.TrimSpace(out.String()), nil
}

func buildPrompt(diff, branch string) string {
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

func buildReviewPrompt(diff string) string {
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

type commitParts struct {
	CommitType  string `json:"commit_type"`
	Description string `json:"description"`
	Summary     string `json:"summary"`
	Body        string `json:"body"`
}

var allowedCommitTypes = map[string]string{
	"feat":     "feat",
	"feature":  "feat",
	"fix":      "fix",
	"bugfix":   "fix",
	"perf":     "perf",
	"refactor": "refactor",
	"docs":     "docs",
	"doc":      "docs",
	"test":     "test",
	"tests":    "test",
	"build":    "build",
	"chore":    "chore",
	"ci":       "ci",
}

func parseCommitParts(raw string) (commitParts, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return commitParts{}, errors.New("empty model response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || start > end {
		return commitParts{}, errors.New("model response missing JSON object")
	}

	var parts commitParts
	if err := json.Unmarshal([]byte(raw[start:end+1]), &parts); err != nil {
		return commitParts{}, err
	}

	parts.CommitType = normalizeCommitType(parts.CommitType)
	parts.Description = sanitizeDescription(parts.Description)
	parts.Summary = sanitizeSummary(parts.Summary)
	parts.Body = sanitizeBody(parts.Body)

	if parts.Description == "" {
		return commitParts{}, errors.New("model response missing description")
	}

	return parts, nil
}

func fallbackCommitParts(raw string) commitParts {
	clean := sanitizeDescription(raw)
	if clean == "" {
		clean = "update project files"
	}

	summary := sanitizeSummary(raw)
	if summary == "" {
		summary = truncate(clean, 100)
	}

	return commitParts{
		CommitType:  detectCommitType(raw),
		Description: clean,
		Summary:     summary,
		Body:        sanitizeBody(raw),
	}
}

func normalizeCommitType(t string) string {
	candidate := strings.ToLower(strings.TrimSpace(t))
	candidate = strings.Trim(candidate, "[]")
	if mapped, ok := allowedCommitTypes[candidate]; ok {
		return mapped
	}
	for key, value := range allowedCommitTypes {
		if strings.HasPrefix(candidate, key) {
			return value
		}
	}
	return "chore"
}

func detectCommitType(raw string) string {
	lower := strings.ToLower(raw)
	for _, candidate := range []string{"fix", "feat", "perf", "refactor", "docs", "test", "build", "ci"} {
		if strings.Contains(lower, candidate) {
			return normalizeCommitType(candidate)
		}
	}
	return "chore"
}

func sanitizeDescription(s string) string {
	s = condenseSpaces(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if len([]rune(s)) > 72 {
		s = truncate(s, 72)
	}
	return strings.TrimRight(s, ".")
}

func sanitizeSummary(s string) string {
	s = condenseSpaces(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if len([]rune(s)) > 100 {
		s = truncate(s, 100)
	}
	return s
}

func sanitizeBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	lines := strings.Split(s, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = condenseSpaces(strings.TrimSpace(line))
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	body := strings.Join(cleaned, "\n")
	if len([]rune(body)) > 300 {
		body = truncate(body, 300)
	}
	return body
}

func formatCommitOutput(branch string, parts commitParts) (string, string) {
	ticket := extractTicket(branch)

	commitType := normalizeCommitType(parts.CommitType)
	description := parts.Description
	if description == "" {
		description = "update project files"
	}

	summary := parts.Summary
	if summary == "" {
		summary = truncate(description, 100)
	}

	body := parts.Body
	if body == "" {
		body = summary
	}

	headline := fmt.Sprintf("%s [%s] %s", ticket, commitType, description)
	return headline, body
}

func extractTicket(branch string) string {
	branch = condenseSpaces(strings.TrimSpace(branch))
	if branch == "" {
		return "unknown"
	}

	if idx := strings.LastIndex(branch, "/"); idx != -1 && idx < len(branch)-1 {
		branch = branch[idx+1:]
	}

	return branch
}

func currentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse failed: %v\n%s", err, out.String())
	}

	branch := strings.TrimSpace(out.String())
	if branch != "" && branch != "HEAD" {
		return branch, nil
	}

	// Detached HEAD fallback
	out.Reset()
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse --short failed: %v\n%s", err, out.String())
	}

	return strings.TrimSpace(out.String()), nil
}

func condenseSpaces(s string) string {
	return regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

func runGitCommit(ctx context.Context, headline, body string) error {
	if strings.TrimSpace(headline) == "" {
		return errors.New("empty message")
	}

	args := []string{"commit", "-m", headline}
	if strings.TrimSpace(body) != "" {
		args = append(args, "-m", body)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func intFromEnv(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func check(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s: %v\n", msg, err)
		os.Exit(1)
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "❌ "+msg)
	os.Exit(1)
}
