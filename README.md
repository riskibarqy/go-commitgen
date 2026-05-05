go-commitgen
============

`go-commitgen` builds AI-assisted commit messages from your staged git diff using an OpenAI-compatible AI router such as [9router](https://github.com/decolua/9router).  
It analyses the diff, surfaces potential issues, and composes a Conventional Commit–style headline plus a short body that references your branch ticket.

Features
--------
- Generates messages like `TES-123 [feat] add login audit hook` based on the current branch.
- Uses an OpenAI-compatible router with per-command model overrides.
- Runs a fast review pass before committing and prints findings.
- Can auto-run `git commit -m "<headline>" -m "<body>"` or just print the draft.
- Supports prepare-commit-msg/commit-msg hooks via `--hook`.

Requirements
------------
- Go 1.21+
- Git with staged changes (`git add …`)
- 9router or another OpenAI-compatible endpoint listening on `http://localhost:20128/v1` (changeable with flags/env vars)

Installation
------------
```sh
# compile once and keep the binaries on your PATH
go build -o go-commitgen ./cmd/go-commitgen
mv go-commitgen ~/go/bin/            # or any directory in PATH

# optional dedicated reviewer binary
go build -o go-commitgen-review ./cmd/go-commitgen-review
mv go-commitgen-review ~/go/bin/

# optional helper alias
echo 'alias gcm="go-commitgen"' >> ~/.zshrc
echo 'alias gcmr="go-commitgen-review"' >> ~/.zshrc
```

Configuration
-------------
Environment variables:
- `COMMITGEN_ENDPOINT` – default `http://localhost:20128/v1`
- `COMMITGEN_API_KEY` – bearer token for router auth
- `COMMITGEN_MODEL` – commit message model (default `9router-codex`)
- `COMMITGEN_REVIEW_MODEL` – review model (defaults to `COMMITGEN_MODEL`)
- `OPENAI_BASE_URL` / `OPENAI_API_KEY` – supported aliases
- `OLLAMA_ENDPOINT` / `OLLAMA_MODEL` / `OLLAMA_REVIEW_MODEL` – legacy fallback aliases
- `COMMITGEN_MAX_BYTES` – max diff bytes sent to the model (default `32000`)

Usage
-----
1. Stage your changes: `git add -p` (or similar).
2. Run the generator:
   ```sh
   go-commitgen                    # uses model 9router-codex
   go-commitgen --commit=false     # only print the suggestion
   go-commitgen --model 9router-codex
   go-commitgen --endpoint http://homeserver:20128/v1 --api-key "$COMMITGEN_API_KEY"
   go-commitgen --review-model 9router-codex --review=false
   go-commitgen --hook .git/COMMIT_EDITMSG
   ```
3. Review the “Review findings” block (if any) and inspect the formatted message.
4. If `--commit` is true (default), your staged changes are committed automatically; otherwise copy/edit the output before committing manually.

Review-only workflow:
```sh
go-commitgen-review
go-commitgen-review --model 9router-codex --max-bytes 64000
```

Flags
-----
- `--model` – model used to compose the commit message.
- `--review-model` – separate model for the review pass.
- `--review` – enable/disable the reviewer (default true).
- `--commit` – auto-run `git commit` when true (default true).
- `--hook <path>` – write the message into the provided hook file and exit.
- `--endpoint` – override OpenAI-compatible endpoint.
- `--api-key` – bearer token for the endpoint.
- `--max-bytes` – limit the diff size sent to the model.

Sample Output
-------------
```
Review findings:
- missing null check on new helper can panic if response is nil

TES-123 [feat] add login audit hook

add audit publisher for login flow and guard nil response path
```

Hook Integration
----------------
Add to `.git/hooks/prepare-commit-msg`:
```sh
#!/bin/sh
go-commitgen --hook "$1" --commit=false
```
Mark it executable with `chmod +x .git/hooks/prepare-commit-msg`.

Troubleshooting
---------------
- “No staged changes” → run `git status` and stage files.
- “review failed” → ensure 9router is reachable or adjust `--endpoint` / `--api-key`.
- Responses look generic → inspect router combo/model selection or increase context via `--max-bytes`.
