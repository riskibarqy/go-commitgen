package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Repository exposes git operations required by the application.
type Repository interface {
	StagedDiff(ctx context.Context) (string, error)
	CurrentBranch(ctx context.Context) (string, error)
	Commit(ctx context.Context, headline, body string) error
	WriteHook(path, message string) error
}

// CLIRepository executes git commands through the local CLI.
type CLIRepository struct {
	Exec func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewCLIRepository returns a concrete Repository backed by the system git binary.
func NewCLIRepository() *CLIRepository {
	return &CLIRepository{
		Exec: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
	}
}

func (r *CLIRepository) StagedDiff(ctx context.Context) (string, error) {
	cmd := r.Exec(ctx, "git", "diff", "--staged", "-U0", "-M")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff error: %v\n%s", err, out.String())
	}
	return out.String(), nil
}

func (r *CLIRepository) CurrentBranch(ctx context.Context) (string, error) {
	cmd := r.Exec(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
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

	out.Reset()
	cmd = r.Exec(ctx, "git", "rev-parse", "--short", "HEAD")
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse --short failed: %v\n%s", err, out.String())
	}

	return strings.TrimSpace(out.String()), nil
}

func (r *CLIRepository) Commit(ctx context.Context, headline, body string) error {
	if strings.TrimSpace(headline) == "" {
		return fmt.Errorf("empty headline")
	}

	args := []string{"commit", "-m", headline}
	if strings.TrimSpace(body) != "" {
		args = append(args, "-m", body)
	}

	cmd := r.Exec(ctx, "git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *CLIRepository) WriteHook(path, message string) error {
	return os.WriteFile(path, []byte(message+"\n"), 0o644)
}
