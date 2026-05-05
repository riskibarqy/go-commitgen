package commit

import "testing"

func TestBuildMessageUsesTicketFromBranch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		branch   string
		headline string
	}{
		{
			name:     "feature branch with plain ticket",
			branch:   "feat/WIT-123",
			headline: "WIT-123 add audit log",
		},
		{
			name:     "feature branch with suffix",
			branch:   "feat/WIT-123-dev",
			headline: "WIT-123 add audit log",
		},
		{
			name:     "release branch with multiple suffixes",
			branch:   "release/WIT-123-dev-stg",
			headline: "WIT-123 add audit log",
		},
		{
			name:     "hotfix branch with random suffix",
			branch:   "hotfix/WIT-123-cnisancdi",
			headline: "WIT-123 add audit log",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := BuildMessage(tt.branch, Parts{
				CommitType:  "feat",
				Description: "add audit log",
				Summary:     "record request activity",
				Body:        "Add structured audit log for request lifecycle.",
			})

			if msg.Headline != tt.headline {
				t.Fatalf("headline = %q, want %q", msg.Headline, tt.headline)
			}
		})
	}
}

func TestBuildMessageFallsBackToDescriptionWhenNoTicket(t *testing.T) {
	t.Parallel()

	msg := BuildMessage("main", Parts{
		CommitType:  "feat",
		Description: "add audit log",
		Summary:     "record request activity",
		Body:        "Add structured audit log for request lifecycle.",
	})

	if msg.Headline != "add audit log" {
		t.Fatalf("headline = %q, want %q", msg.Headline, "add audit log")
	}
}
