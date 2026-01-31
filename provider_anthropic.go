package main

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type anthropicProvider struct{}

func init() {
	registerProvider(anthropicProvider{})
}

func (anthropicProvider) Name() string { return "anthropic" }

func (anthropicProvider) ResolveModel(alias string) string {
	aliases := map[string]string{
		"opus":   "claude-opus-4-5-20251101",
		"sonnet": "claude-sonnet-4-5-20250929",
		"haiku":  "claude-haiku-4-5-20251001",
	}
	if id, ok := aliases[alias]; ok {
		return id
	}
	return alias
}

func (anthropicProvider) ModelAliases() []string {
	return []string{"sonnet", "opus", "haiku"}
}

func (anthropicProvider) DefaultModel() string { return "sonnet" }

func (anthropicProvider) EnvKey() string { return "ANTHROPIC_API_KEY" }

func (p anthropicProvider) ListModels(ctx context.Context, apiKey, baseURL string) ([]RemoteModel, error) {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)

	var models []RemoteModel
	iter := client.Models.ListAutoPaging(ctx, anthropic.ModelListParams{})
	for iter.Next() {
		m := iter.Current()
		models = append(models, RemoteModel{ID: m.ID, Name: m.DisplayName})
	}
	if iter.Err() != nil {
		return nil, fmt.Errorf("Anthropic API error: %v", iter.Err())
	}
	return models, nil
}

func (p anthropicProvider) Run(ctx context.Context, prompt, model, apiKey, baseURL string, features FeatureFlags) error {
	modelID := p.ResolveModel(model)
	if modelID == "" {
		modelID = p.ResolveModel(p.DefaultModel())
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := anthropic.NewClient(opts...)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(modelID),
		MaxTokens: 8192,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}

	if features.Thinking {
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(10000)
		params.MaxTokens = 16000
	}

	if features.WebSearch {
		params.Tools = append(params.Tools, anthropic.ToolUnionParam{
			OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{},
		})
	}

	return runStreaming(func(emit func(string)) error {
		stream := client.Messages.NewStreaming(ctx, params)

		for stream.Next() {
			event := stream.Current()
			switch ev := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch delta := ev.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					emit(delta.Text)
				}
			}
		}

		if stream.Err() != nil {
			return fmt.Errorf("Anthropic API error: %v", stream.Err())
		}
		return nil
	})
}
