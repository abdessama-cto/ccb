package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdessama-cto/ccb/internal/analyzer"
	"github.com/abdessama-cto/ccb/internal/llm"
	"github.com/abdessama-cto/ccb/internal/tui"
)

// ─── Data structures ──────────────────────────────────────────────────────────

// Proposals holds everything the user confirmed in the AI proposal step
type Proposals struct {
	Agents []AgentProposal
	Rules  []RuleProposal
	Skills []SkillProposal
}

// AgentProposal is a Claude Code subagent (.claude/agents/*.md)
type AgentProposal struct {
	Filename string
	Name     string
	Desc     string
	Content  string
	Selected bool
}

// RuleProposal is a project-specific rule extracted from AI understanding
type RuleProposal struct {
	Rule     string
	Source   string // "ai-convention" | "tech-notes" | "domain"
	Selected bool
}

// SkillProposal is a skill file to add to .claude/skills/
type SkillProposal struct {
	Filename string
	Name     string
	Desc     string
	Content  string
	Reason   string // why it was recommended
	Selected bool
}

// ─── Main entry point ─────────────────────────────────────────────────────────

// RunProposals proposes agents, rules, and skills based on AI understanding.
// Shows a confirmation UI, writes confirmed files to disk.
func RunProposals(destDir string, fp *analyzer.ProjectFingerprint, u *llm.ProjectUnderstanding) *Proposals {
	p := &Proposals{
		Agents: buildAgentProposals(fp, u),
		Rules:  buildRuleProposals(u),
		Skills: buildSkillProposals(fp, u),
	}

	printSectionHeader("🤖 AI Proposals — customised for " + u.ProjectName)
	fmt.Printf("  %s\n\n", tui.Dim("Based on AI understanding of your codebase, Claude proposes the following setup."))

	// Agents
	confirmAgents(p)

	// Project-specific rules
	confirmRules(p)

	// Skills
	confirmSkills(p)

	// Write all confirmed items to disk
	writeProposals(destDir, p, u)

	return p
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// ─── Section builders ─────────────────────────────────────────────────────────

func buildAgentProposals(fp *analyzer.ProjectFingerprint, u *llm.ProjectUnderstanding) []AgentProposal {
	agents := []AgentProposal{}
	projectName := u.ProjectName
	if projectName == "" {
		projectName = "this project"
	}
	arch := u.Architecture
	if arch == "" {
		arch = fp.StackString()
	}
	domain := u.Domain
	conventions := strings.Join(u.Conventions, "\n- ")
	techNotes := u.TechNotes

	// ── Always-present agents ────────────────────────────────────────────────

	agents = append(agents, AgentProposal{
		Filename: "code-reviewer.md",
		Name:     "code-reviewer",
		Desc:     "Reviews code changes for quality, security, and project conventions",
		Selected: true,
		Content:  agentCodeReviewer(projectName, arch, domain, conventions),
	})

	agents = append(agents, AgentProposal{
		Filename: "debugger.md",
		Name:     "debugger",
		Desc:     "Systematic bug investigation with hypothesis-driven approach",
		Selected: true,
		Content:  agentDebugger(projectName, arch, techNotes, fp),
	})

	agents = append(agents, AgentProposal{
		Filename: "test-writer.md",
		Name:     "test-writer",
		Desc:     "Writes comprehensive tests following project conventions",
		Selected: len(fp.TestFrameworks) > 0,
		Content:  agentTestWriter(projectName, arch, fp),
	})

	// ── Stack-specific ────────────────────────────────────────────────────────

	stackStr := strings.ToLower(fp.StackString())

	if strings.Contains(stackStr, "next") || strings.Contains(stackStr, "react") ||
		strings.Contains(stackStr, "vue") || strings.Contains(stackStr, "svelte") {
		agents = append(agents, AgentProposal{
			Filename: "frontend-specialist.md",
			Name:     "frontend-specialist",
			Desc:     fmt.Sprintf("Frontend expert for %s", fp.StackString()),
			Selected: true,
			Content:  agentFrontend(projectName, arch, fp),
		})
	}

	if len(u.APIEndpoints) > 0 {
		agents = append(agents, AgentProposal{
			Filename: "api-designer.md",
			Name:     "api-designer",
			Desc:     "Designs and documents API endpoints following REST/GraphQL conventions",
			Selected: true,
			Content:  agentAPIDesigner(projectName, arch, u.APIEndpoints),
		})
	}

	// ── Domain-specific ───────────────────────────────────────────────────────

	domainLower := strings.ToLower(domain + " " + u.Purpose)

	if containsAny(domainLower, "payment", "stripe", "checkout", "e-commerce", "esim", "billing") {
		agents = append(agents, AgentProposal{
			Filename: "payment-specialist.md",
			Name:     "payment-specialist",
			Desc:     "Handles payment flows, Stripe webhooks, and checkout integrity",
			Selected: true,
			Content:  agentPayment(projectName, u.ExternalServices),
		})
	}

	if containsAny(domainLower, "auth", "oauth", "jwt", "session", "login", "user account") {
		agents = append(agents, AgentProposal{
			Filename: "auth-specialist.md",
			Name:     "auth-specialist",
			Desc:     "Handles authentication, authorization, and session security",
			Selected: false,
			Content:  agentAuth(projectName, arch),
		})
	}

	if containsAny(domainLower, "saas", "tenant", "subscription", "multi-tenant") {
		agents = append(agents, AgentProposal{
			Filename: "tenant-guard.md",
			Name:     "tenant-guard",
			Desc:     "Enforces tenant isolation, checks data boundaries across all operations",
			Selected: false,
			Content:  agentTenantGuard(projectName),
		})
	}

	// ── Always last: documenter ───────────────────────────────────────────────

	agents = append(agents, AgentProposal{
		Filename: "documenter.md",
		Name:     "documenter",
		Desc:     "Keeps docs/ and inline comments up to date",
		Selected: false,
		Content:  agentDocumenter(projectName, arch),
	})

	return agents
}

func buildRuleProposals(u *llm.ProjectUnderstanding) []RuleProposal {
	rules := []RuleProposal{}

	// Rules from AI-detected conventions
	for _, c := range u.Conventions {
		if strings.TrimSpace(c) != "" {
			rules = append(rules, RuleProposal{
				Rule:     c,
				Source:   "ai-convention",
				Selected: true,
			})
		}
	}

	// Extract rules from tech notes (split by sentence)
	if u.TechNotes != "" {
		notes := strings.Split(u.TechNotes, ".")
		for _, note := range notes {
			note = strings.TrimSpace(note)
			if len(note) > 20 && len(note) < 200 {
				rules = append(rules, RuleProposal{
					Rule:     note,
					Source:   "tech-notes",
					Selected: true,
				})
			}
		}
	}

	// Extract from WhatClaudeKnows (split by sentence/period)
	if u.WhatClaudeKnows != "" {
		notes := strings.Split(u.WhatClaudeKnows, ".")
		for _, note := range notes {
			note = strings.TrimSpace(note)
			if len(note) > 20 && len(note) < 200 {
				rules = append(rules, RuleProposal{
					Rule:     note,
					Source:   "ai-context",
					Selected: true,
				})
			}
		}
	}

	// Deduplicate
	seen := map[string]bool{}
	unique := []RuleProposal{}
	for _, r := range rules {
		key := strings.ToLower(r.Rule[:min3(30, len(r.Rule))])
		if !seen[key] {
			seen[key] = true
			unique = append(unique, r)
		}
	}

	// Cap at 10 rules
	if len(unique) > 10 {
		unique = unique[:10]
	}

	return unique
}

func buildSkillProposals(fp *analyzer.ProjectFingerprint, u *llm.ProjectUnderstanding) []SkillProposal {
	skills := []SkillProposal{}
	seen := map[string]bool{}

	add := func(s SkillProposal) {
		if seen[s.Filename] {
			return
		}
		seen[s.Filename] = true
		skills = append(skills, s)
	}

	// ── Always included (core workflow) ─────────────────────────────────────

	add(SkillProposal{
		Filename: "systematic-debugging.md",
		Name:     "systematic-debugging",
		Desc:     "Hypothesis-driven debugging methodology",
		Reason:   "Essential for any project",
		Selected: true,
		Content:  skillSystematicDebugging(),
	})

	add(SkillProposal{
		Filename: "verification-before-completion.md",
		Name:     "verification-before-completion",
		Desc:     "Verify work before claiming done",
		Reason:   "Prevents false success claims",
		Selected: true,
		Content:  skillVerificationBeforeCompletion(),
	})

	add(SkillProposal{
		Filename: "test-driven-development.md",
		Name:     "test-driven-development",
		Desc:     "Red-green-refactor TDD workflow",
		Reason:   "Ensures quality",
		Selected: true,
		Content:  skillTDD(),
	})

	add(SkillProposal{
		Filename: "git-pushing.md",
		Name:     "git-pushing",
		Desc:     "Conventional commits and clean push workflow",
		Reason:   "Consistent commit history",
		Selected: true,
		Content:  skillGitPushing(),
	})

	add(SkillProposal{
		Filename: "requesting-code-review.md",
		Name:     "requesting-code-review",
		Desc:     "Before merging, verify work meets requirements",
		Reason:   "Quality gate before each PR",
		Selected: true,
		Content:  skillRequestingCodeReview(),
	})

	// ── Stack-specific ────────────────────────────────────────────────────────

	stackStr := strings.ToLower(fp.StackString())
	domainStr := strings.ToLower(u.Domain + " " + u.Purpose)

	if strings.Contains(stackStr, "next") || strings.Contains(stackStr, "react") {
		add(SkillProposal{
			Filename: "vercel-react-best-practices.md",
			Name:     "vercel-react-best-practices",
			Desc:     "React/Next.js performance patterns (Vercel Engineering)",
			Reason:   "Detected " + fp.StackString(),
			Selected: true,
			Content:  skillReactBestPractices(),
		})
		add(SkillProposal{
			Filename: "frontend-dev-guidelines.md",
			Name:     "frontend-dev-guidelines",
			Desc:     "Component patterns, Suspense, TanStack, MUI patterns",
			Reason:   "Next.js/React project",
			Selected: false,
			Content:  skillFrontendGuidelines(),
		})
	}

	if containsAny(stackStr, "nestjs", "express", "fastify", "laravel", "django", "rails", "go", "golang") {
		add(SkillProposal{
			Filename: "backend-dev-guidelines.md",
			Name:     "backend-dev-guidelines",
			Desc:     "Layered architecture, error handling, validation patterns",
			Reason:   "Backend project detected",
			Selected: true,
			Content:  skillBackendGuidelines(),
		})
	}

	if containsAny(domainStr, "payment", "stripe", "checkout", "esim", "e-commerce", "saas", "subscription") {
		add(SkillProposal{
			Filename: "broken-authentication.md",
			Name:     "broken-authentication",
			Desc:     "Auth & session security testing methodology",
			Reason:   "E-commerce/payments domain — auth is critical",
			Selected: false,
			Content:  skillBrokenAuth(),
		})
	}

	if containsAny(domainStr, "api", "rest", "graphql", "backend", "microservice") {
		add(SkillProposal{
			Filename: "api-fuzzing.md",
			Name:     "api-fuzzing",
			Desc:     "API security testing — IDOR, injection, auth bypass",
			Reason:   "API project — security is critical",
			Selected: false,
			Content:  skillAPIFuzzing(),
		})
	}

	// ── Fill to 10 with useful generic skills ─────────────────────────────────

	add(SkillProposal{
		Filename: "software-architecture.md",
		Name:     "software-architecture",
		Desc:     "Architecture quality focus for scalable maintainable systems",
		Reason:   "Good for any project",
		Selected: false,
		Content:  skillSoftwareArchitecture(),
	})

	add(SkillProposal{
		Filename: "writing-plans.md",
		Name:     "writing-plans",
		Desc:     "Plan before coding — task breakdown, dependencies, risks",
		Reason:   "Reduces implementation errors",
		Selected: false,
		Content:  skillWritingPlans(),
	})

	add(SkillProposal{
		Filename: "receiving-code-review.md",
		Name:     "receiving-code-review",
		Desc:     "Verify and critique received code review feedback",
		Reason:   "Team workflow",
		Selected: false,
		Content:  skillReceivingCodeReview(),
	})

	add(SkillProposal{
		Filename: "brainstorming.md",
		Name:     "brainstorming",
		Desc:     "Explore intent and requirements before implementation",
		Reason:   "Design before building",
		Selected: false,
		Content:  skillBrainstorming(),
	})

	// Cap at 10
	if len(skills) > 10 {
		skills = skills[:10]
	}
	return skills
}

// ─── UI ───────────────────────────────────────────────────────────────────────

func confirmAgents(p *Proposals) {
	if len(p.Agents) == 0 {
		return
	}
	items := make([]CheckItem, len(p.Agents))
	for i, a := range p.Agents {
		items[i] = CheckItem{Label: a.Name, Detail: a.Desc, Selected: a.Selected}
	}
	result := InteractiveCheckbox(
		"🤖 AGENTS — subagents in .claude/agents/",
		"Each selected agent becomes a specialised Claude Code subagent for this project.",
		items,
		false,
	)
	for i := range p.Agents {
		p.Agents[i].Selected = result[i].Selected
	}
}

func confirmRules(p *Proposals) {
	if len(p.Rules) == 0 {
		tui.Warn("No project-specific rules found in AI understanding (conventions empty)")
		return
	}
	items := make([]CheckItem, len(p.Rules))
	for i, rule := range p.Rules {
		src := "[" + rule.Source + "]"
		items[i] = CheckItem{Label: truncate(rule.Rule, 40), Detail: src, Selected: rule.Selected}
	}
	result := InteractiveCheckbox(
		"📐 PROJECT RULES — .claude/rules/05-project-specific.md",
		"AI extracted these rules from your source code. Confirm which to apply.",
		items,
		false,
	)
	for i := range p.Rules {
		p.Rules[i].Selected = result[i].Selected
	}
}

func confirmSkills(p *Proposals) {
	if len(p.Skills) == 0 {
		return
	}
	items := make([]CheckItem, len(p.Skills))
	for i, s := range p.Skills {
		items[i] = CheckItem{Label: s.Name, Detail: s.Reason, Selected: s.Selected}
	}
	result := InteractiveCheckbox(
		"🔧 SKILLS — .claude/skills/",
		"Teach Claude specific methodologies for this project. Press [/] to search for more skills.",
		items,
		true, // searchable
	)
	for i := range p.Skills {
		p.Skills[i].Selected = result[i].Selected
	}
}

// ─── File writer ──────────────────────────────────────────────────────────────

func writeProposals(destDir string, p *Proposals, u *llm.ProjectUnderstanding) {
	agentsDir := filepath.Join(destDir, ".claude", "agents")
	skillsDir := filepath.Join(destDir, ".claude", "skills")
	rulesDir := filepath.Join(destDir, ".claude", "rules")

	_ = os.MkdirAll(agentsDir, 0755)
	_ = os.MkdirAll(skillsDir, 0755)

	// Write agents
	for _, a := range p.Agents {
		if !a.Selected {
			continue
		}
		path := filepath.Join(agentsDir, a.Filename)
		_ = os.WriteFile(path, []byte(a.Content), 0644)
		tui.Success(fmt.Sprintf("  Agent created: .claude/agents/%s", a.Filename))
	}

	// Write project-specific rules
	selectedRules := []string{}
	for _, r := range p.Rules {
		if r.Selected {
			selectedRules = append(selectedRules, r.Rule)
		}
	}
	if len(selectedRules) > 0 {
		content := generateProjectSpecificRulesFile(u.ProjectName, selectedRules)
		path := filepath.Join(rulesDir, "05-project-specific.md")
		_ = os.WriteFile(path, []byte(content), 0644)
		tui.Success(fmt.Sprintf("  Rules created: .claude/rules/05-project-specific.md (%d rules)", len(selectedRules)))
	}

	// Write skills
	for _, s := range p.Skills {
		if !s.Selected {
			continue
		}
		path := filepath.Join(skillsDir, s.Filename)
		_ = os.WriteFile(path, []byte(s.Content), 0644)
		tui.Success(fmt.Sprintf("  Skill created: .claude/skills/%s", s.Filename))
	}
}

func generateProjectSpecificRulesFile(projectName string, rules []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Project-Specific Rules — %s\n\n", projectName))
	sb.WriteString("> Auto-generated by ccbootstrap based on AI analysis of the source code.\n")
	sb.WriteString("> Update this file as the project evolves.\n\n")
	sb.WriteString("## Rules\n\n")
	for _, r := range rules {
		sb.WriteString("- " + r + "\n")
	}
	sb.WriteString("\n## Enforcement\n")
	sb.WriteString("Claude must follow all rules above in addition to the generic rules in 01-04-*.md files.\n")
	sb.WriteString("If a rule conflicts with a user request, Claude must mention the conflict and ask for confirmation.\n")
	return sb.String()
}

func printSectionHeader(title string) {
	line := strings.Repeat("─", 68)
	fmt.Printf("\n%s\n  %s\n%s\n", line, tui.Bold(title), line)
}

// ─── Agent content templates ──────────────────────────────────────────────────

func agentCodeReviewer(project, arch, domain, conventions string) string {
	conv := ""
	if conventions != "" {
		conv = "\n## Project Conventions to Enforce\n- " + conventions + "\n"
	}
	return fmt.Sprintf(`---
name: code-reviewer
description: Use when reviewing code changes, analyzing PRs, auditing files for quality and security. Invoke with "review this", "check this code", "audit this PR".
tools: Read, Bash
---

# Code Reviewer — %s

You are a senior code reviewer specializing in %s (%s).

## Review Protocol

For every review, check ALL of these:

### 1. Correctness
- Does the code do exactly what it claims?
- Are error cases handled gracefully?
- Are edge cases covered?
- Is async/await used correctly (no floating promises)?

### 2. Security
- No hardcoded API keys, passwords, or tokens
- All user inputs validated and sanitized
- SQL queries use parameterized statements
- No sensitive data logged or exposed in responses
- Auth checks present on protected routes
%s
### 3. Code Quality
- Functions under 50 lines; split if larger
- Maximum 3 levels of nesting; refactor if deeper
- No commented-out code
- No unused variables, imports, or dead code
- Naming is clear and intention-revealing

### 4. Tests
- New features have corresponding tests
- Tests cover happy path + at least 2 edge cases
- Tests use realistic data, not empty stubs

## Output Format

Always respond with a structured report:

**✅ Good** — what is well-done
**⚠️ Concerns** — non-blocking issues worth noting
**❌ Must Fix** — blocking issues that need resolution before merge

Be specific: cite file names and line numbers.
`, project, arch, domain, conv)
}

func agentDebugger(project, arch, techNotes string, fp *analyzer.ProjectFingerprint) string {
	testCmd := ""
	for _, s := range fp.Stack {
		switch {
		case strings.Contains(s, "Laravel"):
			testCmd = "php artisan test --parallel"
		case strings.Contains(s, "Next"), strings.Contains(s, "NestJS"):
			testCmd = "npm test"
		case strings.Contains(s, "Django"), strings.Contains(s, "FastAPI"):
			testCmd = "pytest"
		case s == "Go" || strings.HasPrefix(s, "Go/"):
			testCmd = "go test ./..."
		}
	}
	notes := ""
	if techNotes != "" {
		notes = "\n## Known Gotchas\n" + techNotes + "\n"
	}
	testSection := ""
	if testCmd != "" {
		testSection = fmt.Sprintf("\n## Verify Fix\nRun the test suite: `%s`\nConfirm the failing test now passes and no regressions introduced.\n", testCmd)
	}
	return fmt.Sprintf(`---
name: debugger
description: Systematic bug investigation. Invoke when encountering errors, unexpected behavior, failing tests, or performance issues.
tools: Read, Bash, Grep
---

# Systematic Debugger — %s

You are a debugging specialist for %s (%s).

## Debugging Protocol

**NEVER guess or apply random fixes.** Follow this sequence:

### Step 1 — Understand the symptom
- What is the exact error message or unexpected behavior?
- When does it occur? (always, sometimes, specific input?)
- What was the last change before this started?

### Step 2 — Define scope
- Which file/module/function is the entry point?
- Trace the data flow from input to error
- Check logs, stack traces, and network responses

### Step 3 — Form 3 hypotheses (most to least likely)
State each hypothesis clearly before testing any.

### Step 4 — Test hypotheses with minimal reproductions
- Add targeted logging or breakpoints
- Isolate the component causing the issue
- Do NOT modify production code during investigation

### Step 5 — Fix root cause (not symptom)
- Explain why this is the root cause, not a workaround
- The fix should be minimal and targeted
%s%s
## Output Format
Report: symptom → root cause → fix → verification.
`, project, arch, project, notes, testSection)
}

func agentTestWriter(project, arch string, fp *analyzer.ProjectFingerprint) string {
	fws := fp.TestFrameworksString()
	return fmt.Sprintf(`---
name: test-writer
description: Writes comprehensive tests. Invoke with "write tests for", "add test coverage", or "test this function".
tools: Read, Write, Bash
---

# Test Writer — %s

You are a testing specialist for %s (%s).

## Testing Approach

Always follow TDD:
1. Write the test **first** (it should fail)
2. Write minimal implementation to make it pass
3. Refactor with tests green

## Test Structure

Every test must cover:
- **Happy path** — normal successful case
- **Edge cases** — empty input, boundary values, max limits
- **Error cases** — invalid input, service failures, auth failures

## Test Quality Rules
- Tests must be deterministic (no random data without seed)
- Tests must be independent (no shared state between tests)
- Use realistic test data (not empty strings or 0s)
- Test behavior, not implementation details
- Avoid mocking too much — prefer integration tests for critical paths

## Output
- Write tests in the appropriate test file
- Run: %s
- All new tests must pass before completing
`, project, arch, fws, fp.TestFrameworksString())
}

func agentFrontend(project, arch string, fp *analyzer.ProjectFingerprint) string {
	return fmt.Sprintf(`---
name: frontend-specialist
description: Frontend expert. Invoke for React/Next.js components, layout issues, performance, UI bugs, or styling.
tools: Read, Write
---

# Frontend Specialist — %s

You are a senior frontend engineer for %s (%s).

## Core Principles

### Performance
- Prefer Server Components for data fetching (no useEffect for initial data)
- Use React.lazy() and Suspense for non-critical components
- Avoid unnecessary re-renders: use useMemo/useCallback purposefully
- Images: always use next/image with explicit width/height

### TypeScript
- Strict mode only — never use "any"
- Define interfaces for all component props
- Use discriminated unions for complex state

### Component Design
- Components do one thing; extract if >100 lines
- Colocate styles, tests, and types with their component
- Props should be minimal — components should be focused

### CSS / Styling
- No inline styles unless absolutely dynamic
- Consistent spacing using design tokens
- Mobile-first responsive design

## Output
When modifying components:
1. Check for existing patterns in the codebase first
2. Match the existing style conventions
3. Verify responsive behavior
4. Check TypeScript errors: run "npm run type-check" or "tsc --noEmit"
`, project, arch, fp.StackString())
}

func agentAPIDesigner(project, arch string, endpoints []string) string {
	sample := ""
	if len(endpoints) > 0 {
		max := 5
		if len(endpoints) < max {
			max = len(endpoints)
		}
		sample = "\n## Existing Endpoint Patterns\n"
		for _, e := range endpoints[:max] {
			sample += "- " + e + "\n"
		}
	}
	return fmt.Sprintf(`---
name: api-designer
description: Designs and documents API endpoints. Invoke when adding routes, designing API contracts, or reviewing endpoint structure.
tools: Read, Write
---

# API Designer — %s

You are a senior API architect for %s (%s).

## REST Design Rules

### Naming
- Resources in plural nouns: /users, /orders - not /getUsers
- Use HTTP verbs correctly: GET (read), POST (create), PUT/PATCH (update), DELETE
- Nested resources only when truly hierarchical: /users/:id/orders
- Filter with query params: GET /products?category=esim&country=fr

### Responses
- Always return consistent shape: { data, meta, error }
- Use appropriate status codes: 200, 201, 400, 401, 403, 404, 409, 422, 500
- Never return 200 for errors
- Include pagination metadata for lists

### Security
- Validate all route parameters
- Sanitize all query strings
- Rate limit public endpoints
- Auth middleware on all protected routes
- Never return sensitive fields (passwords, keys, tokens)

### Error Format
Return JSON: { "error": { "code": "ERR_CODE", "message": "...", "details": [...] } }
%s
## Output
When designing APIs:
1. Define the endpoint contract first (method, path, request body, response)
2. List all validation rules
3. Document error cases
4. Check for consistency with existing routes
`, project, arch, arch, sample)
}

func agentPayment(project string, services []string) string {
	serviceStr := strings.Join(services, ", ")
	return fmt.Sprintf(`---
name: payment-specialist
description: Handles payment flows, checkout integrity, and financial data. Invoke for any task touching payments, orders, subscriptions, or billing.
tools: Read, Write
---

# Payment Specialist — %s

You are a payment and checkout expert for %s (Services: %s).

## Payment Safety Rules — NEVER SKIP

1. **Webhook verification**: Always verify webhook signatures before processing events
2. **Idempotency**: All payment operations must be idempotent (use idempotency keys)
3. **No client-side trust**: Never trust client-side payment success status — always verify server-side
4. **Atomic operations**: Payment + order creation must be atomic (use database transactions)
5. **Audit trail**: Log all payment state transitions with timestamps
6. **Refund logic**: Never auto-refund without explicit admin approval or verified failure
7. **Currency handling**: Always store amounts in smallest currency unit (cents), never floats

## Checkout Flow Checklist
- [ ] User is authenticated before payment attempt
- [ ] Cart/order is validated server-side (not trusted from client)
- [ ] Payment intent created server-side
- [ ] Webhook handler verifies signature
- [ ] Order status updated only after webhook confirmation
- [ ] Confirmation email sent after successful webhook

## Output
When touching payment code, explicitly confirm each safety rule is met.
`, project, project, serviceStr)
}

func agentAuth(project, arch string) string {
	return fmt.Sprintf(`---
name: auth-specialist
description: Handles authentication, authorization, and session security. Invoke for login flows, JWT handling, role checks, or session management.
tools: Read, Write
---

# Auth Specialist — %s

You are an authentication and authorization specialist for %s (%s).

## Auth Security Rules

### Authentication
- Passwords: bcrypt with cost >= 12
- Tokens: JWT with short expiry (15min access, 7d refresh)
- Store refresh tokens in httpOnly cookies (not localStorage)
- Rate limit login attempts (max 5/min per IP)

### Authorization
- Check permissions at the service layer, not just middleware
- Use role-based access control (RBAC) consistently
- Never trust client-sent user IDs — always derive from auth token
- Log all authorization failures

### Sessions
- Invalidate tokens on logout (maintain a denylist or rotate refresh tokens)
- Session fixation prevention: rotate session ID after login
- Concurrent session limit for sensitive accounts

## Output
When touching auth code:
1. Identify what is protected and by what mechanism
2. Check for missing auth on route/function
3. Verify token validation logic
4. Check for over-permissive role checks
`, project, project, arch)
}

func agentTenantGuard(project string) string {
	return fmt.Sprintf(`---
name: tenant-guard
description: Enforces multi-tenant data isolation. Invoke when working with any database query, background job, or data access layer.
tools: Read
---

# Tenant Guard — %s

You are a multi-tenancy data isolation specialist for %s.

## Isolation Rules — ZERO TOLERANCE

1. Every query must filter by tenant_id — no exceptions
2. Background jobs must carry tenant context from the triggering request
3. Never use findAll() or equivalent without a tenant scope
4. Cross-tenant references must be validated explicitly
5. Tenant context must be verified at the service layer, not just middleware

## Review Checklist — Run on Every Database Access

For each query/ORM call:
- [ ] Has WHERE tenant_id = ? or equivalent scope
- [ ] tenant_id comes from authenticated session, not request body
- [ ] Joins do not expose data from other tenants
- [ ] Background jobs re-validate tenant context

## Red Flags
- Any query without tenant filter
- req.body.tenantId used for scoping (must come from token)
- Aggregate queries without tenant isolation
- Shared cache keys without tenant prefix

Output: always report tenant isolation status for every data access reviewed.
`, project, project)
}

func agentDocumenter(project, arch string) string {
	return fmt.Sprintf(`---
name: documenter
description: Updates documentation, writes changelogs, and maintains inline comments. Invoke with "document this", "update the docs", or "write a changelog".
tools: Read, Write
---

# Documenter — %s

You are a technical documentation specialist for %s (%s).

## Documentation Standards

### Code Comments
- Comments explain WHY, not WHAT (the code explains what)
- Complex algorithms get a brief explanation before them
- Public APIs always have JSDoc/GoDoc/PHPDoc comments

### docs/ Structure
- docs/architecture.md — system design, data flow, tech decisions
- docs/decisions/ — Architecture Decision Records (ADR format)
- docs/solutions/ — solved problems with context for future reference
- docs/progress.md — session progress (update at end of each session)

### Changelog Format
Follow Keep A Changelog (https://keepachangelog.com):
- [Added] for new features
- [Changed] for changes to existing functionality
- [Fixed] for bug fixes
- [Deprecated] for soon-to-be removed features
- [Removed] for now removed features
- [Security] for vulnerabilities

## Output
When writing documentation:
1. Check what already exists before overwriting
2. Match existing style and terminology
3. Update the table of contents if one exists
4. Cross-link related documents
`, project, project, arch)
}

// ─── Skill content templates ──────────────────────────────────────────────────

func skillSystematicDebugging() string {
	return `# Skill: Systematic Debugging

## When to use
Use this skill when encountering any bug, test failure, or unexpected behavior.

## Protocol
1. **Define the symptom** — exact error, when it occurs, what changed
2. **Form 3 hypotheses** — ordered by likelihood
3. **Test each hypothesis** — minimal reproduction
4. **Fix root cause** — not symptom
5. **Verify** — run tests, check edge cases

## Rules
- NEVER apply a fix without understanding the root cause
- NEVER blame the framework/library first
- Log your hypotheses before testing them
- The simplest explanation is usually right (Occam's razor)
`
}

func skillVerificationBeforeCompletion() string {
	return `# Skill: Verification Before Completion

## When to use
Before claiming work is done, fixed, or passing. Before committing or creating PRs.

## Verification Checklist
1. Run the test suite — confirm it passes
2. Run the type checker — no TypeScript/mypy errors
3. Run the linter — no lint errors
4. Test the specific functionality manually if applicable
5. Check edge cases you might have missed
6. Review the diff — no debug logs, no commented-out code, no TODOs left

## Rule
Do NOT claim "done" or "fixed" without evidence. Show the passing test output.
`
}

func skillTDD() string {
	return `# Skill: Test-Driven Development

## When to use
When implementing any new feature or fixing a bug.

## Red-Green-Refactor Cycle
1. **RED** — Write a failing test that describes the desired behavior
2. **GREEN** — Write the minimal code to make the test pass
3. **REFACTOR** — Clean up the code while keeping tests green

## Rules
- Write the test BEFORE the implementation
- Each test must have ONE reason to fail
- Tests must be self-describing (test name = requirement)
- Never modify tests to make them pass — fix the implementation
- Test public API, not internal implementation
`
}

func skillGitPushing() string {
	return `# Skill: Git Pushing

## When to use
When ready to commit and push changes.

## Conventional Commits Format
` + "`" + `<type>(<scope>): <short description>` + "`" + `

Types: feat, fix, chore, docs, refactor, test, perf, style, ci

## Checklist Before Push
- [ ] Tests pass
- [ ] No debug logs or commented code
- [ ] Commit message is descriptive and follows convention
- [ ] Not pushing to main/master directly
- [ ] Branch name is descriptive (feat/, fix/, chore/)

## Command sequence
` + "```bash\ngit add -p        # Stage selectively\ngit commit -m \"feat(scope): description\"\ngit push origin HEAD\n```" + `
`
}

func skillRequestingCodeReview() string {
	return `# Skill: Requesting Code Review

## When to use
When completing tasks, implementing major features, or before merging.

## Steps
1. Run full test suite — confirm green
2. Review your own diff: git diff main
3. Check for: hardcoded values, missing error handling, security issues
4. Write a clear PR description: what changed and why
5. Link related issues or decisions

## PR Description Template
` + "```\n## What\nBrief description of changes.\n\n## Why\nContext and motivation.\n\n## How\nKey implementation decisions.\n\n## Testing\nHow this was tested.\n```"
}

func skillReactBestPractices() string {
	return `# Skill: Vercel React Best Practices

## Data Fetching (Next.js App Router)
- Prefer Server Components for initial data load — NO useEffect for data
- Use fetch() with cache and revalidate options
- Parallel data fetching with Promise.all()
- Use loading.tsx and error.tsx for Suspense boundaries

## Performance
- React.lazy + Suspense for non-critical components
- useMemo only when computation is expensive (profile first)
- useCallback only when passed to memoized child components
- next/image for all images — not plain img tags
- next/font for all fonts — eliminates layout shift

## Common Mistakes to Avoid
- useEffect for data fetching (use Server Components instead)
- Missing keys in lists
- Prop drilling more than 2 levels (use context or state manager)
- Large bundles from client components (move to server when possible)
`
}

func skillFrontendGuidelines() string {
	return `# Skill: Frontend Development Guidelines

## Component Architecture
- Feature-based folder structure: src/features/<feature>/
- Colocate: component + styles + tests + types
- Prefer composition over inheritance
- Smart/Dumb component pattern: container handles logic, presentational handles UI

## TypeScript
- Strict mode; no "any" type
- Use interface for object shapes, type for unions/intersections
- Explicit return types on public functions

## State Management
- Server state: React Query / useSuspenseQuery
- Client state: useState, zustand for global
- Form state: react-hook-form

## Testing
- Unit: component rendering with @testing-library/react
- Integration: full user flows with MSW for API mocking
`
}

func skillBackendGuidelines() string {
	return `# Skill: Backend Development Guidelines

## Layered Architecture
Request -> Route -> Controller -> Service -> Repository -> Database

- Controller: parse/validate request, call service, return response
- Service: business logic, no HTTP concepts, no direct DB calls
- Repository: data access only, returns domain types

## Error Handling
- Never swallow errors silently
- Use typed error classes (ValidationError, NotFoundError, etc.)
- HTTP status codes must match the error type
- Log errors with context (user_id, request_id, stack trace)

## Validation
- Validate ALL inputs at the controller layer
- Use schema validation (Zod, Joi, class-validator)
- Never trust client-sent IDs — validate ownership server-side

## Database
- Use transactions for multi-step operations
- Parameterized queries only — no string concatenation
- Index foreign keys and frequently filtered columns
- Limit N+1 queries: use eager loading / joins
`
}

func skillBrokenAuth() string {
	return `# Skill: Broken Authentication Testing

## When to use
When reviewing authentication and session management code.

## Checklist
- [ ] Passwords hashed with bcrypt/argon2 (not MD5/SHA1)
- [ ] JWT tokens have short expiry (access: 15min, refresh: 7d)
- [ ] Refresh tokens stored in httpOnly cookies
- [ ] Login rate limiting (max 5 attempts/min/IP)
- [ ] Account lockout after repeated failures
- [ ] Password reset tokens are single-use and time-limited
- [ ] Session invalidated on logout
- [ ] Concurrent session detection for sensitive accounts
`
}

func skillAPIFuzzing() string {
	return `# Skill: API Security Testing

## When to use  
When reviewing API endpoints for security vulnerabilities.

## Quick IDOR Check
For any endpoint returning user data:
1. Note the object ID in the response
2. Change the ID to another user's ID
3. Verify the request is rejected with 403 (not 200 or 404)

## Input Validation Check
Test each input field with:
- Empty string: ""
- Null: null
- Very long string: 10,000 chars
- SQL injection: ' OR 1=1--
- XSS: <script>alert(1)</script>
- Path traversal: ../../etc/passwd

## Auth Check
- Remove Authorization header → expect 401
- Use another user's token → expect 403
- Use expired token → expect 401
`
}

func skillSoftwareArchitecture() string {
	return `# Skill: Software Architecture

## When to use
When designing new features, refactoring, or making significant structural decisions.

## Principles
- **SRP**: each module has one reason to change
- **OCP**: open for extension, closed for modification
- **LSP**: substitutable implementations
- **ISP**: clients depend only on what they use
- **DIP**: depend on abstractions, not concretions

## Before Implementing
1. Draw the data flow (even on paper)
2. Identify the boundaries (what changes independently)
3. Ask: what is the simplest design that works?
4. Consider: how will this be tested?

## Decision Record
For significant decisions, create docs/decisions/YYYY-MM-DD-title.md:
- Context: why this decision is needed
- Decision: what was chosen
- Rationale: why this option over alternatives
- Consequences: trade-offs accepted
`
}

func skillWritingPlans() string {
	return `# Skill: Writing Implementation Plans

## When to use
Before starting any complex task (>30 min of work).

## Plan Format
` + "```markdown\n## Goal\n{one sentence describing the outcome}\n\n## Steps\n- [ ] Step 1: ...\n- [ ] Step 2: ...\n\n## Dependencies\n- What must exist before step X\n\n## Risks\n- What could go wrong\n- How to mitigate\n\n## Verification\n- How to confirm the plan worked\n```" + `

## Rules
- Write the plan BEFORE touching code
- Each step is atomic and completable in <1 hour
- Get confirmation on the plan before implementing
`
}

func skillReceivingCodeReview() string {
	return `# Skill: Receiving Code Review

## When to use
When receiving feedback on a PR or code review.

## Protocol
1. **Read all comments first** before responding to any
2. **Understand the concern** — ask clarifying questions if unclear
3. **Evaluate technically** — is the feedback correct? Check the code
4. **Don't implement blindly** — if feedback seems wrong, explain why
5. **Address root causes** — don't just satisfy the reviewer mechanically

## Rules
- Never implement suggestions you disagree with without discussing
- If reviewer is wrong, explain technically why with evidence
- Distinguish: required changes vs suggestions vs questions
- Performance claims must be benchmarked
`
}

func skillBrainstorming() string {
	return `# Skill: Brainstorming Before Implementation

## When to use
Before any creative work — new feature, component, significant change.

## Steps
1. **Understand intent** — what is the user actually trying to achieve?
2. **Explore requirements** — what are the constraints? What must it do/not do?
3. **Generate options** — at least 3 different approaches
4. **Evaluate options** — complexity, maintainability, fit with existing code
5. **Select and justify** — pick one and explain why

## Questions to Ask
- What is the simplest version that solves the problem?
- How will this be tested?  
- Does this match existing patterns in the codebase?
- What can go wrong?
`
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func min3(a, b int) int {
	if a < b {
		return a
	}
	return b
}
