package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/WashRinseRepeat/huh/internal/config"
	"github.com/WashRinseRepeat/huh/internal/llm"
	"github.com/WashRinseRepeat/huh/internal/setup"
	"github.com/WashRinseRepeat/huh/internal/ui"
	"github.com/WashRinseRepeat/huh/internal/usercontext"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
)

var files []string
var showConfigLocation bool
var noColor bool

// version is set via -ldflags "-X main.version=..." at build time.
var version = "dev"

func init() {
	rootCmd.Flags().StringSliceVarP(&files, "file", "f", []string{}, "file(s) to attach")
	rootCmd.Flags().BoolVarP(&showConfigLocation, "config-location", "c", false, "show the location of the config file")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output (also respects the NO_COLOR env var)")
}

// applyColorPreference forces lipgloss into Ascii-only mode when the user has
// opted out of color via --no-color or the NO_COLOR environment variable
// (https://no-color.org — presence of the var is enough, regardless of value).
// When --no-color is set we also export NO_COLOR=1 so downstream renderers
// (glamour, anything else that reads env) get the message too.
func applyColorPreference() {
	if noColor {
		os.Setenv("NO_COLOR", "1")
		lipgloss.SetColorProfile(termenv.Ascii)
		return
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

var rootCmd = &cobra.Command{
	Use:              "huh [question]",
	Short:            "huh is your terminal AI assistant",
	Long:             `huh translates natural language questions into terminal commands.`,
	Version:          version,
	Args:             cobra.MaximumNArgs(100), // Allow any number, we join them. If 0, we enter interactive.
	PersistentPreRun: func(cmd *cobra.Command, args []string) { applyColorPreference() },
	Run: func(cmd *cobra.Command, args []string) {
		if showConfigLocation {
			path, err := config.GetConfigLocation()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(path)
			return
		}

		// 0. First-start setup wizard
		if !config.ConfigExists() {
			if !setup.Run() {
				// User cancelled the wizard
				return
			}
			config.Reload()
		}

		question := strings.Join(args, " ")
		stat, _ := os.Stdin.Stat()
		stdinPiped := (stat.Mode() & os.ModeCharDevice) == 0

		// Refuse early when stdin is piped but no question was given:
		// without a question there's nothing to ask, and bubbletea would
		// crash later with a cryptic "could not open a new TTY" error.
		if question == "" && stdinPiped {
			fmt.Fprintln(os.Stderr, "huh: when piping input, please provide a question")
			fmt.Fprintln(os.Stderr, "  example: cat error.log | huh \"what's failing?\"")
			os.Exit(1)
		}

		// 1. Gather Context
		sysCtx := usercontext.GetContext()

		// 2. Read Attachments
		var contextBuilder strings.Builder
		var contextInfoParts []string

		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", f, err)
				continue
			}
			contextBuilder.WriteString(fmt.Sprintf("\n--- File: %s ---\n%s\n", f, string(b)))
			contextInfoParts = append(contextInfoParts, f)
		}

		if stdinPiped {
			b, err := io.ReadAll(os.Stdin)
			if err == nil {
				contextBuilder.WriteString(fmt.Sprintf("\n--- Stdin ---\n%s\n", string(b)))
				contextInfoParts = append(contextInfoParts, "Stdin")
			}
		}

		attachedContent := contextBuilder.String()
		contextInfo := strings.Join(contextInfoParts, ", ")

		// 3. Setup Provider
		provider, err := llm.NewProvider("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "huh: %v\n", err)
			if cfgPath, perr := config.GetConfigLocation(); perr == nil {
				fmt.Fprintf(os.Stderr, "\nconfig file: %s\n", cfgPath)
			}
			fmt.Fprintln(os.Stderr, "re-run the setup wizard with: huh setup")
			os.Exit(1)
		}

		// 4. Define Query Function
		queryFunc := func(q string, dynamicContext string) (string, llm.TokenUsage, error) {
			finalQuestion := q
			
			// Context is managed by the UI model and passed as dynamicContext
			if dynamicContext != "" {
				finalQuestion = fmt.Sprintf("%s\n\nAttached Context:\n%s", q, dynamicContext)
			}

            // Build User Context String
            var customContext strings.Builder
            if len(sysCtx.Custom) > 0 {
                customContext.WriteString("User Info: ")
                for k, v := range sysCtx.Custom {
                    customContext.WriteString(fmt.Sprintf("%s=%s; ", k, v))
                }
            }

			baseSystemPrompt := config.AppConfig.SystemPrompt
			if baseSystemPrompt == "" {
				baseSystemPrompt = "If the user asks for a command, provide it inside a markdown code block, like:\n" +
					"```bash\ncommand here\n```\n" +
					"You can also provide a brief explanation outside the block. If the user asks a question, answer it normally."
			}

			systemPrompt := fmt.Sprintf(
				"Context: OS: %s, Distro: %s, Shell: %s. %s\n"+
				"User Query: '%s'.\n"+
				"%s",
				sysCtx.OS, sysCtx.Distro, sysCtx.Shell, customContext.String(), q, baseSystemPrompt,
			)
			
			return provider.Query(cmd.Context(), systemPrompt, finalQuestion)
		}

		// 5. Define Explain Function
		explainFunc := func(command string, dynamicContext string) (string, llm.TokenUsage, error) {
			prompt := fmt.Sprintf("Explain the following command briefly: '%s'", command)
			
			if dynamicContext != "" {
				prompt += fmt.Sprintf("\n\nContext:\n%s", dynamicContext)
			}
			return provider.Query(cmd.Context(), "You are a helpful assistant explaining Linux commands. Be concise.", prompt)
		}

		// 6. Define Refine Function
		// `originalQuestion` is the user's first/anchor question (passed in
		// from the model, not the closure-captured CLI arg — that's empty for
		// interactive launches). `original` is either a bare command
		// (single-command refinement) or the full prior answer text including
		// all code blocks (multi-command refinement). The prompt is
		// generalized to handle both shapes.
		refineFunc := func(originalQuestion, original, refinement, dynamicContext string) (string, llm.TokenUsage, error) {
			refinePrompt := fmt.Sprintf(
				"Original Request: '%s'.\n\nOriginal Answer:\n%s\n\nRefinement Request: '%s'.\n\n"+
					"Update the answer. Keep commands inside ```bash code blocks. "+
					"If the original had multiple commands, return the updated set in the same order. "+
					"You may explain the change briefly if needed.",
				originalQuestion, original, refinement,
			)

			if dynamicContext != "" {
				refinePrompt += fmt.Sprintf("\n\nContext:\n%s", dynamicContext)
			}
			systemPrompt := fmt.Sprintf("You are a command line helper for %s. Update the answer based on the user's request.", sysCtx.Distro)
			return provider.Query(cmd.Context(), systemPrompt, refinePrompt)
		}

		// 7. Start TUI
		opts := []tea.ProgramOption{tea.WithOutput(os.Stderr)}
		if stdinPiped {
			f, err := os.Open("/dev/tty")
			if err == nil {
				defer f.Close()
				opts = append(opts, tea.WithInput(f))
			}
		}

		p := tea.NewProgram(ui.NewModel(question, contextInfo, attachedContent, queryFunc, explainFunc, refineFunc), opts...)
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Re-run the first-run setup wizard",
	Long:  `Launches the interactive setup wizard, overwriting any existing configuration.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !setup.Run() {
			os.Exit(1)
		}
		config.Reload()
	},
}

func Execute() {
	config.Init()
	rootCmd.AddCommand(setupCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
