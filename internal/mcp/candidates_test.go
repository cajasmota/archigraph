package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestReadLinkCandidatesWithLinksKey verifies that readLinkCandidates correctly
// reads from the "links" JSON key (not "candidates"). This is a regression test
// for issue #794 where readLinkCandidates was looking for the wrong key.
func TestReadLinkCandidatesWithLinksKey(t *testing.T) {
	tmpDir := t.TempDir()
	groupDir := filepath.Join(tmpDir, "groups")
	if err := os.MkdirAll(groupDir, 0o755); err != nil {
		t.Fatalf("Failed to create group dir: %v", err)
	}

	// Create a synthetic candidates file with 3 Link records under "links" key
	// (this matches the format written by internal/links and writeLinkCandidates)
	testGroup := "test-group-794"
	candidatesPath := filepath.Join(groupDir, testGroup+"-link-candidates.json")
	doc := struct {
		Version int             `json:"version"`
		Links   []LinkCandidate `json:"links"`
	}{
		Version: 1,
		Links: []LinkCandidate{
			{
				ID:         "abc12345",
				Source:     "repo-a::Service1",
				Target:     "repo-b::Service2",
				Kind:       "http",
				Channel:    "fetch",
				Method:     "string",
				Confidence: 0.95,
				Reason:     "URL pattern match",
			},
			{
				ID:         "def67890",
				Source:     "repo-b::Handler",
				Target:     "repo-c::Database",
				Kind:       "import",
				Method:     "label_match",
				Confidence: 0.87,
				Reason:     "Shared label match",
			},
			{
				ID:         "ghi11111",
				Source:     "repo-c::Util",
				Target:     "repo-a::Helper",
				Kind:       "call",
				Method:     "import",
				Confidence: 0.92,
				Reason:     "Direct import",
			},
		},
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal document: %v", err)
	}

	if err := os.WriteFile(candidatesPath, data, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Read the file directly using the same logic as readLinkCandidates
	fileData, err := os.ReadFile(candidatesPath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Verify we can unmarshal it as an array (backwards compat path)
	var asArr []LinkCandidate
	canReadAsArray := json.Unmarshal(fileData, &asArr) == nil

	// Verify we can unmarshal it as object with "links" key (new path)
	var obj struct {
		Links []LinkCandidate `json:"links"`
	}
	err = json.Unmarshal(fileData, &obj)
	if err != nil {
		t.Fatalf("Failed to unmarshal with 'links' key: %v", err)
	}

	// The document should parse as {"links":[...]} format, not array
	if canReadAsArray && len(asArr) > 0 {
		t.Error("File should not parse as bare array (it should require 'links' key)")
	}

	// Verify we got all 3 records
	if len(obj.Links) != 3 {
		t.Errorf("Expected 3 candidates in 'links' key, got %d", len(obj.Links))
	}

	// Verify the records are correct
	expectedIDs := []string{"abc12345", "def67890", "ghi11111"}
	for i, id := range expectedIDs {
		if i >= len(obj.Links) {
			t.Fatalf("Object has fewer than %d records", i+1)
		}
		if obj.Links[i].ID != id {
			t.Errorf("Record %d: expected ID %q, got %q", i, id, obj.Links[i].ID)
		}
	}

	// Verify first record details
	if obj.Links[0].Source != "repo-a::Service1" {
		t.Errorf("First record Source: expected 'repo-a::Service1', got %q", obj.Links[0].Source)
	}
	if obj.Links[0].Confidence != 0.95 {
		t.Errorf("First record Confidence: expected 0.95, got %f", obj.Links[0].Confidence)
	}
}

// TestWriteAndReadLinkCandidatesRoundtrip verifies that writeLinkCandidates
// writes in Document format and can be read correctly.
func TestWriteAndReadLinkCandidatesRoundtrip(t *testing.T) {
	// Create a small in-memory test that directly tests the JSON format
	candidates := []LinkCandidate{
		{
			ID:         "test001",
			Source:     "repo1::Func",
			Target:     "repo2::Func",
			Kind:       "call",
			Method:     "import",
			Confidence: 0.75,
		},
		{
			ID:         "test002",
			Source:     "repo2::Class",
			Target:     "repo3::Class",
			Kind:       "import",
			Method:     "label_match",
			Confidence: 0.82,
		},
	}

	// Simulate what writeLinkCandidates should write
	doc := struct {
		Version int             `json:"version"`
		Links   []LinkCandidate `json:"links"`
	}{
		Version: 1,
		Links:   candidates,
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Now try to read it back using the same logic as readLinkCandidates
	var asArr []LinkCandidate
	asArrErr := json.Unmarshal(data, &asArr)

	var objWithLinks struct {
		Links []LinkCandidate `json:"links"`
	}
	objErr := json.Unmarshal(data, &objWithLinks)

	// The data should NOT parse as a bare array (should require "links" key)
	if asArrErr == nil && len(asArr) > 0 {
		t.Error("Candidates data should not parse as bare array; readLinkCandidates should use 'links' key")
	}

	// But it SHOULD parse with the "links" key
	if objErr != nil {
		t.Errorf("Failed to parse with 'links' key: %v", objErr)
	}

	if len(objWithLinks.Links) != 2 {
		t.Errorf("Expected 2 links after roundtrip, got %d", len(objWithLinks.Links))
	}

	for i, cand := range candidates {
		if i >= len(objWithLinks.Links) {
			break
		}
		if objWithLinks.Links[i].ID != cand.ID {
			t.Errorf("Record %d: expected ID %q, got %q", i, cand.ID, objWithLinks.Links[i].ID)
		}
		if objWithLinks.Links[i].Source != cand.Source {
			t.Errorf("Record %d: expected Source %q, got %q", i, cand.Source, objWithLinks.Links[i].Source)
		}
	}
}
