package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// openaiCompatProvider implements Provider for OpenAI-compatible APIs.
type openaiCompatProvider struct {
	name       string
	envKey     string
	defaultURL string
	aliases    map[string]string
	aliasList  []string
	defaultMdl string
}

func init() {
	registerProvider(openaiCompatProvider{
		name:       "openai",
		envKey:     "OPENAI_API_KEY",
		defaultURL: "https://api.openai.com/v1",
		aliases: map[string]string{
			"gpt4o":      "gpt-4o",
			"gpt4o-mini": "gpt-4o-mini",
			"o3-mini":    "o3-mini",
			"o4-mini":    "o4-mini",
		},
		aliasList:  []string{"gpt4o", "gpt4o-mini", "o3-mini", "o4-mini"},
		defaultMdl: "gpt4o",
	})

	registerProvider(openaiCompatProvider{
		name:       "xai",
		envKey:     "XAI_API_KEY",
		defaultURL: "https://api.x.ai/v1",
		aliases: map[string]string{
			"grok3":      "grok-3-latest",
			"grok3-mini": "grok-3-mini-latest",
		},
		aliasList:  []string{"grok3", "grok3-mini"},
		defaultMdl: "grok3",
	})

	registerProvider(openaiCompatProvider{
		name:       "ollama",
		envKey:     "",
		defaultURL: "http://localhost:11434/v1",
		aliases: map[string]string{
			"llama3":   "llama3",
			"qwen":     "qwen3",
			"deepseek": "deepseek-r1",
		},
		aliasList:  []string{"llama3", "qwen", "deepseek"},
		defaultMdl: "llama3",
	})
}

func (p openaiCompatProvider) Name() string        { return p.name }
func (p openaiCompatProvider) EnvKey() string       { return p.envKey }
func (p openaiCompatProvider) DefaultModel() string { return p.defaultMdl }
func (p openaiCompatProvider) ModelAliases() []string { return p.aliasList }

func (p openaiCompatProvider) ResolveModel(alias string) string {
	if id, ok := p.aliases[alias]; ok {
		return id
	}
	return alias
}

// isChatModel filters OpenAI models to chat-capable ones.
func isChatModel(id string) bool {
	prefixes := []string{"gpt-", "o1", "o3", "o4", "chatgpt"}
	for _, p := range prefixes {
		if strings.HasPrefix(id, p) {
			return true
		}
	}
	return false
}

func (p openaiCompatProvider) ListModels(ctx context.Context, apiKey, baseURL string) ([]RemoteModel, error) {
	if p.name == "ollama" {
		return p.listOllamaModels(baseURL)
	}

	if baseURL == "" {
		baseURL = p.defaultURL
	}

	opts := []option.RequestOption{option.WithBaseURL(baseURL)}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	client := openai.NewClient(opts...)

	var models []RemoteModel
	iter := client.Models.ListAutoPaging(ctx)
	for iter.Next() {
		m := iter.Current()
		if p.name == "openai" && !isChatModel(m.ID) {
			continue
		}
		models = append(models, RemoteModel{ID: m.ID})
	}
	if iter.Err() != nil {
		return nil, fmt.Errorf("%s API error: %v", p.name, iter.Err())
	}
	return models, nil
}

func (p openaiCompatProvider) listOllamaModels(baseURL string) ([]RemoteModel, error) {
	if baseURL == "" {
		baseURL = p.defaultURL
	}
	tagsURL := strings.TrimSuffix(baseURL, "/v1") + "/api/tags"

	resp, err := http.Get(tagsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama at %s: %w", tagsURL, err)
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	var models []RemoteModel
	for _, m := range result.Models {
		models = append(models, RemoteModel{ID: m.Name})
	}
	return models, nil
}

func (p openaiCompatProvider) Run(ctx context.Context, prompt, model, apiKey, baseURL string, features FeatureFlags) error {
	modelID := p.ResolveModel(model)
	if modelID == "" {
		modelID = p.ResolveModel(p.defaultMdl)
	}

	if baseURL == "" {
		baseURL = p.defaultURL
	}

	opts := []option.RequestOption{option.WithBaseURL(baseURL)}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	client := openai.NewClient(opts...)

	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(modelID),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	}

	if features.WebSearch && p.name == "openai" {
		params.WebSearchOptions = openai.ChatCompletionNewParamsWebSearchOptions{
			SearchContextSize: "medium",
		}
	}

	return runStreaming(func(emit func(string)) error {
		stream := client.Chat.Completions.NewStreaming(ctx, params)

		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) > 0 {
				emit(chunk.Choices[0].Delta.Content)
			}
		}

		if stream.Err() != nil {
			return fmt.Errorf("%s API error: %v", p.name, stream.Err())
		}
		return nil
	})
}
