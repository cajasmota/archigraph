package dashboard

// handlers_topology_orphans.go — Topology v2: orphan publisher detector (#1136).
//
//	GET /api/topology/{group}/orphan-publishers
//
// Returns every topic/queue/channel entity that has at least one producer
// (PUBLISHES_TO / WS_EMITS / STREAMS_TO / GRAPHQL_PUBLISHES / WRITES_TO)
// but zero consumers (SUBSCRIBES_TO / WS_SUBSCRIBES_TO / STREAMS_FROM /
// GRAPHQL_SUBSCRIBES / READS_FROM) anywhere in the group.
//
// These are "fire-and-forget" message producers with no known listener —
// a likely dead-letter or integration gap.
//
// Detection algorithm:
//  1. Walk all entities in the group; keep only those whose topology bucket
//     is topic, queue, channel, or subscription (nats_subjects and
//     graphql_subscriptions share these buckets).
//  2. For each such entity, collect producers and consumers using the same
//     brokerEdges / channelEdges helpers already used by collectTopologyResponse.
//  3. Emit a row when len(producers) > 0 && len(consumers) == 0.
//  4. Entities with zero producers AND zero consumers are NOT emitted
//     (those are orphan subscribers, a different endpoint).
//
// Wire shape:
//
//	{
//	  "orphan_publishers": [
//	    {
//	      "id":        "repo::entityId",
//	      "label":     "orders.created",
//	      "broker":    "rabbitmq",
//	      "framework": "",
//	      "repo":      "backend",
//	      "producers": ["repo::entityId1"],
//	      "reason":    "no_subscriber_found"
//	    }
//	  ],
//	  "total": N
//	}
//
// All array fields marshal as [] (never null).

import (
	"net/http"
	"sort"
)

// OrphanPublisherRow is one entity returned by the orphan-publisher endpoint.
type OrphanPublisherRow struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Broker    string   `json:"broker"`
	Framework string   `json:"framework"`
	Repo      string   `json:"repo"`
	Producers []string `json:"producers"`
	Reason    string   `json:"reason"`
}

// orphanPublisherReason is the reason value surfaced to callers.
const reasonNoSubscriberFound = "no_subscriber_found"

// handleOrphanPublishers — GET /api/topology/{group}/orphan-publishers
func (s *Server) handleOrphanPublishers(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	if group == "" {
		writeErr(w, http.StatusBadRequest, "group required")
		return
	}

	grp, err := s.graphs.GetGroup(group)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	rows := collectOrphanPublishers(grp)

	writeJSON(w, http.StatusOK, map[string]any{
		"orphan_publishers": rows,
		"total":             len(rows),
	})
}

// collectOrphanPublishers runs the orphan-publisher detection pass against a
// loaded group. Extracted so unit tests can call it without HTTP scaffolding.
func collectOrphanPublishers(grp *DashGroup) []OrphanPublisherRow {
	var rows []OrphanPublisherRow

	for _, r := range sortedRepos(grp) {
		if r.Doc == nil {
			continue
		}

		for i := range r.Doc.Entities {
			e := &r.Doc.Entities[i]
			bucket := classifyTopologyBucket(e.Kind, e.Name, e.Properties)

			// Only inspect buckets that use broker/channel semantics.
			switch bucket {
			case "topic", "queue", "channel", "subscription":
				// handled below
			default:
				continue
			}

			var producers, consumers []string

			switch bucket {
			case "topic", "queue":
				producers, consumers, _ = brokerEdges(r, e.ID)
			case "channel":
				producers, consumers = channelEdges(r, e.ID)
			case "subscription":
				producers, consumers = graphqlSubEdges(r, e.ID)
			}

			// Emit only when there is at least one producer and zero consumers.
			if len(producers) == 0 || len(consumers) > 0 {
				continue
			}

			broker := e.Properties["broker"]
			if broker == "" {
				broker = inferBrokerFromName(e.Name)
			}
			framework := e.Properties["framework"]

			row := OrphanPublisherRow{
				ID:        dashPrefixedID(r.Slug, e.ID),
				Label:     e.Name,
				Broker:    broker,
				Framework: framework,
				Repo:      r.Slug,
				Producers: producers,
				Reason:    reasonNoSubscriberFound,
			}
			rows = append(rows, row)
		}
	}

	// Stable deterministic sort: repo → label.
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		return a.Label < b.Label
	})

	if rows == nil {
		rows = []OrphanPublisherRow{}
	}
	return rows
}
