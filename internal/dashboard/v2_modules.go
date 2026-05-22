// v2_modules.go — v2 endpoint for the module-level GDS analysis (#1384,
// part of epic #1380).
//
// Surfaces SCC / PageRank / betweenness over the aggregated module graph so
// the dashboard can render a collapsed-view "modules at a glance" UI (the UI
// work itself lives under #1386).
//
// Route registered in server.go:
//
//	GET /api/v2/groups/{group}/modules/analysis
//	    ?repo_filter=slug1,slug2
//	    &top_n=10            (default 10)
//	    &min_scc_size=2      (default 2)
//
// Response (v2 envelope):
//
//	{
//	  "ok": true,
//	  "data": {
//	    "repos": [
//	      {
//	        "repo": "...",
//	        "num_modules": N,
//	        "num_module_edges": N,
//	        "num_sccs": N,
//	        "largest_scc_size": N,
//	        "modules_in_cycle": N,
//	        "top_pagerank": [...],
//	        "top_betweenness": [...],
//	        "sccs": [...]
//	      }, ...
//	    ],
//	    "count": N
//	  }
//	}
package dashboard

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/cajasmota/archigraph/internal/graph"
)

// handleV2ModulesAnalysis — GET /api/v2/groups/{group}/modules/analysis
func (s *Server) handleV2ModulesAnalysis(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	if group == "" {
		writeV2Err(w, http.StatusBadRequest, "bad_request", "group required")
		return
	}
	grp, err := s.graphs.GetGroup(group)
	if err != nil {
		writeV2Err(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	q := r.URL.Query()
	topN := 10
	if v := q.Get("top_n"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topN = n
		}
	}
	minSCC := 2
	if v := q.Get("min_scc_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 2 {
			minSCC = n
		}
	}
	var repoFilter map[string]bool
	if v := q.Get("repo_filter"); v != "" {
		repoFilter = map[string]bool{}
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				repoFilter[s] = true
			}
		}
	}

	type centOut struct {
		ModuleID    string  `json:"module_id"`
		ModuleName  string  `json:"module_name"`
		PageRank    float64 `json:"pagerank"`
		Betweenness float64 `json:"betweenness"`
		InDegree    int     `json:"in_degree"`
		OutDegree   int     `json:"out_degree"`
		InCycle     bool    `json:"in_cycle"`
	}
	type sccOut struct {
		ID          int                `json:"id"`
		Size        int                `json:"size"`
		Members     []string           `json:"members"`
		MemberNames []string           `json:"member_names"`
		Edges       []graph.ModuleEdge `json:"edges"`
	}
	type repoOut struct {
		Repo           string    `json:"repo"`
		NumModules     int       `json:"num_modules"`
		NumModuleEdges int       `json:"num_module_edges"`
		NumSCCs        int       `json:"num_sccs"`
		LargestSCCSize int       `json:"largest_scc_size"`
		ModulesInCycle int       `json:"modules_in_cycle"`
		TopPageRank    []centOut `json:"top_pagerank"`
		TopBetweenness []centOut `json:"top_betweenness"`
		SCCs           []sccOut  `json:"sccs"`
	}

	out := make([]repoOut, 0)
	for _, repo := range sortedRepos(grp) {
		if repo.Doc == nil {
			continue
		}
		if repoFilter != nil && !repoFilter[repo.Slug] {
			continue
		}
		res := graph.RunModuleAlgorithms(repo.Doc.Entities, repo.Doc.Relationships)
		ro := repoOut{
			Repo:           repo.Slug,
			NumModules:     res.Stats.NumModules,
			NumModuleEdges: res.Stats.NumModuleEdges,
			NumSCCs:        res.Stats.NumSCCs,
			LargestSCCSize: res.Stats.LargestSCCSize,
			ModulesInCycle: res.Stats.NumModulesInCycle,
		}
		toCent := func(c graph.ModuleCentrality) centOut {
			return centOut{
				ModuleID:    dashPrefixedID(repo.Slug, c.ModuleID),
				ModuleName:  c.ModuleName,
				PageRank:    c.PageRank,
				Betweenness: c.Betweenness,
				InDegree:    c.InDegree,
				OutDegree:   c.OutDegree,
				InCycle:     res.SCCOf[c.ModuleID] >= 0,
			}
		}
		for _, c := range graph.TopByPageRank(res.Centrality, topN) {
			ro.TopPageRank = append(ro.TopPageRank, toCent(c))
		}
		for _, c := range graph.TopByBetweenness(res.Centrality, topN) {
			ro.TopBetweenness = append(ro.TopBetweenness, toCent(c))
		}
		for _, c := range res.SCCs {
			if c.Size < minSCC {
				continue
			}
			prefixedMembers := make([]string, len(c.Members))
			for i, m := range c.Members {
				prefixedMembers[i] = dashPrefixedID(repo.Slug, m)
			}
			prefixedEdges := make([]graph.ModuleEdge, len(c.Edges))
			for i, e := range c.Edges {
				prefixedEdges[i] = graph.ModuleEdge{
					FromModule: dashPrefixedID(repo.Slug, e.FromModule),
					ToModule:   dashPrefixedID(repo.Slug, e.ToModule),
					Weight:     e.Weight,
				}
			}
			ro.SCCs = append(ro.SCCs, sccOut{
				ID:          c.ID,
				Size:        c.Size,
				Members:     prefixedMembers,
				MemberNames: append([]string{}, c.MemberNames...),
				Edges:       prefixedEdges,
			})
		}
		out = append(out, ro)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Repo < out[j].Repo })

	writeV2JSON(w, http.StatusOK, v2OK(map[string]any{
		"repos": out,
		"count": len(out),
	}))
}
