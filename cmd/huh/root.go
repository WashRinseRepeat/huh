package main

import (
	"fmt"
	"os"
	"strings"

	"huh/internal/config"
	"huh/internal/llm"
	"huh/internal/ui"
	"huh/internal/usercontext"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "huh [question]",
	Short: "huh is your terminal AI assistant",
	Long:  `huh translates natural language questions into terminal commands.`,
	Args:  cobra.MaximumNArgs(100), // Allow any number, we join them. If 0, we enter interactive.
	Run: func(cmd *cobra.Command, args []string) {
		question := strings.Join(args, " ")
		
		// 1. Gather Context
		sysCtx := usercontext.GetContext()

		// 2. Setup Provider
		provider := llm.NewOllamaProvider() // Config-based loading in future

		// 3. Define Query Function
		queryFunc := func(q string) (string, error) {
			systemPrompt := fmt.Sprintf(
				"You are a command line helper for %s running %s shell. Your user asks: '%s'. Return ONLY the command to run, no markdown, no explanation.", 
				sysCtx.Distro, sysCtx.Shell, q,
			)
			// Hardware context could be added to prompt here
			
			return provider.Query(cmd.Context(), systemPrompt, q)
		}

		// 4. Define Explain Function
		explainFunc := func(command string) (string, error) {
			explainPrompt := fmt.Sprintf("Explain the following command briefly: '%s'", command)
			return provider.Query(cmd.Context(), "You are a helpful assistant explaining Linux commands.", explainPrompt)
		}

		// 5. Start TUI
		p := tea.NewProgram(ui.NewModel(question, queryFunc, explainFunc))
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
