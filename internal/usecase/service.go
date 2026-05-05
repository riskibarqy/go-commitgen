package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/riskibarqy/go-commitgen/internal/commit"
	"github.com/riskibarqy/go-commitgen/internal/git"
	"github.com/riskibarqy/go-commitgen/internal/llm"
	"github.com/riskibarqy/go-commitgen/internal/prompt"
	"github.com/riskibarqy/go-commitgen/internal/util"
)

// LLMClient represents transport needed from an LLM backend.
type LLMClient interface {
	Generate(ctx context.Context, endpoint, apiKey string, req llm.Request) (string, error)
}

// Service orchestrates the review and commit message generation flow.
type Service struct {
	Repo git.Repository
	LLM  LLMClient
}

// Result captures the outputs of the use case.
type Result struct {
	Review    string
	ReviewErr error
	Message   commit.Message
	DiffUsed  string
	Branch    string
}

// Options is a light copy of the config options needed inside the use case.
type Options struct {
	Model       string
	ReviewModel string
	Endpoint    string
	APIKey      string
	MaxBytes    int
	Review      bool
}

// NewService constructs a Service with the provided dependencies.
func NewService(repo git.Repository, llm LLMClient) *Service {
	return &Service{Repo: repo, LLM: llm}
}

// Execute performs the review+generation workflow.
func (s *Service) Execute(ctx context.Context, opts Options) (Result, error) {
	if s == nil || s.Repo == nil || s.LLM == nil {
		return Result{}, errors.New("service not properly initialized")
	}

	diff, branch, err := s.prepare(ctx, opts)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		DiffUsed: diff,
		Branch:   branch,
	}

	if opts.Review {
		review, err := s.performReview(ctx, diff, opts)
		if err != nil {
			result.ReviewErr = err
		} else {
			result.Review = review
		}
	}

	raw, err := s.LLM.Generate(ctx, opts.Endpoint, opts.APIKey, llm.Request{
		Model:       opts.Model,
		Prompt:      prompt.Commit(diff, branch),
		Temperature: 0.2,
		TopP:        0.9,
		MaxTokens:   120,
	})
	if err != nil {
		return Result{}, err
	}

	parts, err := commit.ParseParts(raw)
	if err != nil {
		parts = commit.FallbackParts(raw)
	}

	result.Message = commit.BuildMessage(branch, parts)
	return result, nil
}

// ReviewOnly evaluates only the review prompt and returns the findings.
func (s *Service) ReviewOnly(ctx context.Context, opts Options) (string, error) {
	if s == nil || s.Repo == nil || s.LLM == nil {
		return "", errors.New("service not properly initialized")
	}

	diff, _, err := s.prepare(ctx, opts)
	if err != nil {
		return "", err
	}

	return s.performReview(ctx, diff, opts)
}

func (s *Service) prepare(ctx context.Context, opts Options) (string, string, error) {
	diff, err := s.Repo.StagedDiff(ctx)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(diff) == "" {
		return "", "", errors.New("no staged changes detected")
	}

	diff = util.TrimTo(diff, opts.MaxBytes)

	branch, err := s.Repo.CurrentBranch(ctx)
	if err != nil {
		return "", "", err
	}

	return diff, branch, nil
}

func (s *Service) performReview(ctx context.Context, diff string, opts Options) (string, error) {
	model := opts.ReviewModel
	if strings.TrimSpace(model) == "" {
		model = opts.Model
	}

	review, err := s.LLM.Generate(ctx, opts.Endpoint, opts.APIKey, llm.Request{
		Model:       model,
		Prompt:      prompt.Review(diff),
		Temperature: 0.1,
		TopP:        0.9,
		MaxTokens:   200,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(review), nil
}
