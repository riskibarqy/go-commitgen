package commit

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/riskibarqy/go-commitgen/internal/util"
)

// Parts represents the structured information returned by the model.
type Parts struct {
	CommitType  string `json:"commit_type"`
	Description string `json:"description"`
	Summary     string `json:"summary"`
	Body        string `json:"body"`
}

// Message holds the final headline and body to be presented or committed.
type Message struct {
	Headline string
	Body     string
}

var (
	allowedCommitTypes = map[string]string{
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
	ticketPattern  = regexp.MustCompile(`^([A-Za-z]+-\d+)`)
	commitKeywords = []string{"fix", "feat", "perf", "refactor", "docs", "test", "build", "ci"}
)

// ParseParts normalises the model output into Parts enforcing length limits.
func ParseParts(raw string) (Parts, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Parts{}, errors.New("empty response")
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || start > end {
		return Parts{}, errors.New("response missing JSON object")
	}

	var p Parts
	if err := json.Unmarshal([]byte(raw[start:end+1]), &p); err != nil {
		return Parts{}, err
	}

	return normaliseParts(p), nil
}

// FallbackParts attempts to build a meaningful Parts struct from an arbitrary string.
func FallbackParts(raw string) Parts {
	clean := sanitizeDescription(raw)
	if clean == "" {
		clean = "update project files"
	}

	summary := sanitizeSummary(raw)
	if summary == "" {
		summary = util.TruncateShorten(clean, 100)
	}

	return Parts{
		CommitType:  detectCommitType(raw),
		Description: clean,
		Summary:     summary,
		Body:        sanitizeBody(raw, summary),
	}
}

// BuildMessage creates the final printable/committable representation.
func BuildMessage(branch string, parts Parts) Message {
	ticket := extractTicket(branch)
	commitType := normaliseCommitType(parts.CommitType)

	summary := sanitizeSummary(parts.Summary)

	description := sanitizeDescription(parts.Description)
	if description == "" {
		if summary != "" {
			description = summary
		} else {
			description = "update project files"
		}
	}

	if summary == "" {
		summary = util.TruncateShorten(description, 100)
	}

	body := sanitizeBody(parts.Body, summary)
	headline := strings.TrimSpace(strings.Join([]string{ticket, "[" + commitType + "]", description}, " "))

	return Message{
		Headline: headline,
		Body:     body,
	}
}

func normaliseParts(p Parts) Parts {
	p.CommitType = normaliseCommitType(p.CommitType)
	p.Description = sanitizeDescription(p.Description)
	p.Summary = sanitizeSummary(p.Summary)
	p.Body = sanitizeBody(p.Body, p.Summary)
	return p
}

func normaliseCommitType(t string) string {
	candidate := strings.ToLower(strings.TrimSpace(t))
	candidate = strings.Trim(candidate, "[]")
	if candidate == "" {
		return "chore"
	}
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
	for _, candidate := range commitKeywords {
		if strings.Contains(lower, candidate) {
			return normaliseCommitType(candidate)
		}
	}
	return "chore"
}

func sanitizeDescription(s string) string {
	s = util.CondenseSpaces(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if len([]rune(s)) > 72 {
		s = util.TruncateShorten(s, 72)
	}
	return strings.TrimRight(s, ".")
}

func sanitizeSummary(s string) string {
	s = util.CondenseSpaces(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if len([]rune(s)) > 100 {
		s = util.TruncateShorten(s, 100)
	}
	return s
}

func sanitizeBody(body, summary string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		body = summary
	}

	lines := util.TrimLines(body)
	if len(lines) == 0 {
		return ""
	}

	for i, line := range lines {
		line = util.CondenseSpaces(line)
		if len([]rune(line)) > 300 {
			line = util.TruncateShorten(line, 300)
		}
		lines[i] = line
	}

	return strings.Join(lines, "\n")
}

func extractTicket(branch string) string {
	branch = util.CondenseSpaces(strings.TrimSpace(branch))
	if branch == "" {
		return "unknown"
	}

	if idx := strings.LastIndex(branch, "/"); idx != -1 && idx < len(branch)-1 {
		branch = branch[idx+1:]
	}

	if m := ticketPattern.FindStringSubmatch(branch); len(m) == 2 {
		return m[1]
	}

	return branch
}
