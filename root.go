package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var model string
var dryRun bool
var rawOutput bool
var thinkFlag bool
var searchFlag bool
var cfg appConfig

var rootCmd = &cobra.Command{
	Use:   "ask [prompt...]",
	Short: "Quick invoke LLMs from terminal with markdown rendering",
	Long:  "ask is a fast CLI tool for single-shot LLM queries from the terminal.\nIt supports multiple providers (Anthropic, OpenAI, Gemini, xAI, Ollama),\nrendered markdown output, pipe input, and query history.",
	Example: `  ask "how to rebase"
  ask -m opus "complex question"
  ask --raw "question"             # skip markdown rendering
  ask                              # interactive mode (no shell escaping needed)
  git diff | ask "review this code"
  cat error.log | ask "analyze this error"`,
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var pipeContent string
		if isPiped() {
			var err error
			pipeContent, err = readPipe()
			if err != nil {
				return fmt.Errorf("reading pipe: %w", err)
			}
		}

		prompt := buildPrompt(args, pipeContent)
		if prompt == "" {
			// Interactive mode: read prompt from stdin (bypass shell parsing)
			var err error
			prompt, err = readInteractivePrompt()
			if err != nil {
				return nil // canceled (ESC / Ctrl+C), exit silently
			}
			if prompt == "" {
				return cmd.Help()
			}
		}
		saveHistory(prompt)
		if cfg.Mode == "api" {
			return runAPI(prompt, model, cfg)
		}
		return runClaude(prompt, model)
	},
}

var historyCmd = &cobra.Command{
	Use:     "history",
	Aliases: []string{"h"},
	Short:   "Browse and re-run past queries",
	Long:    "Open an interactive fuzzy finder to search and re-run past queries.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return interactiveHistory()
	},
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all query history",
	RunE: func(cmd *cobra.Command, args []string) error {
		return clearHistory()
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Interactive configuration wizard",
	Long:  "Launch a TUI wizard to configure ask step by step.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConfigInit()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&model, "model", "m", "", "model alias or full ID (provider-specific: sonnet, gpt4o, flash, etc.)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "print the claude command instead of running it")
	rootCmd.PersistentFlags().BoolVar(&rawOutput, "raw", false, "output raw text without markdown rendering")
	rootCmd.PersistentFlags().BoolVar(&thinkFlag, "think", false, "enable extended thinking")
	rootCmd.PersistentFlags().BoolVar(&searchFlag, "search", false, "enable web search")

	// Apply config defaults before command execution
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cfg = loadConfig()
		if !cmd.Flags().Changed("model") && cfg.DefaultModel != "" {
			model = cfg.DefaultModel
		}
		if !cmd.Flags().Changed("raw") && cfg.RawOutput {
			rawOutput = true
		}
		if !cmd.Flags().Changed("think") {
			thinkFlag = cfg.Thinking
		}
		if !cmd.Flags().Changed("search") {
			searchFlag = cfg.WebSearch
		}
	}

	historyCmd.AddCommand(historyClearCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(modelsCmd)

	rootCmd.SetVersionTemplate("ask version {{.Version}}\n")

	// Override default error output
	rootCmd.SetErrPrefix("Error:")

	// Disable completion command (keep it clean)
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Show help if stderr is needed
	rootCmd.SetOut(os.Stdout)
}
