# huh

**huh** is a fast, streamlined CLI tool that translates natural language questions into Linux terminal commands. It serves as an instant "man page" for the modern era, leveraging AI to help you navigate the command line with confidence.

![huh demo](https://via.placeholder.com/800x400?text=huh+CLI+Demo+Placeholder)

## Features

*   **Natural Language to Command**: Just ask "how do I..." and get the command you need.
*   **Safety First**: Commands are not executed automatically. You review them first.
*   **Interactive TUI**: Review, refine, copy, or get an explanation of the command before running it.
*   **Context Aware**: Knows your OS, Distro, and Shell to provide relevant answers.
*   **Flexible Providers**: Supports local models via **Ollama** (default) or cloud providers like **OpenAI** and **OpenRouter**.
*   **Cross-Platform**: Works on **Linux** and **macOS**.

## Installation

### From Source

You need [Go](https://go.dev/dl/) installed (version 1.24+ recommended).

```bash
git clone https://github.com/WashRinseRepeat/huh.git
cd huh
make install
```

## Configuration

`huh` uses a configuration file located at `~/.config/huh/config.yaml`.
On the first run, `huh` will create a default configuration file for you.

### Supported Providers

#### 1. Ollama (Local - Default)
Great for privacy and offline usage. Requires [Ollama](https://ollama.com/) to be running.

```yaml
default_provider: ollama
providers:
  ollama:
    type: ollama
    params:
      host: http://localhost:11434
      model: llama3
```

#### 2. OpenAI
Requires an API key.

```yaml
default_provider: openai
providers:
  openai:
    type: openai
    params:
      api_key: sk-proj-...
      model: gpt-4o
```

#### 3. OpenRouter
Access a wide variety of models.

```yaml
default_provider: openrouter
providers:
  openrouter:
    type: openrouter
    params:
      api_key: sk-or-...
      model: anthropic/claude-3-opus
```

### Customizing Behavior

You can customize the system prompt to change how `huh` behaves, or add custom context variables.
I would recommend you keep the bash part so huh can display and interact with code blocks appropiately.

```yaml
# ~/.config/huh/config.yaml

# Custom System Prompt
system_prompt: |
  You are a helpful CLI assistant.
  Always explain the command briefly before showing the code block.
  If a sequence of commands is suggested, explain each command briefly before showing the code block containing the commands in sequence.
  If the user asks for a command, provide it inside a markdown code block, like:
  ```bash
  command here
  ```

context:
  level: basic
  preference: "I prefer using ripgrep over grep"
```

## Usage

### Basic Query
Ask a question to get a command suggestion.

```bash
huh how do I find the largest file in the current directory
```

### Interactive Mode
Once a command is suggested, you enter the interactive mode:
*   **Enter**: Copy command to clipboard and exit.
*   **e**: Explain the command.
*   **c**: Copy command to clipboard and exit.
*   **q**: Quit without copying.

### Attach Files
You can attach files to your query for context.

```bash
huh -f error.log "why is this failing?"
```

## License

MIT License. See [LICENSE](LICENSE) for details.
