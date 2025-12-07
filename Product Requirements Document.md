Product Requirements Document: "huh"
1. Product Overview
"huh" is a fast, streamlined CLI tool that translates natural language questions into Linux terminal commands. It serves as an instant "man page" for the modern era, leveraging AI to help users—especially beginners or those switching from Windows—navigate the Linux command line with confidence.

2. Goals & Principles
Speed is Feature #1: The tool must be instant (<500ms for simple queries) to feel like a native part of the shell.
Safety First: Prevent accidental execution of destructive commands and minimize data leakage.
Ease of Use: Zero-config start (where possible), single binary distribution, and intuitive TUI.
Local-First Mentality: Prioritize local models (Ollama) and lightweight usage, with cloud fallback for complex queries.
3. Core Features
3.1 Natural Language Query
Trigger: huh [question]
Output: Brief explanation + suggested command(s).
Example:
$ huh how do I find the largest file here
# Suggestion:
du -ah . | sort -rh | head -n 5
3.2 Interactive TUI (The "Do It" Flow)
If a command is suggested, the tool enters an interactive mode (using a library like Bubble Tea).
Options provided:
Copy: Copies the command to the system clipboard.
Explain: Breaks down the command flags and arguments.
Edit: Allows the user to edit the command before copying.
Cancel: Exits clearly.
Note: "Execute immediately" is intentionally OMITTED for v1 to enforce user review and safety.
3.3 "Explain" Mode
Flag: huh -e [command] or huh --explain [command]
Function: Takes a raw shell command and explains what it does in plain English.
Context: Useful for auditing commands found online before running them.
3.4 Configuration & Privacy
Context Level: Users can configure how much system info is sent:
Basic: Distro + Shell.
Hardware: Adds GPU, CPU, RAM, and Storage details.
Full: Adds limited environment variables.
Sanitization:
Built-in regex filters to strip potential secrets (API keys, passwords).
Configurable per Provider: Can be disabled for local models (e.g., Ollama) or trusted endpoints, but enabled by default for public clouds.
Provider Backend:
Support for Local (Ollama).
Support for Cloud (OpenAI, Gemini, OpenRouter).
Smart Routing: (Future) Auto-switch between a "Small/Fast" model for syntax help and a "Large/Slow" model for complex logic.
4. Technical Architecture
4.1 Implementation Language: Go (Golang)
Why:
Single Binary: Easy cross-platform distribution (Linux, macOS, Windows) without dependency hell.
Performance: Fast startup time is critical for a CLI tool.
Ecosystem: Excellent CLI libraries (Cobra, Viper) and TUI libraries (Bubble Tea).
4.2 Key Libraries
CLI Framework: spf13/cobra
TUI: charmbracelet/bubbletea
Clipboard: atotto/clipboard (with fallback to OSC 52 if possible)
5. User Stories
The Newbie: Alice just installed Ubuntu. She wants to install vlc but forgets the package manager command. She types huh install vlc. She sees sudo apt install vlc, hits "Copy", pastes it, and runs it.
The Pro: Bob is stuck in a headless server. He types huh check disk usage. He gets df -h. He copies it using the TUI which supports his terminal's clipboard integration.
6. Future Considerations (v2+)
Shell Integration: Hook Ctrl+Space to invoke huh with the current buffer.
Active Context: Analyze the previous command's error code to auto-suggest fixes (e.g., huh --fix).
Alias Generator: "Save this command as 'update-system'" -> adds alias to .bashrc.
