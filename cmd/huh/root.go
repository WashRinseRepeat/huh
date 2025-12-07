package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"huh/internal/config"
	"huh/internal/llm"
	"huh/internal/ui"
	"huh/internal/usercontext"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var files []string

func init() {
	rootCmd.Flags().StringSliceVarP(&files, "file", "f", []string{}, "file(s) to attach")
}

var rootCmd = &cobra.Command{
	Use:   "huh [question]",
	Short: "huh is your terminal AI assistant",
	Long:  `huh translates natural language questions into terminal commands.`,
	Args:  cobra.MaximumNArgs(100), // Allow any number, we join them. If 0, we enter interactive.
	Run: func(cmd *cobra.Command, args []string) {
		question := strings.Join(args, " ")
		
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

		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			b, err := io.ReadAll(os.Stdin)
			if err == nil {
				contextBuilder.WriteString(fmt.Sprintf("\n--- Stdin ---\n%s\n", string(b)))
				contextInfoParts = append(contextInfoParts, "Stdin")
			}
		}

		attachedContent := contextBuilder.String()
		contextInfo := strings.Join(contextInfoParts, ", ")

		// 3. Setup Provider
		provider := llm.NewOllamaProvider()

		// 4. Define Query Function
		queryFunc := func(q string) (string, error) {
			finalQuestion := q
			if attachedContent != "" {
				finalQuestion = fmt.Sprintf("%s\n\nAttached Context:\n%s", q, attachedContent)
			}

			systemPrompt := fmt.Sprintf(
				"You are a command line helper for %s running %s shell. Your user asks: '%s'. Return ONLY the command to run, no markdown, no explanation.", 
				sysCtx.Distro, sysCtx.Shell, q,
			)
			
			return provider.Query(cmd.Context(), systemPrompt, finalQuestion)
		}

		// 5. Define Explain Function
		explainFunc := func(command string) (string, error) {
			// Attach content to explain as well? Usually explain is about the command.
			// But context might explain "why" the command works on "this file".
			// Let's attach it.
			prompt := fmt.Sprintf("Explain the following command briefly: '%s'", command)
			if attachedContent != "" {
				prompt += fmt.Sprintf("\n\nContext:\n%s", attachedContent)
			}
			return provider.Query(cmd.Context(), "You are a helpful assistant explaining Linux commands.", prompt)
		}

		// 6. Define Refine Function
		refineFunc := func(originalCommand, refinement string) (string, error) {
			refinePrompt := fmt.Sprintf(
				"Original Request: '%s'. Original Command: '%s'. Refinement Request: '%s'. Return ONLY the updated command, no markdown, no explanation.", 
				question, originalCommand, refinement,
			)
			if attachedContent != "" {
				refinePrompt += fmt.Sprintf("\n\nContext:\n%s", attachedContent)
			}
			systemPrompt := fmt.Sprintf("You are a command line helper for %s. Update the command based on user request.", sysCtx.Distro)
			return provider.Query(cmd.Context(), systemPrompt, refinePrompt)
		}

		// 7. Start TUI
		opts := []tea.ProgramOption{tea.WithOutput(os.Stderr)} // Standard practice to keep stdout clean? No, TUI usually owns /dev/tty.
		// If stdin is piped, we MUST use /dev/tty for input
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			f, err := os.Open("/dev/tty")
			if err == nil {
				defer f.Close()
				opts = append(opts, tea.WithInput(f))
			}
		}

		p := tea.NewProgram(ui.NewModel(question, contextInfo, queryFunc, explainFunc, refineFunc), opts...)
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	config.Init()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
