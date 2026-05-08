package mcp

import (
	"container/heap"
	"math"

	"github.com/cajasmota/archigraph/internal/graph"
)

// adjacency is a per-repo precomputed neighbor map.
type adjacency struct {
	out map[string][]edge
	in  map[string][]edge
}

// edge is a typed neighbor reference; weight is for shortest-path scoring.
type edge struct {
	target string
	kind   string
	weight float64
	repo   string
}

// buildAdjacency constructs in/out neighbor lists for one repo.
func buildAdjacency(doc *graph.Document, repo string) *adjacency {
	a := &adjacency{
		out: make(map[string][]edge, len(doc.Entities)),
		in:  make(map[string][]edge, len(doc.Entities)),
	}
	for i := range doc.Relationships {
		r := &doc.Relationships[i]
		w := 1.0
		a.out[r.FromID] = append(a.out[r.FromID], edge{target: r.ToID, kind: r.Kind, weight: w, repo: repo})
		a.in[r.ToID] = append(a.in[r.ToID], edge{target: r.FromID, kind: r.Kind, weight: w, repo: repo})
	}
	return a
}

// bfs walks `depth` hops outward from `start` along the given adjacency,
// returning the visited set as a map id->depth.
func bfs(adj *adjacency, start string, depth int, contextFilter map[string]bool) map[string]int {
	visited := map[string]int{start: 0}
	frontier := []string{start}
	for d := 0; d < depth; d++ {
		next := []string{}
		for _, n := range frontier {
			for _, e := range adj.out[n] {
				if contextFilter != nil && !contextFilter[e.kind] {
					continue
				}
				if _, seen := visited[e.target]; seen {
					continue
				}
				visited[e.target] = d + 1
				next = append(next, e.target)
			}
			for _, e := range adj.in[n] {
				if contextFilter != nil && !contextFilter[e.kind] {
					continue
				}
				if _, seen := visited[e.target]; seen {
					continue
				}
				visited[e.target] = d + 1
				next = append(next, e.target)
			}
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}
	return visited
}

// pqItem is one entry in the dijkstra priority queue.
type pqItem struct {
	id    string // prefixed: <repo>::<localId>
	cost  float64
	prev  string
	prevK string
	index int
}

type pq []*pqItem

func (p pq) Len() int            { return len(p) }
func (p pq) Less(i, j int) bool  { return p[i].cost < p[j].cost }
func (p pq) Swap(i, j int)       { p[i], p[j] = p[j], p[i]; p[i].index = i; p[j].index = j }
func (p *pq) Push(x interface{}) { *p = append(*p, x.(*pqItem)) }
func (p *pq) Pop() interface{}   { old := *p; n := len(old); x := old[n-1]; *p = old[:n-1]; return x }

// dijkstra finds the shortest path from src to dst using a callable that
// expands neighbors. node IDs are prefixed (<repo>::<localId>) so this works
// across repos via overlay edges. Returns (path, edgeKinds, weakest, ok).
//
// expand returns []edge for the given prefixed node id.
func dijkstra(src, dst string, expand func(string) []edge) ([]string, []string, float64, bool) {
	if src == dst {
		return []string{src}, nil, 1.0, true
	}
	dist := map[string]float64{src: 0}
	prev := map[string]string{}
	prevKind := map[string]string{}
	q := &pq{}
	heap.Init(q)
	heap.Push(q, &pqItem{id: src, cost: 0})
	for q.Len() > 0 {
		cur := heap.Pop(q).(*pqItem)
		if cur.id == dst {
			path := []string{dst}
			edges := []string{prevKind[dst]}
			at := dst
			for prev[at] != "" {
				at = prev[at]
				path = append([]string{at}, path...)
				if k, ok := prevKind[at]; ok && at != src {
					edges = append([]string{k}, edges...)
				}
			}
			weakest := math.Inf(1)
			for _, p := range path[1:] {
				if d, ok := dist[p]; ok && d-(dist[prev[p]]) < weakest {
					weakest = d - dist[prev[p]]
				}
			}
			if math.IsInf(weakest, 1) {
				weakest = 1.0
			}
			// Convert "weight" cost back to confidence. Higher weight = lower
			// confidence; we used cost = -log(confidence).
			conf := math.Exp(-weakest)
			return path, edges, conf, true
		}
		if cur.cost > dist[cur.id] {
			continue
		}
		for _, e := range expand(cur.id) {
			nd := cur.cost + e.weight
			if old, ok := dist[e.target]; !ok || nd < old {
				dist[e.target] = nd
				prev[e.target] = cur.id
				prevKind[e.target] = e.kind
				heap.Push(q, &pqItem{id: e.target, cost: nd})
			}
		}
	}
	return nil, nil, 0, false
}
