package main

import (
	"context"
	"fmt"
	"os"
)

// runAPI resolves the provider, API key, and model, then delegates to the provider.
func runAPI(prompt, model string, cfg appConfig) error {
	providerName := cfg.resolvedProvider()
	p, err := getProvider(providerName)
	if err != nil {
		return err
	}

	apiKey := os.Getenv(p.EnvKey())
	if apiKey == "" {
		apiKey = cfg.APIKey
	}
	if apiKey == "" && p.EnvKey() != "" {
		return fmt.Errorf("API mode requires an API key. Set %q env var or \"api_key\" in config.", p.EnvKey())
	}

	if model == "" {
		model = p.DefaultModel()
	}
	modelID := p.ResolveModel(model)

	features := FeatureFlags{Thinking: thinkFlag, WebSearch: searchFlag}

	if dryRun {
		fmt.Printf("[%s] model=%s thinking=%v search=%v prompt=%q\n", providerName, modelID, features.Thinking, features.WebSearch, prompt)
		return nil
	}

	return p.Run(context.TODO(), prompt, model, apiKey, cfg.BaseURL, features)
}
