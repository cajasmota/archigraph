package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cajasmota/archigraph/internal/graph"
)

// nodeWithRepo carries an entity together with the repo it lives in. Edges
// reference nodes by their prefixed id (<repo>::<localId>).
type nodeWithRepo struct {
	Repo   string
	Entity *graph.Entity
	Score  float64
}

// renderResult is the structured input to the compact renderer.
type renderResult struct {
	Header         string
	MatchedTotal   int
	Nodes          []nodeWithRepo
	Edges          []renderEdge
	HiddenImpCalls int
	OneRepo        bool
	OneCommunity   *int
	TruncatedNote  string
}

// renderEdge is a directed edge entry in the compact output.
type renderEdge struct {
	From string // label
	To   string // label
	Kind string
}

// prefixedID produces "<repo>::<localId>" per ADR-0009.
func prefixedID(repo, id string) string { return repo + "::" + id }

// splitPrefixed splits "<repo>::<localId>"; returns ("",id) if no prefix.
func splitPrefixed(s string) (string, string) {
	i := strings.Index(s, "::")
	if i < 0 {
		return "", s
	}
	return s[:i], s[i+2:]
}

// estimateTokens approximates token count as len/4 per the brief.
func estimateTokens(s string) int { return len(s) / 4 }

// renderCompact serializes a renderResult to the compact text format and
// enforces tokenBudget. Implicit "calls" edges between visible nodes are
// suppressed; SCOPE. prefix is stripped on user-facing kinds.
func renderCompact(r renderResult, tokenBudget int) string {
	if len(r.Nodes) == 0 {
		return "# no matches\n"
	}
	// Sort nodes by score desc.
	sort.SliceStable(r.Nodes, func(i, j int) bool { return r.Nodes[i].Score > r.Nodes[j].Score })

	var b strings.Builder
	headerLine := fmt.Sprintf("# nodes (%d matched", r.MatchedTotal)
	if r.OneCommunity != nil {
		headerLine += fmt.Sprintf(", community: %d", *r.OneCommunity)
	}
	headerLine += ")"
	b.WriteString(headerLine + "\n")

	visible := map[string]string{} // prefixedID -> label
	shown := 0
	for i := range r.Nodes {
		nw := r.Nodes[i]
		label := nw.Entity.Name
		loc := fmt.Sprintf("%s:%d", nw.Entity.SourceFile, nw.Entity.StartLine)
		var line string
		if r.OneRepo {
			line = fmt.Sprintf("%s  %s", label, loc)
		} else {
			line = fmt.Sprintf("[%s] %s  %s", nw.Repo, label, loc)
		}
		// Token-budget enforcement: stop adding nodes if the running budget
		// (current rendered text) exceeds the limit.
		if tokenBudget > 0 && estimateTokens(b.String()+line+"\n") > tokenBudget {
			break
		}
		b.WriteString(line + "\n")
		visible[prefixedID(nw.Repo, nw.Entity.ID)] = label
		shown++
	}
	if shown < len(r.Nodes) {
		b.WriteString(fmt.Sprintf("# truncated: %d nodes hidden by token budget\n", len(r.Nodes)-shown))
	}

	// Edges: drop implicit calls between visible nodes, strip SCOPE prefix.
	hidden := r.HiddenImpCalls
	visibleEdges := []renderEdge{}
	for _, e := range r.Edges {
		k := stripScopePrefix(e.Kind)
		if strings.EqualFold(k, "calls") || strings.EqualFold(k, "CALLS") {
			// implicit call between two visible nodes -> hide
			hidden++
			continue
		}
		visibleEdges = append(visibleEdges, renderEdge{From: e.From, To: e.To, Kind: k})
	}
	if len(visibleEdges) > 0 || hidden > 0 {
		b.WriteString(fmt.Sprintf("\n# edges (suppressed: %d implicit calls; shown: %d)\n", hidden, len(visibleEdges)))
		for _, e := range visibleEdges {
			line := fmt.Sprintf("%s → %s  [%s]\n", e.From, e.To, e.Kind)
			if tokenBudget > 0 && estimateTokens(b.String()+line) > tokenBudget {
				b.WriteString("# edges truncated by token budget\n")
				break
			}
			b.WriteString(line)
		}
	}
	if r.TruncatedNote != "" {
		b.WriteString("# " + r.TruncatedNote + "\n")
	}
	return b.String()
}
