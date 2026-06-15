package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/skills/installed
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleSkillsInstalled_Empty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/skills/installed", nil)
	w := httptest.NewRecorder()
	s.handleSkillsInstalled(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var reply SkillsInstalledReply
	if err := json.NewDecoder(w.Body).Decode(&reply); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(reply.Skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(reply.Skills))
	}
}

func TestHandleSkillsInstalled_WithSkills(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	// Create a skill directory with a SKILL.md
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
name: my-skill
description: A test skill
type: action
when-to-use: In tests
version: 1.0.0
---

# my-skill

A test skill.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/skills/installed", nil)
	w := httptest.NewRecorder()
	s.handleSkillsInstalled(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var reply SkillsInstalledReply
	if err := json.NewDecoder(w.Body).Decode(&reply); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(reply.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(reply.Skills))
	}
	sk := reply.Skills[0]
	if sk.Slug != "my-skill" {
		t.Errorf("slug: got %q, want %q", sk.Slug, "my-skill")
	}
	if sk.Name != "my-skill" {
		t.Errorf("name: got %q, want %q", sk.Name, "my-skill")
	}
	if sk.Version != "1.0.0" {
		t.Errorf("version: got %q, want %q", sk.Version, "1.0.0")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/skills/available
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleSkillsAvailable_Shape(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/skills/available", nil)
	w := httptest.NewRecorder()
	s.handleSkillsAvailable(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var reply SkillsAvailableReply
	if err := json.NewDecoder(w.Body).Decode(&reply); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(reply.Skills) == 0 {
		t.Fatal("expected non-empty catalog")
	}
	// All entries must have a slug
	for _, sk := range reply.Skills {
		if sk.Slug == "" {
			t.Errorf("catalog entry missing slug: %+v", sk)
		}
	}
}

func TestHandleSkillsAvailable_InstalledFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	// Pre-install one of the bundled skills
	skillDir := filepath.Join(dir, "using-grafel")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/skills/available", nil)
	w := httptest.NewRecorder()
	s.handleSkillsAvailable(w, req)

	var reply SkillsAvailableReply
	if err := json.NewDecoder(w.Body).Decode(&reply); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, sk := range reply.Skills {
		if sk.Slug == "using-grafel" {
			if !sk.Installed {
				t.Error("expected using-grafel to be flagged installed=true")
			}
			return
		}
	}
	t.Error("using-grafel not found in catalog")
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/skills/install
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleSkillsInstall_OK(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"slug": "using-grafel"})
	req := httptest.NewRequest(http.MethodPost, "/api/skills/install", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSkillsInstall(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// SKILL.md should now exist
	skillMD := filepath.Join(dir, "using-grafel", "SKILL.md")
	if _, err := os.Stat(skillMD); os.IsNotExist(err) {
		t.Error("SKILL.md was not created after install")
	}
}

func TestHandleSkillsInstall_UnknownSlug(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"slug": "does-not-exist"})
	req := httptest.NewRequest(http.MethodPost, "/api/skills/install", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSkillsInstall(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleSkillsInstall_EmptyBody(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/skills/install", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	s.handleSkillsInstall(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /api/skills/uninstall
// ─────────────────────────────────────────────────────────────────────────────

func TestHandleSkillsUninstall_OK(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	// Pre-create a skill directory
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"slug": "my-skill"})
	req := httptest.NewRequest(http.MethodPost, "/api/skills/uninstall", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSkillsUninstall(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Directory must be gone
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("skill directory still exists after uninstall")
	}
}

func TestHandleSkillsUninstall_NotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"slug": "not-installed"})
	req := httptest.NewRequest(http.MethodPost, "/api/skills/uninstall", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSkillsUninstall(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleSkillsUninstall_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GRAFEL_SKILLS_DIR", dir)

	s, err := NewServer(DefaultConfig(), newFakeStore())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"slug": "../../../etc/passwd"})
	req := httptest.NewRequest(http.MethodPost, "/api/skills/uninstall", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleSkillsUninstall(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal, got %d", w.Code)
	}
}
