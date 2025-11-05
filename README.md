go-commitgen
============

`go-commitgen` builds AI-assisted commit messages from your staged git diff using a local [Ollama](https://ollama.com) model.  
It analyses the diff, surfaces potential issues, and composes a Conventional Commit–style headline plus a short body that references your branch ticket.

Features
--------
- Generates messages like `TES-123 [feat] add login audit hook` based on the current branch.
- Uses Ollama locally (no cloud calls) with per-command model overrides.
- Runs a fast review pass before committing and prints findings.
- Can auto-run `git commit -m "<headline>" -m "<body>"` or just print the draft.
- Supports prepare-commit-msg/commit-msg hooks via `--hook`.

Requirements
------------
- Go 1.21+
- Git with staged changes (`git add …`)
- Ollama listening on `http://localhost:11434` (changeable with flags/env vars)

Installation
------------
```sh
# compile once and keep the binary on your PATH
go build -o go-commitgen ./cmd/go-commitgen
mv go-commitgen ~/go/bin/            # or any directory in PATH

# optional helper alias
echo 'alias gcm="go-commitgen"' >> ~/.zshrc
```

Configuration
-------------
Environment variables:
- `OLLAMA_ENDPOINT` – default `http://localhost:11434`
- `OLLAMA_MODEL` – commit message model (default `qwen3:8b`)
- `OLLAMA_REVIEW_MODEL` – review model (defaults to `OLLAMA_MODEL`)
- `COMMITGEN_MAX_BYTES` – max diff bytes sent to the model (default `32000`)

Usage
-----
1. Stage your changes: `git add -p` (or similar).
2. Run the generator:
   ```sh
   go-commitgen                    # runs review, commits automatically
   go-commitgen --commit=false     # only print the suggestion
   go-commitgen --model llama3     # switch the generation model
   go-commitgen --review-model qwen2:7b --review=false
   go-commitgen --hook .git/COMMIT_EDITMSG
   ```
3. Review the “Review findings” block (if any) and inspect the formatted message.
4. If `--commit` is true (default), your staged changes are committed automatically; otherwise copy/edit the output before committing manually.

Flags
-----
- `--model` – Ollama model used to compose the commit message.
- `--review-model` – separate model for the review pass.
- `--review` – enable/disable the reviewer (default true).
- `--commit` – auto-run `git commit` when true (default true).
- `--hook <path>` – write the message into the provided hook file and exit.
- `--endpoint` – override Ollama endpoint.
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
- “review failed” → ensure Ollama is running or adjust `--endpoint`.
- Responses look generic → try a larger model (`--model qwen2.5-coder:14b`) or increase context via `--max-bytes`.
