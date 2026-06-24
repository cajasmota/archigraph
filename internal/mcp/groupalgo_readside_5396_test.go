package mcp

// groupalgo_readside_5396_test.go — read-side serving of the group-algo overlay
// (#5396, #5397).
//
// The group-algo overlay (~/.grafel/groups/<group>-algo.json) is computed by a
// single Louvain pass across ALL repos in the group, then applied to the live
// graph by applyGroupAlgoOverlay (#5354): it stamps each entity's CommunityID /
// PageRank / Centrality and populates LoadedGroup.Communities with the group
// community summary.
//
// Before #5396 the read-side consumers ignored that overlay:
//   - handleListCommunities looped repos and read per-repo r.Doc.Communities, so
//     a cross-repo community was never visible and a repo whose per-repo Louvain
//     never persisted (acme-mobile, #5397) silently produced zero communities.
//   - grafel_inspect only surfaced pagerank/community_id under verbose and never
//     centrality, and ignored include=community,pagerank,centrality.
//
// These tests build a synthetic 2-repo group with an overlay placing a
// cross-repo entity in a shared community and assert the read-side now serves
// the group view.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cajasmota/grafel/internal/graph"
	mcpapi "github.com/mark3labs/mcp-go/mcp"
)

// twoRepoOverlayServer builds a "test" group with two repos whose entities are
// already overlay-stamped into a single shared group community (id 42) plus one
// backend-only community (id 7), and sets lg.Communities to the group summary —
// exactly the post-applyGroupAlgoOverlay state the read-side must serve.
func twoRepoOverlayServer(t *testing.T) *Server {
	t.Helper()
	backend := &graph.Document{
		Repo: "backend",
		Entities: []graph.Entity{
			{ID: "be_shared", Name: "User", Kind: "SCOPE.Class", SourceFile: "user.py", StartLine: 1,
				CommunityID: ip(42), PageRank: f64p(0.0065), Centrality: f64p(10415.99), IsGodNode: true},
			{ID: "be_only", Name: "Migration", Kind: "SCOPE.Class", SourceFile: "m.py", StartLine: 1,
				CommunityID: ip(7), PageRank: f64p(0.001), Centrality: f64p(3.0)},
		},
	}
	mobile := &graph.Document{
		Repo: "acme-mobile",
		Entities: []graph.Entity{
			{ID: "mob_shared", Name: "ResponsiveView", Kind: "SCOPE.Component", SourceFile: "v.tsx", StartLine: 1,
				CommunityID: ip(42), PageRank: f64p(0.004), Centrality: f64p(120.5)},
		},
	}
	srv := newTestServer(t, backend, mobile)

	// Mirror what applyGroupAlgoOverlay would have set: the group community
	// summary. acme-mobile (#5397) carries members of community 42 even though
	// it has no per-repo Doc.Communities of its own.
	srv.State.mu.Lock()
	lg := srv.State.groups["test"]
	lg.Communities = []graph.CommunityResult{
		{ID: 42, Size: 2, Modularity: 0.7319, TopEntities: []string{"be_shared", "mob_shared"}, AutoName: "user-domain"},
		{ID: 7, Size: 1, Modularity: 0.5, TopEntities: []string{"be_only"}, AutoName: "migrations"},
	}
	srv.State.mu.Unlock()
	return srv
}

func clustersResult(t *testing.T, srv *Server, args map[string]any) []any {
	t.Helper()
	req := mcpapi.CallToolRequest{}
	if args == nil {
		args = map[string]any{}
	}
	args["group"] = "test"
	req.Params.Arguments = args
	res, err := srv.handleListCommunities(context.Background(), req)
	if err != nil {
		t.Fatalf("handleListCommunities error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("clusters tool error: %v", res)
	}
	// handleListCommunities returns a top-level JSON array.
	var arr []any
	if err := json.Unmarshal([]byte(extractResultText(t, res)), &arr); err != nil {
		t.Fatalf("clusters: unmarshal array failed: %v\nraw: %s", err, extractResultText(t, res))
	}
	return arr
}

// TestClusters_ServesGroupOverlay_CrossRepoCommunity asserts handleListCommunities
// returns a community whose members span both repos (#5396) and is NOT
// force-tagged to a single repo — including acme-mobile (#5397).
func TestClusters_ServesGroupOverlay_CrossRepoCommunity(t *testing.T) {
	srv := twoRepoOverlayServer(t)
	items := clustersResult(t, srv, map[string]any{"min_size": 1})

	var crossRepo map[string]any
	for _, it := range items {
		m := it.(map[string]any)
		if int(m["id"].(float64)) == 42 {
			crossRepo = m
		}
		// No group community may be force-tagged to a single repo via "repo".
		if _, hasSingleRepo := m["repo"]; hasSingleRepo {
			t.Errorf("group community %v force-tagged to single repo (per-repo path leaked): %v", m["id"], m)
		}
	}
	if crossRepo == nil {
		t.Fatalf("community 42 (cross-repo) not returned; got %v", items)
	}
	if cr, _ := crossRepo["cross_repo"].(bool); !cr {
		t.Errorf("community 42 not flagged cross_repo: %v", crossRepo)
	}
	reposAny, ok := crossRepo["repos"].([]any)
	if !ok || len(reposAny) != 2 {
		t.Fatalf("community 42 must span 2 repos, got repos=%v", crossRepo["repos"])
	}
	seen := map[string]bool{}
	for _, r := range reposAny {
		seen[r.(string)] = true
	}
	if !seen["backend"] || !seen["acme-mobile"] {
		t.Errorf("community 42 members must span backend AND acme-mobile (#5397), got %v", reposAny)
	}
}

// TestClusters_RepoFilter_KeepsCrossRepoCommunity asserts a repo_filter naming
// only one of a cross-repo community's repos still surfaces that community.
func TestClusters_RepoFilter_KeepsCrossRepoCommunity(t *testing.T) {
	srv := twoRepoOverlayServer(t)
	items := clustersResult(t, srv, map[string]any{"min_size": 1, "repo_filter": []any{"acme-mobile"}})

	found42, found7 := false, false
	for _, it := range items {
		m := it.(map[string]any)
		switch int(m["id"].(float64)) {
		case 42:
			found42 = true
		case 7:
			found7 = true
		}
	}
	if !found42 {
		t.Errorf("repo_filter=acme-mobile dropped cross-repo community 42 (#5397)")
	}
	if found7 {
		t.Errorf("repo_filter=acme-mobile should NOT surface backend-only community 7")
	}
}

// TestInspect_SurfacesOverlayAlgoFields asserts grafel_inspect surfaces the
// overlay community_id/pagerank/centrality when requested via
// include=community,pagerank,centrality (validation flaw 3 / #5396).
func TestInspect_SurfacesOverlayAlgoFields(t *testing.T) {
	srv := twoRepoOverlayServer(t)
	req := mcpapi.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"group":     "test",
		"entity_id": "backend::be_shared",
		"include":   "community,pagerank,centrality",
	}
	res, err := srv.handleGetNode(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGetNode error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("inspect tool error: %v", res)
	}
	out := extractResultJSON(t, res)

	cid, ok := out["community_id"].(float64)
	if !ok || int(cid) != 42 {
		t.Errorf("inspect did not surface overlay community_id=42, got %v", out["community_id"])
	}
	if pr, ok := out["pagerank"].(float64); !ok || pr == 0 {
		t.Errorf("inspect did not surface overlay pagerank, got %v", out["pagerank"])
	}
	if cen, ok := out["centrality"].(float64); !ok || cen == 0 {
		t.Errorf("inspect did not surface overlay centrality, got %v", out["centrality"])
	}
}
