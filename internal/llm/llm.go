// Package llm provides a unified interface for OpenAI, Google Gemini, and Ollama.
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProjectUnderstanding holds the AI-generated understanding of the project.
// Kept in this package so generator and openai packages import from here.
type ProjectUnderstanding struct {
	ProjectName      string   `json:"project_name"`
	Purpose          string   `json:"purpose"`
	Domain           string   `json:"domain"`
	Architecture     string   `json:"architecture"`
	KeyFeatures      []string `json:"key_features"`
	MainModules      []string `json:"main_modules"`
	APIEndpoints     []string `json:"api_endpoints"`
	ExternalServices []string `json:"external_services"`
	Conventions      []string `json:"conventions"`
	TechNotes        string   `json:"tech_notes"`
	WhatClaudeKnows  string   `json:"what_claude_should_know"`
}

// Provider enumerates supported AI backends
type Provider string

const (
	ProviderOpenAI Provider = "openai"
	ProviderGemini Provider = "gemini"
	ProviderOllama Provider = "ollama"
)

// Config is the minimal config needed to call an LLM
type Config struct {
	Provider  Provider
	Model     string
	APIKey    string   // not needed for Ollama
	OllamaURL string   // only for Ollama (default: http://localhost:11434)
}

// UnderstandProject dispatches to the correct provider
func UnderstandProject(cfg Config, prompt string) (*ProjectUnderstanding, error) {
	// Truncate to avoid context limits
	if len(prompt) > 80000 {
		prompt = prompt[:80000] + "\n\n[ content truncated ]"
	}

	var raw string
	var err error

	switch cfg.Provider {
	case ProviderGemini:
		raw, err = callGemini(cfg.APIKey, cfg.Model, prompt)
	case ProviderOllama:
		url := cfg.OllamaURL
		if url == "" {
			url = "http://localhost:11434"
		}
		raw, err = callOllama(url, cfg.Model, prompt)
	default: // openai
		raw, err = callOpenAI(cfg.APIKey, cfg.Model, prompt)
	}

	if err != nil {
		return nil, err
	}

	return parseJSON(raw)
}

// ─── OpenAI ──────────────────────────────────────────────────────────────────

type openAIRequest struct {
	Model       string            `json:"model"`
	Messages    []openAIMessage   `json:"messages"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct{ Content string `json:"content"` } `json:"message"`
	} `json:"choices"`
	Error *struct{ Message string `json:"message"` } `json:"error"`
}

func callOpenAI(apiKey, model, prompt string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not set — run: ccbootstrap settings")
	}
	body, _ := json.Marshal(openAIRequest{
		Model:       model,
		Messages:    []openAIMessage{{Role: "user", Content: prompt}},
		Temperature: 0.1,
		MaxTokens:   2048,
	})
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpDo(req)
	if err != nil {
		return "", err
	}

	var r openAIResponse
	if err := json.Unmarshal(resp, &r); err != nil {
		return "", fmt.Errorf("OpenAI parse error: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("OpenAI error: %s", r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("OpenAI returned empty response")
	}
	return r.Choices[0].Message.Content, nil
}

// ─── Google Gemini ────────────────────────────────────────────────────────────

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
	GenerationConfig struct {
		Temperature     float64 `json:"temperature"`
		MaxOutputTokens int     `json:"maxOutputTokens"`
	} `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func callGemini(apiKey, model, prompt string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("Gemini API key not set — run: ccbootstrap settings")
	}
	reqBody := geminiRequest{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}},
	}
	reqBody.GenerationConfig.Temperature = 0.1
	reqBody.GenerationConfig.MaxOutputTokens = 2048

	body, _ := json.Marshal(reqBody)
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpDo(req)
	if err != nil {
		return "", err
	}

	var r geminiResponse
	if err := json.Unmarshal(resp, &r); err != nil {
		return "", fmt.Errorf("Gemini parse error: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("Gemini error %d: %s", r.Error.Code, r.Error.Message)
	}
	if len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("Gemini returned empty response")
	}
	return r.Candidates[0].Content.Parts[0].Text, nil
}

// ─── Ollama ───────────────────────────────────────────────────────────────────
// Uses the OpenAI-compatible endpoint (/v1/chat/completions) available in Ollama 0.1.24+

func callOllama(baseURL, model, prompt string) (string, error) {
	body, _ := json.Marshal(openAIRequest{
		Model:       model,
		Messages:    []openAIMessage{{Role: "user", Content: prompt}},
		Temperature: 0.1,
		MaxTokens:   2048,
	})
	url := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer ollama")
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpDo(req)
	if err != nil {
		return "", fmt.Errorf("Ollama unreachable at %s (is it running?): %w", baseURL, err)
	}

	var r openAIResponse
	if err := json.Unmarshal(resp, &r); err != nil {
		return "", fmt.Errorf("Ollama parse error: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("Ollama error: %s", r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("Ollama returned empty response")
	}
	return r.Choices[0].Message.Content, nil
}

// ─── Shared helpers ───────────────────────────────────────────────────────────

func httpDo(req *http.Request) ([]byte, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func parseJSON(raw string) (*ProjectUnderstanding, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var u ProjectUnderstanding
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		// Fallback: return raw as purpose text
		return &ProjectUnderstanding{
			Purpose:         raw[:min(300, len(raw))],
			WhatClaudeKnows: raw[:min(600, len(raw))],
		}, nil
	}
	return &u, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ─── Model lists (for UI) ─────────────────────────────────────────────────────

var OpenAIModels = []string{
	"gpt-4o-mini",
	"gpt-4o",
	"o1-mini",
	"o1",
}

var GeminiModels = []string{
	"gemini-2.0-flash",
	"gemini-2.0-flash-lite",
	"gemini-1.5-pro",
	"gemini-1.5-flash",
	"gemini-2.5-pro-preview-03-25",
}

var OllamaModels = []string{
	"llama3.2",
	"llama3.1",
	"mistral",
	"mistral-nemo",
	"codellama",
	"deepseek-coder-v2",
	"qwen2.5-coder",
	"phi4",
	"gemma3",
	"mixtral",
}
