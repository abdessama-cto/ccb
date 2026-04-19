package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProjectUnderstanding holds the AI-generated understanding of the project
type ProjectUnderstanding struct {
	ProjectName        string   `json:"project_name"`
	Purpose            string   `json:"purpose"`
	Domain             string   `json:"domain"`
	Architecture       string   `json:"architecture"`
	KeyFeatures        []string `json:"key_features"`
	MainModules        []string `json:"main_modules"`
	APIEndpoints       []string `json:"api_endpoints"`
	ExternalServices   []string `json:"external_services"`
	Conventions        []string `json:"conventions"`
	TechNotes          string   `json:"tech_notes"`
	WhatClaudeKnows    string   `json:"what_claude_should_know"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// UnderstandProject sends key project files to OpenAI and returns a structured understanding
func UnderstandProject(apiKey, model, prompt string) (*ProjectUnderstanding, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("no OpenAI API key configured")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	// Truncate prompt to avoid exceeding context limits (~100k chars = ~25k tokens)
	if len(prompt) > 80000 {
		prompt = prompt[:80000] + "\n\n[ ... content truncated to fit context ... ]"
	}

	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   2048,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body error: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("OpenAI error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	rawContent := chatResp.Choices[0].Message.Content

	// Strip markdown code fences if present
	rawContent = strings.TrimSpace(rawContent)
	rawContent = strings.TrimPrefix(rawContent, "```json")
	rawContent = strings.TrimPrefix(rawContent, "```")
	rawContent = strings.TrimSuffix(rawContent, "```")
	rawContent = strings.TrimSpace(rawContent)

	var understanding ProjectUnderstanding
	if err := json.Unmarshal([]byte(rawContent), &understanding); err != nil {
		// Try to extract project name and purpose from raw text as fallback
		return &ProjectUnderstanding{
			Purpose:         rawContent[:min(200, len(rawContent))],
			WhatClaudeKnows: rawContent[:min(500, len(rawContent))],
		}, nil
	}

	return &understanding, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
