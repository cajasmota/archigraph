package types

import (
	"fmt"
	"strings"
)

// RelationshipRecord represents a directed edge between two entities.
// Kind values are enumerated by RelationshipKind (see kinds.go).
type RelationshipRecord struct {
	FromID     string            `json:"from_id"`
	ToID       string            `json:"to_id"`
	Kind       string            `json:"kind"`
	Properties map[string]string `json:"properties,omitempty"`
}

// Validate checks that all required fields are present.
// Returns a single error containing all violations found.
//
// Note: FromID is intentionally allowed to be empty for SELF-referential
// embedded relationships. When an EntityRecord carries an embedded
// RelationshipRecord with FromID == "", the graph assembly loop
// (cmd/archigraph/index.go) substitutes the entity's own hex ID as
// the FromID so that the entity becomes the source of the edge.
// This pattern is used by the testmap extractor (issue #2080) to ensure
// SCOPE.Pattern/unit coverage entities are connected to the graph.
func (r *RelationshipRecord) Validate() error {
	var errs []string
	// FromID == "" is valid for embedded self-referential relationships:
	// the assembly loop fills in the entity's own ID at build time.
	if r.ToID == "" {
		errs = append(errs, "to_id is required")
	}
	if r.Kind == "" {
		errs = append(errs, "kind is required")
	}
	if len(errs) > 0 {
		return fmt.Errorf("RelationshipRecord validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Relationship is an alias used by batch/SQS message types for source→target edges.
// SourceID/TargetID map to FromID/ToID in RelationshipRecord.
// Used by downstream handlers and matches the task brief field names.
type Relationship struct {
	SourceID   string            `json:"source_id"`
	TargetID   string            `json:"target_id"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`
}

// Validate checks that all required fields are present.
func (r *Relationship) Validate() error {
	var errs []string
	if r.SourceID == "" {
		errs = append(errs, "source_id is required")
	}
	if r.TargetID == "" {
		errs = append(errs, "target_id is required")
	}
	if r.Type == "" {
		errs = append(errs, "type is required")
	}
	if len(errs) > 0 {
		return fmt.Errorf("Relationship validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}
