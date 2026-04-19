// Package skills queries the skills.sh directory API and fetches raw SKILL.md
// content from the underlying GitHub repositories.
package skills

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	searchEndpoint = "https://skills.sh/api/search"
	defaultLimit   = 100
	httpTimeout    = 20 * time.Second
)

// Skill is a single result returned by the skills.sh search API,
// optionally augmented with the raw SKILL.md content once fetched.
type Skill struct {
	ID       string `json:"id"`       // e.g. "supabase/agent-skills/supabase-postgres-best-practices"
	SkillID  string `json:"skillId"`  // e.g. "supabase-postgres-best-practices"
	Name     string `json:"name"`
	Source   string `json:"source"`   // e.g. "supabase/agent-skills"
	Installs int    `json:"installs"`

	// Populated only after a call to FetchContent.
	Description string `json:"-"`
	Content     string `json:"-"`
}

type searchResponse struct {
	Query      string  `json:"query"`
	SearchType string  `json:"searchType"`
	Skills     []Skill `json:"skills"`
	Count      int     `json:"count"`
	DurationMs int     `json:"duration_ms"`
}

var httpClient = &http.Client{Timeout: httpTimeout}

// Search queries skills.sh for a list of matching skills. Pass limit=0 to
// use the default (100).
func Search(query string, limit int) ([]Skill, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty query")
	}

	u, _ := url.Parse(searchEndpoint)
	qs := u.Query()
	qs.Set("q", q)
	qs.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = qs.Encode()

	resp, err := httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("skills.sh search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("skills.sh returned %d", resp.StatusCode)
	}

	var out searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("skills.sh decode: %w", err)
	}
	return out.Skills, nil
}

// FetchContent downloads the raw SKILL.md for the given skill from its
// underlying GitHub repository and populates s.Content and s.Description.
// It tries several branch/path combinations because the repo layout varies:
// some place SKILL.md directly at <skillId>/SKILL.md, others nest them
// under skills/<skillId>/SKILL.md.
func FetchContent(s *Skill) error {
	if s.ID == "" || s.Source == "" {
		return fmt.Errorf("skill missing ID or Source")
	}
	// path-within-repo derived from the skills.sh id
	rel := strings.TrimPrefix(s.ID, s.Source+"/")
	if rel == "" || rel == s.ID {
		return fmt.Errorf("could not derive repo-relative path for %q", s.ID)
	}

	branches := []string{"HEAD", "main", "master"}
	prefixes := []string{"", "skills/"}

	var lastErr error
	for _, branch := range branches {
		for _, prefix := range prefixes {
			raw := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s%s/SKILL.md",
				s.Source, branch, prefix, rel)
			resp, err := httpClient.Get(raw)
			if err != nil {
				lastErr = err
				continue
			}
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
				continue
			}
			if resp.StatusCode == 200 && len(body) > 0 {
				s.Content = string(body)
				s.Description = extractDescription(s.Content)
				return nil
			}
			lastErr = fmt.Errorf("%s%s @ %s: HTTP %d", prefix, rel, branch, resp.StatusCode)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("not found in any default branch or layout")
	}
	return fmt.Errorf("fetch SKILL.md for %s: %w", s.ID, lastErr)
}

// Reachable checks whether the skills.sh API responds. Used by `ccb doctor`.
func Reachable() error {
	_, err := Search("test", 1)
	return err
}

// extractDescription pulls the `description:` field out of a SKILL.md YAML
// frontmatter block, returning the first 200 chars at most.
func extractDescription(content string) string {
	lines := strings.Split(content, "\n")
	inFront := false
	fences := 0
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "---" {
			fences++
			if fences == 1 {
				inFront = true
				continue
			}
			if fences == 2 {
				return ""
			}
		}
		if inFront && strings.HasPrefix(t, "description:") {
			val := strings.TrimSpace(strings.TrimPrefix(t, "description:"))
			val = strings.Trim(val, `"'`)
			if len(val) > 200 {
				val = val[:197] + "…"
			}
			return val
		}
	}
	return ""
}
