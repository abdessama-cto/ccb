package cmd

import (
	"fmt"
	"strings"

	"github.com/abdessama-cto/ccb/internal/llm"
	"github.com/abdessama-cto/ccb/internal/tui"
)

// ConfirmProposals asks the LLM to propose agents/rules/skills tailored to the
// project, then presents three interactive checkboxes so the user can confirm.
// It returns the proposals with their Selected flags updated.
func ConfirmProposals(proposals *llm.Proposals) *llm.Proposals {
	if proposals == nil {
		return &llm.Proposals{}
	}

	printProposalHeader()

	confirmAgents(proposals)
	confirmRules(proposals)
	confirmSkills(proposals)

	return proposals
}

func printProposalHeader() {
	line := strings.Repeat("─", 68)
	fmt.Printf("\n%s\n  %s\n%s\n", line, tui.Bold("🤖 AI Proposals — agents, rules, skills tailored to this project"), line)
	fmt.Printf("  %s\n\n", tui.Dim("Select the ones you want. Unselected items will not be written to disk."))
}

func confirmAgents(p *llm.Proposals) {
	if len(p.Agents) == 0 {
		tui.Warn("No agents proposed by the AI — skipping")
		return
	}
	items := make([]CheckItem, len(p.Agents))
	for i, a := range p.Agents {
		detail := a.Description
		if a.Reason != "" {
			detail = a.Description + "  ·  " + a.Reason
		}
		items[i] = CheckItem{Label: a.Name, Detail: detail, Selected: a.Selected}
	}
	result := InteractiveCheckbox(
		"🤖 AGENTS — .claude/agents/",
		"Subagents tailored to this project. SPACE to toggle, ENTER to confirm.",
		items,
		false,
	)
	for i := range p.Agents {
		p.Agents[i].Selected = result[i].Selected
	}
}

func confirmRules(p *llm.Proposals) {
	if len(p.Rules) == 0 {
		tui.Warn("No project-specific rules proposed by the AI — skipping")
		return
	}
	items := make([]CheckItem, len(p.Rules))
	for i, r := range p.Rules {
		items[i] = CheckItem{Label: truncate(r.Rule, 42), Detail: truncate(r.Reason, 36), Selected: r.Selected}
	}
	result := InteractiveCheckbox(
		"📐 PROJECT RULES — .claude/rules/05-project-specific.md",
		"Rules the AI extracted from your codebase. Confirm which to enforce.",
		items,
		false,
	)
	for i := range p.Rules {
		p.Rules[i].Selected = result[i].Selected
	}
}

func confirmSkills(p *llm.Proposals) {
	// Render the checkbox even if AI proposed zero skills — the user can still
	// add skills from skills.sh via "/" search.
	items := make([]CheckItem, len(p.Skills))
	for i, s := range p.Skills {
		detail := s.Description
		if s.Reason != "" {
			detail = s.Description + "  ·  " + s.Reason
		}
		items[i] = CheckItem{Label: s.Name, Detail: detail, Selected: s.Selected}
	}
	result := InteractiveCheckbox(
		"🔧 SKILLS — .claude/skills/",
		"Skills teach Claude specific methodologies. Press [/] to search skills.sh.",
		items,
		true,
	)

	// Update existing proposals
	for i := range p.Skills {
		if i < len(result) {
			p.Skills[i].Selected = result[i].Selected
		}
	}
	// Append newly-added skills from the skills.sh search
	for i := len(p.Skills); i < len(result); i++ {
		it := result[i]
		if it.SkillRef == nil {
			continue
		}
		ref := it.SkillRef
		p.Skills = append(p.Skills, llm.SkillProposal{
			ID:             "skillssh-" + ref.SkillID,
			Filename:       ref.SkillID + ".md",
			Name:           ref.SkillID,
			Description:    ref.Name,
			Reason:         "Added from skills.sh (" + ref.Source + ")",
			Selected:       it.Selected,
			ExternalID:     ref.ID,
			ExternalSource: ref.Source,
		})
	}
}

// SelectedAgents returns only the agents the user kept checked.
func SelectedAgents(p *llm.Proposals) []llm.AgentProposal {
	out := make([]llm.AgentProposal, 0, len(p.Agents))
	for _, a := range p.Agents {
		if a.Selected {
			out = append(out, a)
		}
	}
	return out
}

// SelectedRules returns only the rules the user kept checked.
func SelectedRules(p *llm.Proposals) []llm.RuleProposal {
	out := make([]llm.RuleProposal, 0, len(p.Rules))
	for _, r := range p.Rules {
		if r.Selected {
			out = append(out, r)
		}
	}
	return out
}

// SelectedSkills returns only the skills the user kept checked.
func SelectedSkills(p *llm.Proposals) []llm.SkillProposal {
	out := make([]llm.SkillProposal, 0, len(p.Skills))
	for _, s := range p.Skills {
		if s.Selected {
			out = append(out, s)
		}
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
