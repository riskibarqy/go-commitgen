package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/riskibarqy/go-commitgen/internal/config"
	"github.com/riskibarqy/go-commitgen/internal/git"
	"github.com/riskibarqy/go-commitgen/internal/openai"
	"github.com/riskibarqy/go-commitgen/internal/usecase"
)

func main() {
	opts, err := config.Parse()
	if err != nil {
		exitWithError("failed to parse configuration", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	service := usecase.NewService(git.NewCLIRepository(), openai.NewClient(opts.Timeout))
	review, err := service.ReviewOnly(ctx, usecase.Options{
		Model:       opts.Model,
		ReviewModel: opts.ReviewModel,
		Endpoint:    opts.Endpoint,
		APIKey:      opts.APIKey,
		MaxBytes:    opts.MaxBytes,
		Review:      true,
	})
	if err != nil {
		exitWithError("review failed", err)
	}

	review = strings.TrimSpace(review)
	if review == "" {
		fmt.Println("No blocking issues found.")
		return
	}

	fmt.Println("Review findings:")
	fmt.Println(review)
}

func exitWithError(msg string, err error) {
	fmt.Fprintf(os.Stderr, "❌ %s: %v\n", msg, err)
	os.Exit(1)
}
