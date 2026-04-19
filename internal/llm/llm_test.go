package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeJSONStrings(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantKey  string // value we expect after parsing
		wantOK   bool   // must parse as valid JSON
	}{
		{
			name:    "plain valid JSON passes through",
			in:      `{"x":"hello"}`,
			wantKey: "hello",
			wantOK:  true,
		},
		{
			name:    "raw newline in string is escaped",
			in:      "{\"x\":\"line1\nline2\"}",
			wantKey: "line1\nline2",
			wantOK:  true,
		},
		{
			name:    "invalid escape backslash-paren is preserved",
			in:      `{"x":"text\)more"}`,
			wantKey: `text\)more`,
			wantOK:  true,
		},
		{
			name:    "invalid escape backslash-bang is preserved",
			in:      `{"x":"hey\!"}`,
			wantKey: `hey\!`,
			wantOK:  true,
		},
		{
			name:    "valid backslash-n is untouched",
			in:      `{"x":"a\nb"}`,
			wantKey: "a\nb",
			wantOK:  true,
		},
		{
			name:    "valid backslash-quote is untouched",
			in:      `{"x":"say \"hi\""}`,
			wantKey: `say "hi"`,
			wantOK:  true,
		},
		{
			name:    "double backslash stays double",
			in:      `{"x":"path\\here"}`,
			wantKey: `path\here`,
			wantOK:  true,
		},
		{
			name:    "malformed unicode escape is preserved",
			in:      `{"x":"\uZZZZ"}`,
			wantKey: `\uZZZZ`,
			wantOK:  true,
		},
		{
			name:    "raw tab in string is escaped",
			in:      "{\"x\":\"a\tb\"}",
			wantKey: "a\tb",
			wantOK:  true,
		},
		{
			name:    "content containing paren escape from the real failure",
			in:      `{"x":"contributions.\)"}`,
			wantKey: `contributions.\)`,
			wantOK:  true,
		},
		{
			name:    "markdown fences are stripped",
			in:      "```json\n{\"x\":\"ok\"}\n```",
			wantKey: "ok",
			wantOK:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleaned := StripJSONFences(tc.in)

			var m map[string]string
			err := json.Unmarshal([]byte(cleaned), &m)
			if tc.wantOK && err != nil {
				t.Fatalf("expected valid JSON, got error: %v\ninput:   %q\ncleaned: %q", err, tc.in, cleaned)
			}
			if !tc.wantOK {
				if err == nil {
					t.Fatalf("expected parse failure, got success: %q", cleaned)
				}
				return
			}
			if got := m["x"]; got != tc.wantKey {
				t.Fatalf("value mismatch:\n  input:   %q\n  cleaned: %q\n  got:     %q\n  want:    %q",
					tc.in, cleaned, got, tc.wantKey)
			}
		})
	}
}

func TestSanitizeJSONStrings_outsideStringsUntouched(t *testing.T) {
	// Backslashes outside string literals should not be affected (they're
	// illegal in JSON anyway, but our sanitizer must not introduce bugs).
	in := `{ "a": 1, "b": "x" }`
	out := StripJSONFences(in)
	if !strings.Contains(out, `"a": 1`) {
		t.Fatalf("structure corrupted: %q", out)
	}
}
