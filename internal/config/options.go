package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultEndpoint    = "http://localhost:11434"
	defaultModel       = "qwen2.5-coder:1.5b"
	defaultReviewModel = "qwen2.5-coder:1.5b"
	defaultMaxBytes    = 32000
	defaultTimeout     = 40 * time.Second
)

// Options captures all user facing configuration.
type Options struct {
	Model        string
	ReviewModel  string
	Endpoint     string
	MaxBytes     int
	Commit       bool
	Review       bool
	HookPath     string
	Timeout      time.Duration
	Args         []string
	RawFlagSet   *flag.FlagSet
	DisplayUsage func()
}

// Parse consumes CLI flags/environment variables and returns validated options.
func Parse() (Options, error) {
	fs := flag.NewFlagSet("go-commitgen", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	model := fs.String("model", envOr("OLLAMA_MODEL", defaultModel), "Ollama model used for commit generation")
	reviewModel := fs.String("review-model", envOr("OLLAMA_REVIEW_MODEL", defaultReviewModel), "Ollama model used for code review (falls back to --model)")
	endpoint := fs.String("endpoint", envOr("OLLAMA_ENDPOINT", defaultEndpoint), "Ollama base URL")
	maxBytes := fs.Int("max-bytes", intFromEnv("COMMITGEN_MAX_BYTES", defaultMaxBytes), "Maximum diff bytes to send to the model")
	commitNow := fs.Bool("commit", true, "Run `git commit -m` with the generated message")
	runReview := fs.Bool("review", false, "Run an AI review before generating the commit message")
	hookPath := fs.String("hook", "", "When set, write the message into the given hook file")
	timeout := fs.Duration("timeout", durationFromEnv("COMMITGEN_TIMEOUT", defaultTimeout), "Total timeout for the command")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return Options{}, fmt.Errorf("parse flags: %w", err)
	}

	opts := Options{
		Model:        stringsFallback(*model, defaultModel),
		ReviewModel:  stringsFallback(*reviewModel, *model),
		Endpoint:     stringsFallback(*endpoint, defaultEndpoint),
		MaxBytes:     *maxBytes,
		Commit:       *commitNow,
		Review:       *runReview,
		HookPath:     *hookPath,
		Timeout:      *timeout,
		Args:         fs.Args(),
		RawFlagSet:   fs,
		DisplayUsage: fs.Usage,
	}

	return opts, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func intFromEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		var d time.Duration
		if parsed, err := time.ParseDuration(v); err == nil {
			d = parsed
		} else if _, err := fmt.Sscanf(v, "%d", &d); err == nil {
			// accept bare integers treated as seconds
			d *= time.Second
		}
		if d > 0 {
			return d
		}
	}
	return fallback
}

func stringsFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
