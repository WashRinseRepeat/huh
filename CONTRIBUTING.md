# Contributing to huh

Thanks for the interest! `huh` is small enough that you can read the whole codebase in an afternoon — please do before opening a non-trivial PR.

## Quick start

```bash
git clone https://github.com/WashRinseRepeat/huh.git
cd huh
go build ./cmd/huh    # binary in ./huh
go test ./...
go vet ./...
```

To run locally without installing:

```bash
go run ./cmd/huh "how do I list files by size"
```

To run the first-run wizard against an isolated config:

```bash
XDG_CONFIG_HOME=$(mktemp -d) go run ./cmd/huh
```

## Project layout

```
cmd/huh/           CLI entry point (cobra)
internal/config/   YAML config load/write
internal/llm/      Provider interface + Ollama / OpenAI / OpenRouter impls
internal/setup/    First-run wizard (bubbletea)
internal/ui/       Main TUI model (bubbletea)
internal/history/  History persistence
internal/usercontext/  OS / distro / shell detection
```

## Pull requests

- Keep changes surgical — one concern per PR.
- Match the existing code style; we use stock `gofmt`.
- If you add a feature flag or config field, update `internal/config/config.example.yaml` and the README.
- New LLM providers should implement the `llm.LLM` interface and be wired in `internal/llm/factory.go`.
- TUI behavior changes: please include a short screen recording or ASCII before/after in the PR description.

## Reporting bugs

Use the bug template in `.github/ISSUE_TEMPLATE/`. The most useful things to include:

- `huh --version`
- Provider type (Ollama / OpenRouter / OpenAI-compatible) and the model
- A redacted snippet of `~/.config/huh/config.yaml` (strip API keys)
- The exact command you ran and the error output

## Security

For anything sensitive (credential leaks, command injection, etc.) please follow `SECURITY.md` rather than filing a public issue.
