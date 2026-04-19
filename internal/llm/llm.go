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
	Provider       Provider
	Model          string
	APIKey         string   // not needed for Ollama
	OllamaURL      string   // only for Ollama
	MaxContextChars int     // 0 = no limit. Set per provider (Gemini 1M = ~4M chars)
}

// UnderstandProject dispatches to the correct provider
func UnderstandProject(cfg Config, prompt string) (*ProjectUnderstanding, error) {
	raw, err := CallLLM(cfg, prompt)
	if err != nil {
		return nil, err
	}
	return parseJSON(raw)
}

// CallLLM is a generic dispatcher that calls the configured provider and
// returns raw text. Used by UnderstandProject, wizard, proposals, and generator.
func CallLLM(cfg Config, prompt string) (string, error) {
	if cfg.MaxContextChars > 0 && len(prompt) > cfg.MaxContextChars {
		prompt = prompt[:cfg.MaxContextChars] + "\n\n[ content truncated to fit context limit ]"
	}

	switch cfg.Provider {
	case ProviderGemini:
		return callGemini(cfg.APIKey, cfg.Model, prompt)
	case ProviderOllama:
		url := cfg.OllamaURL
		if url == "" {
			url = "http://localhost:11434"
		}
		return callOllama(url, cfg.Model, prompt)
	default:
		return callOpenAI(cfg.APIKey, cfg.Model, prompt)
	}
}

// StripJSONFences removes markdown code fences and preamble/trailing text
// around a JSON payload returned by the LLM.
func StripJSONFences(raw string) string {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "```json"); idx != -1 {
		raw = raw[idx+7:]
		if end := strings.LastIndex(raw, "```"); end != -1 {
			raw = raw[:end]
		}
	} else if idx := strings.Index(raw, "```"); idx != -1 {
		raw = raw[idx+3:]
		if end := strings.LastIndex(raw, "```"); end != -1 {
			raw = raw[:end]
		}
	}
	raw = strings.TrimSpace(raw)
	if start := strings.Index(raw, "{"); start > 0 {
		raw = raw[start:]
	}
	if end := strings.LastIndex(raw, "}"); end != -1 && end < len(raw)-1 {
		raw = raw[:end+1]
	}
	return raw
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
		MaxTokens:   16384, // large enough for full file generation step
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

	type Part struct {
		Text string `json:"text"`
	}
	type Content struct {
		Parts []Part `json:"parts"`
	}
	type GenConfig struct {
		Temperature     float64 `json:"temperature"`
		MaxOutputTokens int     `json:"maxOutputTokens"`
	}
	type GeminiReq struct {
		Contents         []Content `json:"contents"`
		GenerationConfig GenConfig `json:"generationConfig"`
	}

	reqBody := GeminiReq{
		Contents: []Content{{Parts: []Part{{Text: prompt}}}},
		GenerationConfig: GenConfig{
			Temperature:     0.1,
			MaxOutputTokens: 16384, // Gemini 2.x supports up to 65535
		},
	}

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
		MaxTokens:   8192,
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
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func parseJSON(raw string) (*ProjectUnderstanding, error) {
	raw = StripJSONFences(raw)

	var u ProjectUnderstanding
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		// Last resort fallback: return raw as purpose text
		return &ProjectUnderstanding{
			Purpose:         "[Parse error — raw response: " + raw[:min(200, len(raw))] + "]",
			WhatClaudeKnows: raw[:min(500, len(raw))],
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
	// GPT-5 series
	"gpt-5.4",          // Main — best overall
	"gpt-5.4-mini",     // Fast
	"gpt-5.4-nano",     // Cheap
	"gpt-5.4-pro",      // Deep reasoning
	"gpt-5.3-codex",    // Coding specialized
	// GPT-4o series (stable)
	"gpt-4o",
	"gpt-4o-mini",
	// o1 reasoning
	"o1-mini",
	"o1",
}

var GeminiModels = []string{
	// Gemini 3 previews (latest)
	"gemini-3.1-pro-preview",        // Main — multimodal native
	"gemini-3-flash-preview",        // Fast
	"gemini-3.1-flash-lite-preview", // Cheap
	// Gemini 2.5 stable
	"gemini-2.5-pro",                // Stable, deep reasoning
	"gemini-2.5-flash",              // Stable, fast
	// Alias
	"gemini-pro-latest",             // Auto hot-swap latest
	// Gemini 2.0
	"gemini-2.0-flash",
	"gemini-2.0-flash-lite",
	"gemini-1.5-pro",                // Long context (1M tokens)
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

