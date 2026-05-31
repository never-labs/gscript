package llm

// ToolSummary is the runtime-independent shape used by AI tool capability
// checks. Runtime adapters populate it from script-visible tool tables.
type ToolSummary struct {
	Name     string
	Requires []string
}

// MissingCapability describes the first capability required by a tool that is
// not covered by the ambient allowlist.
type MissingCapability struct {
	Tool       string
	Capability string
}

// ToolCapabilities returns the unique, non-empty capability requirements
// declared by tools, preserving first-seen order for user-facing output.
func ToolCapabilities(tools []ToolSummary) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, tool := range tools {
		for _, cap := range tool.Requires {
			if cap == "" || seen[cap] {
				continue
			}
			seen[cap] = true
			out = append(out, cap)
		}
	}
	return out
}

// CheckToolCapabilities applies the public llm.check_tools allowlist rules.
// It returns nil when every tool requirement is permitted.
func CheckToolCapabilities(tools []ToolSummary, allowedCaps []string) *MissingCapability {
	allowed := map[string]bool{}
	for _, cap := range allowedCaps {
		allowed[cap] = true
	}
	if allowed["all"] || allowed["cap.all"] || allowed["*"] {
		return nil
	}
	if allowed["none"] || allowed["cap.none"] {
		allowed = map[string]bool{}
	}
	for _, tool := range tools {
		for _, cap := range tool.Requires {
			if cap == "" || cap == "none" || cap == "cap.none" || allowed[cap] {
				continue
			}
			return &MissingCapability{Tool: tool.Name, Capability: cap}
		}
	}
	return nil
}
