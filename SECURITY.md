# Security policy

## Reporting a vulnerability

If you find a security issue in `huh`, please **do not** open a public GitHub issue.

Instead, open a private security advisory:

  https://github.com/WashRinseRepeat/huh/security/advisories/new

Please include:

- A description of the issue and its potential impact.
- Steps to reproduce, or a proof of concept.
- The version of `huh` you tested (`huh --version`).

We'll acknowledge receipt within 7 days and aim to ship a fix or mitigation within 30 days of confirmation.

## Scope

In scope:

- The `huh` binary and its direct dependencies as pinned in `go.mod`.
- The first-run wizard and config file handling.
- The sudo file-read path used for attaching restricted files.

Out of scope:

- The behavior of third-party LLM providers (Ollama, OpenAI, OpenRouter) — report those upstream.
- The content of LLM responses themselves — `huh` never executes a suggested command automatically, and the user is expected to review every command before running it.
