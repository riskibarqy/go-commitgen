package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/riskibarqy/go-commitgen/internal/commit"
	"github.com/riskibarqy/go-commitgen/internal/git"
	"github.com/riskibarqy/go-commitgen/internal/ollama"
	"github.com/riskibarqy/go-commitgen/internal/prompt"
	"github.com/riskibarqy/go-commitgen/internal/util"
)

// LLMClient represents the behaviour needed from an Ollama client.
type LLMClient interface {
	Generate(ctx context.Context, endpoint string, req ollama.Request) (string, error)
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

	if opts.ReviewModel == "" {
		opts.ReviewModel = opts.Model
	}

	diff, err := s.Repo.StagedDiff(ctx)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(diff) == "" {
		return Result{}, errors.New("no staged changes detected")
	}

	diff = util.TrimTo(diff, opts.MaxBytes)

	branch, err := s.Repo.CurrentBranch(ctx)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		DiffUsed: diff,
		Branch:   branch,
	}

	if opts.Review {
		review, err := s.LLM.Generate(ctx, opts.Endpoint, ollama.Request{
			Model:   opts.ReviewModel,
			Prompt:  prompt.Review(diff),
			Stream:  true,
			Options: map[string]interface{}{"temperature": 0.1, "top_p": 0.9, "num_predict": 200},
		})
		if err != nil {
			result.ReviewErr = err
		} else {
			result.Review = strings.TrimSpace(review)
		}
	}

	raw, err := s.LLM.Generate(ctx, opts.Endpoint, ollama.Request{
		Model:   opts.Model,
		Prompt:  prompt.Commit(diff, branch),
		Stream:  true,
		Options: map[string]interface{}{"temperature": 0.2, "top_p": 0.9, "num_predict": 120},
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
