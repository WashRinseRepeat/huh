Implementation Plan - "huh" CLI
Goal
Build the v0.1 prototype of "huh", a fast Linux CLI AI helper in Go. This version establishes the core architecture, interactive TUI, and local LLM (Ollama) integration.

Proposed Changes
Project Structure
New Go project initialized in user's workspace.

1. Core Application (cmd/, main.go)
main.go: Entry point.
cmd/root.go: Uses spf13/cobra to handle the primary huh [question] command.
cmd/version.go: Version info.
2. Configuration (internal/config)
Uses spf13/viper.
Loads from ~/.config/huh/config.yaml or defaults.
Settings defined:
Provider (default: "ollama")
Model (default: "llama3")
ContextLevel (default: "basic")
Sanitize (default: true for remote, optional for local)
3. Context & Sanitization (internal/usercontext)
Describe(level string) string: Gathers info based on config.
Basic: OS Release file (
/etc/os-release
), $SHELL.
Hardware: Adds basic CPU/Mem/Disk info (using shirou/gopsutil or reading /proc).
Sanitize(input string) string: Regex replacement for common secrets before sending.
4. LLM Backend (internal/llm)
Interface: Provider with Ask(ctx context.Context, prompt string, system string) (string, error).
Implementations:
ollama: Connects to http://localhost:11434/api/generate.
5. Interactive UI (internal/ui)
Library: charmbracelet/bubbletea.
Flow:
Loading Spinner: While waiting for LLM.
Result View:
Displays the command in a syntax-highlighted block.
Keys: Enter (Copy & Exit), e (Explain), q (Quit), c (Copy & Exit).
Clipboard: Uses atotto/clipboard for local, prints OSC 52 sequence if SSH detected.
Verification Plan
Automated Tests
Unit Tests:
internal/usercontext: reliable gathering of OS info (mocking file reads).
internal/llm/ollama: Mock HTTP server to verify request format.
Manual Verification
Build: go build -o huh .
Setup: Ensure Ollama is running (systemctl status ollama or local check).
Run: ./huh "how do I check disk space"
Expect: Spinner -> Result df -h -> TUI options.
Interaction:
Press Enter -> Should exit and copy df -h to clipboard.
Paste to terminal to verify.
Config: Create ~/.config/huh/config.yaml, change model, verify it picks it up (via debug log or verbose flag).