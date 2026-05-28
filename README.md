# huh

**huh** is a fast, streamlined CLI tool that translates natural language questions into Linux terminal commands. It's an instant "man page" for the modern era — ask in plain English, review the suggested command, then copy it (or refine it, or get it explained).

```
   $ huh how do I find files larger than 100MB in the current dir

   Suggestion:
   To find files larger than 100MB in the current directory, use the
   `find` command with `-size +100M`:

   ╭────────────────────────────────╮
   │   find . -type f -size +100M   │
   ╰────────────────────────────────╯

   [C]opy  [E]xplain  [R]efine  [Q]uit       tokens: 141 in / 133 out
```

> 📺 A screen recording is on the roadmap — see issue tracker for progress.

## Features

- **Natural language → command** — Ask "how do I…" and get the command you need.
- **Safety first** — Commands are never executed automatically. You review them first.
- **Interactive TUI** — Copy, explain, or refine the suggested command. Cycle through multiple suggestions with Tab.
- **Context aware** — Knows your OS, distro, and shell to give relevant answers.
- **Flexible providers** — Local models via [Ollama](https://ollama.com/) (default), or cloud providers like [OpenRouter](https://openrouter.ai) and any OpenAI-compatible endpoint.
- **First-run setup wizard** — Guided TUI walks you through provider selection and configuration on first launch. Re-run anytime with `huh setup`.
- **History** — Resume from your last query or browse recent ones.
- **Token usage tracking** — Per-session input/output/total counts in the footer.
- **Cross-platform** — Works on Linux and macOS.

## Installation

### `go install` (recommended)

Requires [Go](https://go.dev/dl/) 1.24+:

```bash
go install github.com/WashRinseRepeat/huh/cmd/huh@latest
```

The binary lands in `$(go env GOPATH)/bin` — make sure that's on your `PATH`.

### From source

```bash
git clone https://github.com/WashRinseRepeat/huh.git
cd huh

# Install to /usr/local/bin (needs sudo for the default prefix)
sudo make install

# …or install to a user-writable prefix, no sudo needed:
PREFIX=$HOME/.local make install
```

The Makefile honors `PREFIX` (default `/usr/local`); the binary is placed at `$(PREFIX)/bin/huh`.

## First-run setup

On first launch, `huh` shows an interactive setup wizard that walks you through:

1. **Provider selection** — Ollama (local), OpenRouter (cloud), or any OpenAI-compatible endpoint.
2. **Provider configuration**:
   - **Ollama** — auto-detects your local instance and lists installed models.
   - **OpenRouter** — guides you through getting an API key and links to [browse cheap programming models](https://openrouter.ai/models?categories=programming&max_price=1&order=most-popular).
   - **OpenAI-compatible** — prompts for base URL, optional API key, and model name.
3. **Config written** — Saves to `~/.config/huh/config.yaml`.

You can navigate back at any step with `Esc` and quit entirely with `Ctrl+C`.

Re-run the wizard later with `huh setup` (it overwrites the existing config).

## Usage

### Basic query

```bash
huh how do I find the largest file in the current directory
```

You can also pipe context:

```bash
cat error.log | huh "why is this failing?"
```

### Attach files

```bash
huh -f error.log "why is this failing?"
```

You can also attach files from within the TUI: press `Tab` to focus the **[Attach File]** button, then `Enter` to open a path picker (with live tab-completion).

### Interactive controls

Once a suggestion appears:

| Key | Action |
|---|---|
| `c` or `Enter` (with **Copy** selected) | Copy the highlighted command to the clipboard, then quit. |
| `e` | Get a one-paragraph explanation of the command. |
| `r` | Refine — describe how the command should change, get a new suggestion. |
| `q` or `Esc` | Quit (saves to history). |
| `↑` / `↓` or `Tab` / `Shift+Tab` | Pick the previous/next command when the answer has multiple code blocks. With a single command, they scroll the view instead. |
| `←` / `→` | Move between actions in the bottom bar. |
| `PgUp` / `PgDown` / `Ctrl+U` / `Ctrl+D` | Scroll by half a page (works regardless of how many commands there are). |

When you launch `huh` with no question and history exists, you'll see a menu with **Start new chat**, **View last answer**, and **View history**.

### Other flags

```
-c, --config-location   Print the path to the config file and exit.
-f, --file strings      Attach one or more files as context.
-v, --version           Print the version and exit.
-h, --help              Show help.
```

And the `setup` subcommand:

```bash
huh setup    # re-run the first-run wizard
```

## Configuration

`huh` uses a YAML config file at `~/.config/huh/config.yaml` (the wizard creates it on first run; print the path with `huh -c`).

### Supported providers

#### Ollama (local — default)

Great for privacy and offline usage. Requires [Ollama](https://ollama.com/) running.

```yaml
default_provider: ollama
providers:
  ollama:
    type: ollama
    params:
      host: http://localhost:11434
      model: llama3
```

#### OpenAI

```yaml
default_provider: openai
providers:
  openai:
    type: openai
    params:
      api_key: sk-proj-...
      model: gpt-4o
```

#### OpenRouter

Access a wide variety of models through one API key.

```yaml
default_provider: openrouter
providers:
  openrouter:
    type: openrouter
    params:
      api_key: sk-or-...
      model: anthropic/claude-3-opus
```

### Keeping API keys out of the config file

For both OpenAI and OpenRouter, you can replace the literal `api_key` value with `api_key_env: VAR_NAME` to read the key from an environment variable instead. This keeps secrets out of your dotfiles/repo.

```yaml
providers:
  openrouter:
    type: openrouter
    params:
      api_key_env: OPENROUTER_API_KEY    # read $OPENROUTER_API_KEY at runtime
      model: anthropic/claude-3-opus
```

If both `api_key` and `api_key_env` are set, `api_key_env` wins when the variable is non-empty; otherwise `api_key` is used as a fallback.

### Customizing behavior

You can customize the system prompt and add custom context that gets injected into every query. We recommend keeping the `bash` markdown block instruction so the TUI can detect and box commands correctly.

```yaml
# ~/.config/huh/config.yaml

system_prompt: |
  You are a helpful CLI assistant.
  Always explain the command briefly before showing the code block.
  If a sequence of commands is suggested, explain each before showing the
  code block containing the commands in sequence.
  If the user asks for a command, provide it inside a markdown code block, like:
  ```bash
  command here
  ```

context:
  level: basic
  preference: "I prefer using ripgrep over grep"
```

Any extra keys under `context:` are passed to the LLM as user preferences.

### Token usage

Token counts appear at the bottom of the screen throughout the session: input tokens, output tokens, and the running total. Useful for keeping an eye on cloud-provider costs.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, project layout, and PR guidelines. Bug and feature templates are in `.github/ISSUE_TEMPLATE/`.

For security issues, please follow [SECURITY.md](SECURITY.md) rather than filing a public issue.

## License

MIT License. See [LICENSE](LICENSE) for details.
