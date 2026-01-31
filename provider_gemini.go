package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type geminiProvider struct{}

func init() {
	registerProvider(geminiProvider{})
}

func (geminiProvider) Name() string { return "gemini" }

func (geminiProvider) ResolveModel(alias string) string {
	aliases := map[string]string{
		"flash":      "gemini-2.5-flash",
		"pro":        "gemini-2.5-pro",
		"flash-lite": "gemini-2.0-flash-lite",
	}
	if id, ok := aliases[alias]; ok {
		return id
	}
	return alias
}

func (geminiProvider) ModelAliases() []string {
	return []string{"flash", "pro", "flash-lite"}
}

func (geminiProvider) DefaultModel() string { return "flash" }

func (geminiProvider) EnvKey() string { return "GEMINI_API_KEY" }

func (p geminiProvider) ListModels(ctx context.Context, apiKey, _ string) ([]RemoteModel, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	page, err := client.Models.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("Gemini API error: %v", err)
	}

	var models []RemoteModel
	for _, m := range page.Items {
		id := strings.TrimPrefix(m.Name, "models/")
		// Only include generateContent-capable models
		supported := false
		for _, action := range m.SupportedActions {
			if action == "generateContent" {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}
		models = append(models, RemoteModel{ID: id, Name: m.DisplayName})
	}
	return models, nil
}

func (p geminiProvider) Run(ctx context.Context, prompt, model, apiKey, _ string, features FeatureFlags) error {
	modelID := p.ResolveModel(model)
	if modelID == "" {
		modelID = p.ResolveModel(p.DefaultModel())
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %w", err)
	}

	var config *genai.GenerateContentConfig
	if features.Thinking || features.WebSearch {
		config = &genai.GenerateContentConfig{}
		if features.Thinking {
			config.ThinkingConfig = &genai.ThinkingConfig{
				ThinkingBudget: genai.Ptr(int32(10000)),
			}
		}
		if features.WebSearch {
			config.Tools = []*genai.Tool{
				{GoogleSearch: &genai.GoogleSearch{}},
			}
		}
	}

	return runStreaming(func(emit func(string)) error {
		for result, err := range client.Models.GenerateContentStream(
			ctx,
			modelID,
			genai.Text(prompt),
			config,
		) {
			if err != nil {
				return fmt.Errorf("Gemini API error: %v", err)
			}
			emit(result.Text())
		}
		return nil
	})
}
