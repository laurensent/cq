package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var remoteModels bool

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available models for the current provider",
	Long:  "Show model aliases for the configured provider.\nWith --remote, query the provider API for all available models.",
	RunE: func(cmd *cobra.Command, args []string) error {
		providerName := cfg.resolvedProvider()
		if cfg.Mode != "api" {
			providerName = "anthropic"
		}

		p, err := getProvider(providerName)
		if err != nil {
			return err
		}

		fmt.Printf("Provider: %s\n\n", providerName)

		// Aliases table
		aliases := p.ModelAliases()
		defaultModel := p.DefaultModel()

		w := tabwriter.NewWriter(os.Stdout, 2, 0, 3, ' ', 0)
		fmt.Fprintf(w, "  ALIAS\tMODEL ID\t\n")
		for _, alias := range aliases {
			marker := ""
			if alias == defaultModel {
				marker = " (default)"
			}
			fmt.Fprintf(w, "  %s\t%s%s\t\n", alias, p.ResolveModel(alias), marker)
		}
		w.Flush()

		if remoteModels {
			apiKey := os.Getenv(p.EnvKey())
			if apiKey == "" {
				apiKey = cfg.APIKey
			}
			if apiKey == "" && p.EnvKey() != "" {
				return fmt.Errorf("--remote requires API key. Set %q or run: ask config", p.EnvKey())
			}

			models, err := p.ListModels(context.TODO(), apiKey, cfg.BaseURL)
			if err != nil {
				return fmt.Errorf("failed to list models: %w", err)
			}

			// Filter out models that already have aliases
			aliasIDs := make(map[string]bool)
			for _, a := range aliases {
				aliasIDs[p.ResolveModel(a)] = true
			}

			var extra []RemoteModel
			for _, m := range models {
				if !aliasIDs[m.ID] {
					extra = append(extra, m)
				}
			}

			sort.Slice(extra, func(i, j int) bool { return extra[i].ID < extra[j].ID })

			if len(extra) > 0 {
				fmt.Printf("\n  Additional models:\n")
				for _, m := range extra {
					if m.Name != "" && m.Name != m.ID {
						fmt.Printf("    %s (%s)\n", m.ID, m.Name)
					} else {
						fmt.Printf("    %s\n", m.ID)
					}
				}
			}
		}

		fmt.Printf("\nTip: pass any full model ID with -m\n")
		return nil
	},
}

func init() {
	modelsCmd.Flags().BoolVar(&remoteModels, "remote", false, "query provider API for all available models")
}
