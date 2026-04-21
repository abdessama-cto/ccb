package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/abdessama-cto/ccb/internal/analyzer"
)

// GeneratedAgent is a single Claude Code subagent produced by the LLM.
type GeneratedAgent struct {
	Filename string `json:"filename"`
	Name     string `json:"name"`
	Content  string `json:"content"`
}

// GenerateAgent asks the LLM to produce one agent tailored to the project.
// If nameHint is non-empty, the agent is created around that topic/name.
// Otherwise, the LLM picks a useful gap to fill.
func GenerateAgent(cfg Config, u *ProjectUnderstanding, fp *analyzer.ProjectFingerprint, nameHint string) (*GeneratedAgent, error) {
	prompt := buildAgentPrompt(u, fp, nameHint) + "\n\n" + LanguageDirective(cfg)
	raw, err := CallLLM(cfg, prompt)
	if err != nil {
		return nil, err
	}
	return parseAgent(raw)
}

func buildAgentPrompt(u *ProjectUnderstanding, fp *analyzer.ProjectFingerprint, nameHint string) string {
	var sb strings.Builder
	sb.WriteString(`You are creating a Claude Code subagent (.claude/agents/*.md) tailored to a specific project.

The agent file MUST have TWO parts:

1. YAML frontmatter:
   ---
   name: kebab-case-name
   description: one sentence that makes it obvious when Claude should invoke this agent (include example trigger phrases from this project's domain).
   tools: Read, Edit, Bash
   ---

   Valid tool names (use ONLY these, comma-separated — pick what the agent
   actually needs, not a kitchen sink):

     Read, Write, Edit, Bash, Grep, Glob, WebFetch, WebSearch, Agent, BashOutput, KillShell, Skill, Task, TodoWrite, NotebookEdit

2. A body of at least 25 lines, structured with these sections:

   ## When to use this agent
   - 2-3 concrete trigger scenarios grounded in THIS codebase.

   ## How this agent works
   - Numbered step-by-step approach the agent follows.

   ## Relevant files in this project
   - 3-8 real file paths the agent touches, with a one-line purpose each.

   (Optional) ## Examples or ## Common pitfalls

   An agent with only frontmatter is unusable — always include the body.

Phrase everything as guidance grounded in the project, not as commandments.
Keep the total file under 150 lines.

`)

	if nameHint != "" {
		sb.WriteString(fmt.Sprintf("The user asked for an agent named/focused on: %q.\n", nameHint))
		sb.WriteString("Use that as the agent name (kebab-case) and build the content around that role.\n\n")
	} else {
		sb.WriteString("The user did not specify a name — pick the single MOST VALUABLE missing agent for this project\n")
		sb.WriteString("based on the understanding below, and produce it.\n\n")
	}

	sb.WriteString(`Return ONLY this JSON (no markdown fences, no preamble):
{
  "filename": "kebab-case.md",
  "name": "kebab-case-name",
  "content": "---\nname: ...\ndescription: ...\ntools: Read, Edit, Bash\n---\n\n# ...\n\n..."
}

`)

	sb.WriteString("## Project understanding\n")
	if b, err := json.MarshalIndent(u, "", "  "); err == nil {
		sb.Write(b)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Stack\n")
	sb.WriteString(fp.StackString())
	sb.WriteString("\n")
	return sb.String()
}

func parseAgent(raw string) (*GeneratedAgent, error) {
	raw = StripJSONFences(raw)
	var a GeneratedAgent
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		preview := raw
		if len(preview) > 300 {
			preview = preview[:300]
		}
		return nil, fmt.Errorf("agent JSON parse failed: %w (raw: %s)", err, preview)
	}

	a.Filename = strings.TrimSpace(a.Filename)
	if a.Filename == "" {
		a.Filename = a.Name
	}
	a.Filename = strings.ToLower(strings.ReplaceAll(a.Filename, " ", "-"))
	if !strings.HasSuffix(a.Filename, ".md") {
		a.Filename += ".md"
	}
	if strings.TrimSpace(a.Content) == "" {
		return nil, fmt.Errorf("LLM returned empty agent content")
	}
	return &a, nil
}
