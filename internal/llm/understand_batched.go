package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abdessama-cto/ccb/internal/analyzer"
)

// UnderstandProjectSmart chooses between single-call and batched analysis
// based on the total content size relative to the LLM budget.
//
//   - If all content fits in one call: single-call path (cheapest, highest quality).
//   - If content exceeds the budget: batched pipeline (N per-batch summaries
//     + one synthesis call).
//
// Progress is reported via the optional callback so the caller can animate a
// spinner with stage labels.
func UnderstandProjectSmart(
	cfg Config,
	semCtx *analyzer.SemanticContext,
	fp *analyzer.ProjectFingerprint,
	onStage func(stage string),
) (*ProjectUnderstanding, error) {
	notify := func(s string) {
		if onStage != nil {
			onStage(s)
		}
	}

	budget := cfg.MaxContextChars
	if budget <= 0 {
		budget = 2_800_000
	}

	// Fast path: the single-call prompt fits comfortably.
	singlePrompt := analyzer.BuildAIPromptLimited(semCtx, fp, budget)
	if len(singlePrompt) <= budget {
		notify(fmt.Sprintf("Sending %dk chars in a single call", len(singlePrompt)/1000))
		return UnderstandProject(cfg, singlePrompt)
	}

	// Batched path: pack files greedily into batches that fit the budget, then
	// synthesize the final understanding from per-batch summaries.
	batches := splitIntoBatches(semCtx, fp, budget)
	notify(fmt.Sprintf("Codebase too large — using %d-batch analysis", len(batches)))

	summaries := make([]string, 0, len(batches))
	for i, prompt := range batches {
		notify(fmt.Sprintf("Analyzing batch %d/%d (%dk chars)...",
			i+1, len(batches), len(prompt)/1000))
		raw, err := CallLLM(cfg, prompt)
		if err != nil {
			return nil, fmt.Errorf("batch %d/%d failed: %w", i+1, len(batches), err)
		}
		summaries = append(summaries, raw)
	}

	notify(fmt.Sprintf("Synthesizing understanding from %d batch summaries...", len(summaries)))
	synthPrompt := buildSynthesisPrompt(summaries, fp, cfg)
	raw, err := CallLLM(cfg, synthPrompt)
	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}
	cleaned := StripJSONFences(raw)

	var u ProjectUnderstanding
	if err := json.Unmarshal([]byte(cleaned), &u); err != nil {
		// Fallback — at least surface the raw text to the user rather than failing.
		return &ProjectUnderstanding{
			Purpose:         "[synthesis parse failed — raw text in what_claude_should_know]",
			WhatClaudeKnows: cleaned,
		}, nil
	}
	return &u, nil
}

// splitIntoBatches greedily packs ranked files into batches whose prompt size
// stays under the budget. Structural files + the directory tree are included
// in every batch so each call has enough context to identify patterns.
func splitIntoBatches(semCtx *analyzer.SemanticContext, fp *analyzer.ProjectFingerprint, budget int) []string {
	ranked := analyzer.RankFiles(semCtx.Files)

	// Separate always-include files (dir tree, README, manifests) from the rest.
	alwaysKeys, restKeys := partitionAlways(ranked)

	// Reserve ~35% of the budget for the per-batch prompt scaffolding + the
	// always-include block. Batches get the remaining 65% for rotating files.
	scaffoldReserve := budget * 35 / 100
	batchBudget := budget - scaffoldReserve
	if batchBudget < 50_000 {
		batchBudget = 50_000
	}

	alwaysBlock := buildFileBlock(semCtx, alwaysKeys)

	var batches []string
	var curKeys []string
	curSize := len(alwaysBlock)

	flush := func(idx, total int) {
		if len(curKeys) == 0 {
			return
		}
		prompt := buildBatchPrompt(fp, alwaysBlock, buildFileBlock(semCtx, curKeys), idx, total)
		batches = append(batches, prompt)
		curKeys = nil
		curSize = len(alwaysBlock)
	}

	for _, k := range restKeys {
		chunkLen := estimateFileChunkLen(k, semCtx.Files[k])
		if curSize+chunkLen > batchBudget && len(curKeys) > 0 {
			flush(0, 0) // index filled in pass 2
		}
		curKeys = append(curKeys, k)
		curSize += chunkLen
	}
	flush(0, 0)

	// Rebuild with correct batch-index labels now that we know the total.
	total := len(batches)
	if total == 0 {
		// Nothing left after always-include — run a single batch with just the always-block.
		return []string{buildBatchPrompt(fp, alwaysBlock, "", 1, 1)}
	}
	labeled := make([]string, 0, total)
	batchIdx := 0
	curKeys = nil
	curSize = len(alwaysBlock)
	for _, k := range restKeys {
		chunkLen := estimateFileChunkLen(k, semCtx.Files[k])
		if curSize+chunkLen > batchBudget && len(curKeys) > 0 {
			batchIdx++
			labeled = append(labeled, buildBatchPrompt(fp, alwaysBlock, buildFileBlock(semCtx, curKeys), batchIdx, total))
			curKeys = nil
			curSize = len(alwaysBlock)
		}
		curKeys = append(curKeys, k)
		curSize += chunkLen
	}
	if len(curKeys) > 0 {
		batchIdx++
		labeled = append(labeled, buildBatchPrompt(fp, alwaysBlock, buildFileBlock(semCtx, curKeys), batchIdx, total))
	}
	if len(labeled) == 0 {
		return []string{buildBatchPrompt(fp, alwaysBlock, "", 1, 1)}
	}
	return labeled
}

// partitionAlways splits ranked keys into (always-included, rest). The
// always-included set is small and shared across every batch.
func partitionAlways(ranked []string) (always, rest []string) {
	alwaysSet := map[string]bool{
		"__dir_tree__":     true,
		"README.md":        true,
		"readme.md":        true,
		"package.json":     true,
		"composer.json":    true,
		"go.mod":           true,
		"pyproject.toml":   true,
		"Gemfile":          true,
		"Cargo.toml":       true,
		"tsconfig.json":    true,
		"next.config.ts":   true,
		"next.config.js":   true,
		"nuxt.config.ts":   true,
		"nuxt.config.js":   true,
		"Dockerfile":       true,
		"docker-compose.yml": true,
	}
	for _, k := range ranked {
		if alwaysSet[k] {
			always = append(always, k)
		} else {
			rest = append(rest, k)
		}
	}
	return always, rest
}

func buildFileBlock(semCtx *analyzer.SemanticContext, keys []string) string {
	var sb strings.Builder
	for _, k := range keys {
		content, ok := semCtx.Files[k]
		if !ok {
			continue
		}
		sb.WriteString("=== ")
		sb.WriteString(k)
		sb.WriteString(" ===\n")
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func estimateFileChunkLen(path, content string) int {
	// Header format matches buildFileBlock: "=== path ===\ncontent\n\n"
	return len(path) + len(content) + 12
}

func buildBatchPrompt(fp *analyzer.ProjectFingerprint, alwaysBlock, batchBlock string, idx, total int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"You are analyzing batch %d of %d of a larger codebase. Your job for THIS batch is to emit a concise Markdown summary of what you observe in the files below.\n\n",
		idx, total,
	))
	sb.WriteString(fmt.Sprintf("Detected stack: %s\nTotal project files: %d\n\n", fp.StackString(), fp.Files))
	sb.WriteString(`Format your response as plain Markdown (NOT JSON) with these sections:

## Key modules seen in this batch
- module_or_dir: one-line purpose

## API endpoints / public interfaces
- METHOD /path — purpose (or function signature for libraries)

## External services / integrations
- ServiceName: how it's used

## Patterns & conventions observed
- Pattern/convention with a short example

## Domain clues
- Anything that hints at the business domain or user-facing purpose

Keep the whole summary under 3000 characters. Do not invent content that isn't in the files. If a section is empty, write "none observed in this batch".

`)
	sb.WriteString("## Always-included files (shared across all batches)\n")
	sb.WriteString(alwaysBlock)
	if batchBlock != "" {
		sb.WriteString(fmt.Sprintf("## Files in batch %d/%d\n", idx, total))
		sb.WriteString(batchBlock)
	}
	return sb.String()
}

func buildSynthesisPrompt(summaries []string, fp *analyzer.ProjectFingerprint, cfg Config) string {
	var sb strings.Builder
	sb.WriteString(`You have received per-batch Markdown summaries of a codebase analyzed in multiple passes. Combine them into a single JSON ProjectUnderstanding.

Return ONLY the JSON object below (no markdown fences, no preamble):

{
  "project_name": "exact name",
  "purpose": "2-3 sentences: what this app does, who uses it, business value",
  "domain": "e.g. e-commerce, SaaS platform, mobile backend",
  "architecture": "concise architecture description",
  "key_features": ["feature1", "feature2"],
  "main_modules": ["module_name: what it does"],
  "api_endpoints": ["METHOD /path — purpose"],
  "external_services": ["ServiceName: how it is used"],
  "conventions": ["naming or structural convention observed"],
  "tech_notes": "important patterns, gotchas, architectural decisions",
  "what_claude_should_know": "essential context for an AI assistant working on this codebase daily"
}

Merge rules:
- De-duplicate items that appear in multiple batch summaries.
- If two summaries conflict, prefer the more specific one.
- If a section was empty across all batches, use an empty array (or empty string for scalars).

`)
	sb.WriteString(fmt.Sprintf("Detected stack: %s\nTotal files: %d\n\n", fp.StackString(), fp.Files))
	for i, s := range summaries {
		sb.WriteString(fmt.Sprintf("## Batch %d summary\n%s\n\n", i+1, strings.TrimSpace(s)))
	}
	sb.WriteString("\n" + LanguageDirective(cfg))
	return sb.String()
}
