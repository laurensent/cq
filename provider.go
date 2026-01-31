package main

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
)

// RemoteModel represents a model returned by a provider's API.
type RemoteModel struct {
	ID   string
	Name string
}

// FeatureFlags controls optional provider features like thinking and web search.
type FeatureFlags struct {
	Thinking  bool
	WebSearch bool
}

// Provider defines the interface for LLM API providers.
type Provider interface {
	Name() string
	ResolveModel(alias string) string
	ModelAliases() []string
	DefaultModel() string
	EnvKey() string
	Run(ctx context.Context, prompt, model, apiKey, baseURL string, features FeatureFlags) error
	ListModels(ctx context.Context, apiKey, baseURL string) ([]RemoteModel, error)
}

// providers is the registry of available providers.
var providers = map[string]Provider{}

// registerProvider adds a provider to the registry.
func registerProvider(p Provider) {
	providers[p.Name()] = p
}

// getProvider returns the provider with the given name, or an error.
func getProvider(name string) (Provider, error) {
	p, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q (available: %s)", name, strings.Join(providerNames(), ", "))
	}
	return p, nil
}

// providerNames returns a sorted list of registered provider names.
func providerNames() []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// runStreaming is a shared helper that manages the spinner and buffer/render
// lifecycle for streaming provider responses. The streamFn callback receives
// a function to call with each text chunk.
func runStreaming(streamFn func(emit func(text string)) error) error {
	sp := startSpinner()

	needRender := !rawOutput && isStdoutTerminal()
	var outBuf bytes.Buffer
	spinnerStopped := false

	emit := func(text string) {
		if needRender {
			outBuf.WriteString(text)
		} else {
			if !spinnerStopped {
				sp.Stop()
				spinnerStopped = true
			}
			fmt.Print(text)
		}
	}

	err := streamFn(emit)
	sp.Stop()

	if err != nil {
		return err
	}

	if needRender {
		raw := outBuf.String()
		if raw == "" {
			return nil
		}
		rendered, renderErr := renderMarkdown(strings.TrimSpace(raw))
		if renderErr != nil {
			fmt.Print(raw)
			return nil
		}
		fmt.Print(rendered)
	}

	return nil
}
