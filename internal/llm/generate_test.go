package llm

import (
	"strings"
	"testing"
)

func TestScanFileBlocks_basicTwoFiles(t *testing.T) {
	raw := `some preamble the LLM emitted
=== FILE: CLAUDE.md ===
# Project

Prose with "quotes" and \backslashes\ — no escaping needed.
Multi-line content.
=== END FILE ===

=== FILE: .claude/rules/01-core.md ===
Rule 1
Rule 2
=== END FILE ===

trailing chatter that should be ignored`

	files := scanFileBlocks(raw)
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d", len(files))
	}
	if files[0].Path != "CLAUDE.md" {
		t.Fatalf("file0 path: %q", files[0].Path)
	}
	if !strings.Contains(files[0].Content, `"quotes"`) {
		t.Fatalf("file0 content missing quotes: %q", files[0].Content)
	}
	if !strings.Contains(files[0].Content, `\backslashes\`) {
		t.Fatalf("file0 content missing backslashes")
	}
	if files[1].Path != ".claude/rules/01-core.md" {
		t.Fatalf("file1 path: %q", files[1].Path)
	}
}

func TestScanFileBlocks_unclosedFinalBlock(t *testing.T) {
	// LLM ran out of tokens mid-way — we still want the partial file.
	raw := `=== FILE: CLAUDE.md ===
# Partial content
more lines`
	files := scanFileBlocks(raw)
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d", len(files))
	}
	if !strings.Contains(files[0].Content, "Partial content") {
		t.Fatalf("missing content: %q", files[0].Content)
	}
}

func TestScanFileBlocks_backToBackWithoutClose(t *testing.T) {
	// LLM emitted two opens in a row, forgetting the close.
	raw := `=== FILE: a.md ===
body A
=== FILE: b.md ===
body B
=== END FILE ===`
	files := scanFileBlocks(raw)
	if len(files) != 2 {
		t.Fatalf("want 2 files, got %d", len(files))
	}
	if !strings.Contains(files[0].Content, "body A") || strings.Contains(files[0].Content, "body B") {
		t.Fatalf("file0 leaked or missed content: %q", files[0].Content)
	}
	if !strings.Contains(files[1].Content, "body B") {
		t.Fatalf("file1 missing content: %q", files[1].Content)
	}
}

func TestScanFileBlocks_preservesInnerCodeFences(t *testing.T) {
	// Markdown content typically contains ``` fences — must pass through.
	raw := "=== FILE: docs.md ===\n" +
		"# Title\n" +
		"```go\n" +
		"func main() {}\n" +
		"```\n" +
		"=== END FILE ==="
	files := scanFileBlocks(raw)
	if len(files) != 1 {
		t.Fatalf("want 1, got %d", len(files))
	}
	if !strings.Contains(files[0].Content, "```go") {
		t.Fatalf("fence stripped: %q", files[0].Content)
	}
}

func TestParseGeneration_fallsBackToJSON(t *testing.T) {
	// If the LLM stubbornly returns JSON, we still accept it.
	raw := `{"files":[{"path":"a.md","content":"hi"}]}`
	res, err := parseGeneration(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res.Files) != 1 || res.Files[0].Content != "hi" {
		t.Fatalf("bad result: %+v", res)
	}
}

func TestParseGeneration_delimiterFormat(t *testing.T) {
	raw := "=== FILE: CLAUDE.md ===\nhello\n=== END FILE ==="
	res, err := parseGeneration(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res.Files) != 1 || res.Files[0].Path != "CLAUDE.md" {
		t.Fatalf("bad result: %+v", res)
	}
	if !strings.Contains(res.Files[0].Content, "hello") {
		t.Fatalf("missing content: %q", res.Files[0].Content)
	}
}

func TestSanitizePaths_dropsUnsafe(t *testing.T) {
	in := []GeneratedFile{
		{Path: "CLAUDE.md", Content: "ok"},
		{Path: "../etc/passwd", Content: "bad"},
		{Path: "/abs/path", Content: "bad"},
		{Path: "  ", Content: "bad"},
		{Path: "`quoted.md`", Content: "ok"},
	}
	out := sanitizePaths(in)
	if len(out) != 2 {
		t.Fatalf("want 2 kept, got %d: %+v", len(out), out)
	}
	if out[1].Path != "quoted.md" {
		t.Fatalf("backticks not trimmed: %q", out[1].Path)
	}
}
