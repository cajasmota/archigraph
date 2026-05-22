# Docs Skill-Prompt Onboarding (#1584) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the Docs screen so it shows a copyable "run the docs skill" prompt when no skill-generated docs exist, instead of labelling pre-existing repo READMEs as "Generated docs"; when repo READMEs ARE present they appear in a clearly labelled secondary section.

**Architecture:** The backend adds a `skillGenerated bool` field to the docs-tree response by checking for the skill's canonical output structure (`overview.md` or a `modules/` subdirectory in at least one repo's docs dir). The frontend reads this flag; when false it shows the onboarding empty state with a copy-paste prompt instead of the tree. Pre-existing docs that do NOT match the skill structure are surfaced under a secondary "Repository docs" collapse in the tree, never labelled "Generated docs".

**Tech Stack:** Go (net/http), React 18, TypeScript, TanStack Query, Tailwind CSS, Lucide icons, Vite.

**Working directory:** `/Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt`

---

## File Map

| File | Change |
|------|--------|
| `internal/dashboard/handlers_v2_docs.go` | Add `SkillGenerated bool` to tree response; add `isSkillGenerated(docPath)` helper; classify each repo's docs dir; add `isRepoDocs bool` to `v2DocNode` |
| `webui-v2/src/data/types.ts` | Add `skillGenerated?: boolean` to `DocsTreeResponse` wrapper type; add `isRepoDocs?: boolean` to `DocNode` |
| `webui-v2/src/lib/api.ts` | Update `getDocsTree` return type to include `skillGenerated` flag |
| `webui-v2/src/hooks/use-docs.ts` | Expose `skillGenerated` flag from the tree query |
| `webui-v2/src/components/docs/docs-empty.tsx` | Rewrite `DocsNotGenerated` to show the copy-prompt CTA; accept `groupId` prop |
| `webui-v2/src/routes/docs.tsx` | Pass `groupId` to `DocsNotGenerated`; derive `hasSkillDocs` from tree response; conditionally split tree into skill vs repo-docs nodes |
| `webui-v2/src/components/docs/docs-tree.tsx` | Accept optional `repoDocs` prop; render "Repository docs" section below the main tree with a disclosure header |

---

## Task 1: Backend — detect skill-generated docs structure

**Files:**
- Modify: `internal/dashboard/handlers_v2_docs.go`

The skill writes `overview.md` or a `modules/` dir inside `<repo>/docs/`. If any repo in the group has at least one of those, the docs are skill-generated. Non-skill docs (random READMEs, stray `.md` files) never have that structure.

Additionally, tag individual repo nodes with `IsRepoDocs bool` (true = no skill structure, just repo-level md files).

- [ ] **Step 1: Add `SkillGenerated` to the tree response wire shape**

In `handlers_v2_docs.go`, replace the existing `v2DocNode` block and add a `v2DocsTreeReply` wrapper:

```go
// v2DocsTreeReply wraps the per-repo tree with a top-level flag that
// tells the UI whether the skill has run for this group. When false, the
// tree may contain pre-existing repo README files, not skill output.
type v2DocsTreeReply struct {
	SkillGenerated bool        `json:"skillGenerated"`
	Nodes          []v2DocNode `json:"nodes"`
}

// v2DocNode is one node in the generated-docs file tree.
type v2DocNode struct {
	Type        string      `json:"type"`                  // "folder" | "doc"
	Name        string      `json:"name"`
	Path        string      `json:"path,omitempty"`        // doc key (docs only)
	Category    string      `json:"category,omitempty"`    // overview|modules|reference|patterns|guide
	IsRepoDocs  bool        `json:"isRepoDocs,omitempty"`  // true = not skill-generated (raw repo md)
	Children    []v2DocNode `json:"children,omitempty"`
}
```

- [ ] **Step 2: Add `isSkillGeneratedDir` helper**

After the existing `docCategory` function, add:

```go
// isSkillGeneratedDir reports whether a docs directory looks like it was
// produced by the archigraph generate-docs skill.  The skill always writes
// at least one of:
//   - docs/overview.md
//   - docs/modules/ (directory)
//
// Pre-existing repo READMEs never have this structure.
func isSkillGeneratedDir(docPath string) bool {
	if _, err := os.Stat(filepath.Join(docPath, "overview.md")); err == nil {
		return true
	}
	if info, err := os.Stat(filepath.Join(docPath, "modules")); err == nil && info.IsDir() {
		return true
	}
	return false
}
```

- [ ] **Step 3: Update `handleV2DocsTree` to use the new wrapper + set flags**

Replace the body of `handleV2DocsTree` after the `docPaths` is resolved:

```go
func (s *Server) handleV2DocsTree(w http.ResponseWriter, r *http.Request) {
	group := r.PathValue("group")
	if _, err := s.graphs.GetGroup(group); err != nil {
		writeV2Err(w, http.StatusNotFound, "not_found", "group not found: "+group)
		return
	}

	docPaths, err := groupDocPaths(group)
	if err != nil {
		// No registered docs path — not generated yet.
		writeV2JSON(w, http.StatusOK, v2OK(v2DocsTreeReply{SkillGenerated: false, Nodes: []v2DocNode{}}))
		return
	}

	roots := []v2DocNode{}
	skillGenerated := false

	slugs := make([]string, 0, len(docPaths))
	for slug := range docPaths {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	for _, slug := range slugs {
		docPath := docPaths[slug]
		if docPath == "" {
			continue
		}
		if _, err := os.Stat(docPath); err != nil {
			continue
		}
		isSkill := isSkillGeneratedDir(docPath)
		if isSkill {
			skillGenerated = true
		}
		repoNode := buildV2DocTree(docPath, slug)
		repoNode.IsRepoDocs = !isSkill
		if len(repoNode.Children) > 0 {
			roots = append(roots, repoNode)
		}
	}

	writeV2JSON(w, http.StatusOK, v2OK(v2DocsTreeReply{SkillGenerated: skillGenerated, Nodes: roots}))
}
```

- [ ] **Step 4: Build the Go binary to verify it compiles**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt && go build ./internal/dashboard/...
```

Expected: zero output, exit 0.

- [ ] **Step 5: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt
git add internal/dashboard/handlers_v2_docs.go
git commit -m "feat(docs): add skillGenerated flag + isRepoDocs to v2 docs-tree response (Fixes #1584)"
```

---

## Task 2: Frontend types + API client

**Files:**
- Modify: `webui-v2/src/data/types.ts`
- Modify: `webui-v2/src/lib/api.ts`

- [ ] **Step 1: Add `DocsTreeResponse` wrapper type and `isRepoDocs` to `DocNode`**

In `webui-v2/src/data/types.ts`, after the existing `DocPage` interface, add and update:

```typescript
export interface DocNode {
  type: "folder" | "doc";
  name: string;
  path?: string;          // doc leaf only — key for the page endpoint
  category?: DocCategory; // top-level section
  isRepoDocs?: boolean;   // true = not skill-generated (raw repo markdown)
  children?: DocNode[];
}

/** Wrapper returned by GET /api/v2/groups/:id/docs/tree */
export interface DocsTreeResponse {
  skillGenerated: boolean;
  nodes: DocNode[];
}
```

(Replace the existing `DocNode` interface — only adding `isRepoDocs?` line.)

- [ ] **Step 2: Update `getDocsTree` in `api.ts`**

In `webui-v2/src/lib/api.ts`, change:

```typescript
getDocsTree: (groupId: string) =>
  requestV2<DocNode[]>(`/groups/${groupId}/docs/tree`),
```

to:

```typescript
getDocsTree: (groupId: string) =>
  requestV2<DocsTreeResponse>(`/groups/${groupId}/docs/tree`),
```

Also add `DocsTreeResponse` to the import from `@/data/types` at the top of `api.ts` (wherever `DocNode` and `DocPage` are imported).

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt/webui-v2 && npm run build 2>&1 | tail -20
```

Expected: build errors are expected here (use-docs.ts still returns `DocNode[]`). Record the specific errors and proceed to Task 3.

- [ ] **Step 4: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt
git add webui-v2/src/data/types.ts webui-v2/src/lib/api.ts
git commit -m "feat(docs): add DocsTreeResponse type + isRepoDocs to DocNode"
```

---

## Task 3: Hook — expose `skillGenerated` from `useDocsTree`

**Files:**
- Modify: `webui-v2/src/hooks/use-docs.ts`

- [ ] **Step 1: Update `useDocsTree` to expose `skillGenerated`**

Replace the entire contents of `webui-v2/src/hooks/use-docs.ts`:

```typescript
/* ============================================================
   use-docs.ts — TanStack Query hooks for the Docs screen.

   Docs are the GENERATED MARKDOWN documents produced by the
   `generate-docs` skill (run by the user's coding agent), served by
   the v2 endpoints in handlers_v2_docs.go:

     useDocsTree — tree + skillGenerated flag (per-repo → category → doc)
     useDocPage  — raw markdown of one document, by its tree path key
   ============================================================ */

import { useQuery } from "@tanstack/react-query";
import { api, ApiError } from "@/lib/api";
import type { DocNode, DocPage } from "@/data/types";

export interface DocsTreeResult {
  /** Whether the generate-docs skill has run for this group. */
  skillGenerated: boolean;
  /** Per-repo tree of documents. */
  nodes: DocNode[];
}

export function useDocsTree(groupId: string) {
  return useQuery<DocsTreeResult, ApiError>({
    queryKey: ["docs-tree", groupId],
    queryFn: async () => {
      const res = await api.getDocsTree(groupId);
      return { skillGenerated: res.skillGenerated, nodes: res.nodes };
    },
    staleTime: 30_000,
    placeholderData: { skillGenerated: false, nodes: [] },
  });
}

export function useDocPage(groupId: string, path: string | null) {
  return useQuery<DocPage, ApiError>({
    queryKey: ["docs-page", groupId, path],
    queryFn: () => api.getDocPage(groupId, path!),
    enabled: path !== null,
    staleTime: 30_000,
    retry: (count, err) => {
      if (err instanceof ApiError && err.status === 404) return false;
      return count < 2;
    },
  });
}
```

- [ ] **Step 2: Build to check progress**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt/webui-v2 && npm run build 2>&1 | tail -30
```

Expected: errors now shift to `docs.tsx` (still uses old `useDocsTree` return shape). Proceed.

- [ ] **Step 3: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt
git add webui-v2/src/hooks/use-docs.ts
git commit -m "feat(docs): update useDocsTree to return skillGenerated flag + nodes"
```

---

## Task 4: `DocsNotGenerated` — add copy-prompt CTA

**Files:**
- Modify: `webui-v2/src/components/docs/docs-empty.tsx`

The correct skill invocation phrase per the SKILL.md is `/generate-docs` — but there is no CLI command. The prompt the user copies to their coding agent is:
> `/generate-docs` — or in natural language: "Generate documentation for the **<group>** group using the archigraph generate-docs skill."

- [ ] **Step 1: Rewrite `DocsNotGenerated` with copy button**

Replace the entire contents of `webui-v2/src/components/docs/docs-empty.tsx`:

```tsx
/* ============================================================
   docs-empty.tsx — Docs screen empty states (#1552, #1584).

   Two cases:

   • DocsNotGenerated — the generate-docs SKILL has not run for this group.
     Shows a copyable prompt the user pastes into their coding agent.

   • DocsPickDocument — documents exist but none is selected yet.
   ============================================================ */

import { useState, useCallback } from "react";
import { BookOpen, Sparkles, Copy, Check } from "lucide-react";

interface DocsNotGeneratedProps {
  /** The current archigraph group slug — used in the copy prompt. */
  groupId: string;
}

// Whole-screen state: no skill-generated documents for this group.
export function DocsNotGenerated({ groupId }: DocsNotGeneratedProps) {
  const [copied, setCopied] = useState(false);

  // The prompt the user pastes into their coding agent (Claude Code, Cursor, etc.)
  const prompt = `/generate-docs`;

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(prompt);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard API unavailable — select the text instead
      const el = document.getElementById("docs-skill-prompt");
      if (el) {
        const range = document.createRange();
        range.selectNodeContents(el);
        const sel = window.getSelection();
        sel?.removeAllRanges();
        sel?.addRange(range);
      }
    }
  }, [prompt]);

  return (
    <div className="flex flex-1 items-center justify-center px-6">
      <div className="flex flex-col items-center text-center max-w-lg gap-5">
        <span className="text-text-4" aria-hidden="true">
          <Sparkles size={36} strokeWidth={1.25} />
        </span>
        <div className="flex flex-col gap-2">
          <h2 className="text-lg font-medium text-text">No generated docs yet</h2>
          <p className="text-sm text-text-3 leading-relaxed">
            Documentation for the{" "}
            <span className="font-mono text-text-2">{groupId}</span> group is produced
            by your coding agent running the{" "}
            <span className="font-mono text-text-2">generate-docs</span> skill. There
            is no CLI command — ask your agent directly.
          </p>
        </div>

        {/* Copy-paste CTA */}
        <div className="w-full rounded-lg border border-border bg-surface p-4 flex flex-col gap-3">
          <p className="text-xs font-medium text-text-2 text-left">
            Paste this into your coding agent (Claude Code, Cursor, etc.):
          </p>
          <div className="flex items-center gap-2 rounded-md bg-surface-2 border border-border px-3 py-2">
            <code
              id="docs-skill-prompt"
              className="flex-1 text-sm font-mono text-[var(--accent)] select-all"
            >
              {prompt}
            </code>
            <button
              onClick={handleCopy}
              aria-label={copied ? "Copied" : "Copy prompt"}
              className="shrink-0 p-1 rounded text-text-4 hover:text-text-2 hover:bg-surface transition-colors"
            >
              {copied ? (
                <Check size={14} className="text-green-500" />
              ) : (
                <Copy size={14} />
              )}
            </button>
          </div>
          <p className="text-xs text-text-4 text-left leading-relaxed">
            The skill walks the knowledge graph and writes navigable markdown
            (overview, per-module deep-dives, reference, patterns) into each
            repo&rsquo;s <span className="font-mono">docs/</span> folder. Once
            complete, those documents appear here grouped by repo and category.
          </p>
        </div>
      </div>
    </div>
  );
}

// Right-pane state: documents exist, but the user hasn't opened one.
export function DocsPickDocument() {
  return (
    <div className="flex flex-col items-center justify-center h-full gap-3 px-6 text-center">
      <span className="text-text-4" aria-hidden="true">
        <BookOpen size={32} strokeWidth={1.25} />
      </span>
      <h2 className="text-base font-medium text-text">Pick a document</h2>
      <p className="text-sm text-text-3 max-w-xs">
        Choose a document from the index on the left, or search by name above.
        Each page renders the markdown your coding agent generated.
      </p>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt
git add webui-v2/src/components/docs/docs-empty.tsx
git commit -m "feat(docs): add copy-prompt CTA to DocsNotGenerated (#1584)"
```

---

## Task 5: `DocsTree` — add secondary "Repository docs" section

**Files:**
- Modify: `webui-v2/src/components/docs/docs-tree.tsx`

When the tree contains nodes with `isRepoDocs: true`, render them below the generated-docs tree under a collapsed "Repository docs" section — so they're findable but clearly secondary.

- [ ] **Step 1: Update `DocsTree` to accept and render repo docs**

In `webui-v2/src/components/docs/docs-tree.tsx`, update the `DocsTreeProps` interface and `DocsTree` function.

Add to `DocsTreeProps`:
```typescript
export interface DocsTreeProps {
  tree: DocNode[];
  repoDocs: DocNode[];     // nodes with isRepoDocs=true (raw repo READMEs)
  selectedPath: string | null;
  onSelect: (path: string) => void;
  query: string;
}
```

Update `DocsTree` function — add `repoDocs` destructuring and a secondary section:
```typescript
export function DocsTree({ tree, repoDocs, selectedPath, onSelect, query }: DocsTreeProps) {
  const [openMap, setOpenMap] = useState<Record<string, boolean>>({});
  const [repoDocsOpen, setRepoDocsOpen] = useState(false);

  const handleToggle = useCallback((key: string) => {
    const depthSuffix = key.slice(key.lastIndexOf(":") + 1);
    const defaultVal = depthSuffix === "0" || depthSuffix === "1";
    setOpenMap((prev) => ({ ...prev, [key]: !(prev[key] ?? defaultVal) }));
  }, []);

  const totalDocs = useMemo(() => tree.reduce((s, r) => s + countDocs(r), 0), [tree]);
  const totalRepoDocs = useMemo(() => repoDocs.reduce((s, r) => s + countDocs(r), 0), [repoDocs]);

  const lowerQ = query.toLowerCase();
  const noMatches =
    !!query &&
    tree.every((r) => !hasMatch(r, lowerQ)) &&
    repoDocs.every((r) => !hasMatch(r, lowerQ));

  return (
    <div className="flex flex-col h-full w-[320px] shrink-0 border-r border-border overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-border shrink-0">
        <span className="text-sm font-medium text-text">Documentation</span>
        <span className="text-xs font-mono text-text-3 tabular-nums">
          {totalDocs.toLocaleString()}
        </span>
      </div>

      <div className="flex-1 overflow-y-auto py-1 px-1">
        {noMatches ? (
          <p className="px-3 py-4 text-sm text-text-3 text-center">
            No documents match &ldquo;{query}&rdquo;
          </p>
        ) : (
          <>
            {tree.map((repo, i) => (
              <TreeNode
                key={repo.name + "-" + i}
                node={repo}
                depth={0}
                selectedPath={selectedPath}
                onSelect={onSelect}
                query={query}
                openMap={openMap}
                onToggle={handleToggle}
              />
            ))}

            {/* Secondary: pre-existing repo docs that are NOT skill output */}
            {repoDocs.length > 0 && (
              <div className="mt-3 border-t border-border pt-2">
                <button
                  className="flex items-center gap-1.5 w-full px-3 py-1 text-xs text-text-4 hover:text-text-2 transition-colors"
                  onClick={() => setRepoDocsOpen((v) => !v)}
                  aria-expanded={repoDocsOpen}
                >
                  <ChevronRight
                    size={11}
                    className={["shrink-0 transition-transform", repoDocsOpen ? "rotate-90" : ""].join(" ")}
                  />
                  <span className="font-medium uppercase tracking-wide">Repository docs</span>
                  <span className="ml-auto font-mono tabular-nums">{totalRepoDocs}</span>
                </button>
                {repoDocsOpen &&
                  repoDocs.map((repo, i) => (
                    <TreeNode
                      key={repo.name + "-rd-" + i}
                      node={repo}
                      depth={0}
                      selectedPath={selectedPath}
                      onSelect={onSelect}
                      query={query}
                      openMap={openMap}
                      onToggle={handleToggle}
                    />
                  ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt
git add webui-v2/src/components/docs/docs-tree.tsx
git commit -m "feat(docs): add secondary Repository docs section to DocsTree"
```

---

## Task 6: Wire everything in `docs.tsx`

**Files:**
- Modify: `webui-v2/src/routes/docs.tsx`

- [ ] **Step 1: Update `docs.tsx` to use new hook shape + pass props**

Replace the entire contents of `webui-v2/src/routes/docs.tsx`:

```tsx
/* ============================================================
   docs.tsx — Docs screen: GENERATED markdown documents (#1552, #1584).
   Route: /g/:groupId/docs  and  /g/:groupId/docs/<repoSlug/rel/path.md>

   States:
   - No skill-generated docs → DocsNotGenerated (whole screen) with copy-prompt CTA
   - Docs exist, none selected → DocsPickDocument (right pane)
   - Loading page → skeleton text
   - Loaded → DocsReader
   - 404 page → redirects to base /g/:groupId/docs
   ============================================================ */

import { useState, useEffect, useCallback, useRef, useMemo } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { Search, X } from "lucide-react";
import { Kbd } from "@/components/ui";
import { useDocsTree, useDocPage } from "@/hooks/use-docs";
import { ApiError } from "@/lib/api";
import { DocsTree } from "@/components/docs/docs-tree";
import { DocsReader } from "@/components/docs/docs-reader";
import { DocsNotGenerated, DocsPickDocument } from "@/components/docs/docs-empty";

// ── DocsScreen ────────────────────────────────────────────────────────────────

export default function DocsScreen() {
  const { groupId = "demo" } = useParams<{ groupId: string }>();
  const params = useParams();
  const wildcardPath = (params["*"] ?? "") as string;
  const navigate = useNavigate();

  const [search, setSearch] = useState("");
  const [selectedPath, setSelectedPath] = useState<string | null>(wildcardPath || null);

  useEffect(() => {
    setSelectedPath(wildcardPath || null);
  }, [wildcardPath]);

  const handleSelect = useCallback(
    (path: string) => {
      setSelectedPath(path);
      const encoded = path.split("/").map(encodeURIComponent).join("/");
      navigate(`/g/${groupId}/docs/${encoded}`, { replace: false });
    },
    [groupId, navigate],
  );

  const { data: treeResult, isLoading: treeLoading } = useDocsTree(groupId);
  const {
    data: page,
    isLoading: pageLoading,
    error: pageError,
  } = useDocPage(groupId, selectedPath);

  // On 404 (doc removed/renamed), redirect to the base docs page.
  useEffect(() => {
    if (pageError instanceof ApiError && pageError.status === 404) {
      navigate(`/g/${groupId}/docs`, { replace: true });
    }
  }, [pageError, groupId, navigate]);

  // Debounce search ~120ms.
  const [debouncedSearch, setDebouncedSearch] = useState("");
  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search), 120);
    return () => clearTimeout(t);
  }, [search]);

  // Split tree into skill-generated nodes vs raw repo-doc nodes.
  const skillNodes = useMemo(
    () => (treeResult?.nodes ?? []).filter((n) => !n.isRepoDocs),
    [treeResult],
  );
  const repoDocNodes = useMemo(
    () => (treeResult?.nodes ?? []).filter((n) => n.isRepoDocs),
    [treeResult],
  );

  const hasSkillDocs = !treeLoading && (treeResult?.skillGenerated ?? false) && skillNodes.length > 0;

  // Keyboard: "/" focuses search input
  const searchRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "/" && document.activeElement?.tagName !== "INPUT") {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  return (
    <div className="flex flex-col h-full">
      {/* Controls row */}
      <div className="flex items-center gap-3 h-10 shrink-0 px-4 border-b border-border bg-bg">
        <div className="relative flex items-center flex-1 max-w-xs">
          <Search size={13} className="absolute left-2.5 text-text-4 pointer-events-none" />
          <input
            ref={searchRef}
            type="text"
            className="w-full pl-8 pr-8 h-7 rounded-md bg-surface border border-border text-sm text-text placeholder:text-text-4 focus:outline-none focus:ring-1 focus:ring-[var(--accent)] focus:border-[var(--accent)]"
            placeholder="Search documents…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          {search ? (
            <button
              className="absolute right-2 text-text-4 hover:text-text-2"
              onClick={() => setSearch("")}
              aria-label="Clear"
            >
              <X size={11} />
            </button>
          ) : (
            <Kbd className="absolute right-2 text-[10px]">/</Kbd>
          )}
        </div>
        {hasSkillDocs && (
          <span className="text-xs text-text-3 ml-auto shrink-0">Generated docs</span>
        )}
      </div>

      {/* No skill-generated docs → onboarding CTA */}
      {!treeLoading && !hasSkillDocs ? (
        <DocsNotGenerated groupId={groupId} />
      ) : (
        <div className="flex flex-1 min-h-0">
          {/* Left pane: document index */}
          {treeLoading ? (
            <div className="w-[320px] shrink-0 border-r border-border flex items-center justify-center">
              <span className="text-sm text-text-4">Loading…</span>
            </div>
          ) : (
            <DocsTree
              tree={skillNodes}
              repoDocs={repoDocNodes}
              selectedPath={selectedPath}
              onSelect={handleSelect}
              query={debouncedSearch}
            />
          )}

          {/* Right pane: rendered markdown */}
          <div className="flex-1 overflow-y-auto">
            {!selectedPath ? (
              <DocsPickDocument />
            ) : pageLoading ? (
              <div className="mx-auto max-w-3xl px-8 py-6 space-y-3 animate-pulse">
                <div className="h-7 w-1/2 rounded bg-surface-2" />
                <div className="h-4 w-full rounded bg-surface-2" />
                <div className="h-4 w-5/6 rounded bg-surface-2" />
                <div className="h-4 w-2/3 rounded bg-surface-2" />
              </div>
            ) : page ? (
              <DocsReader page={page} />
            ) : null}
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Build to check TypeScript compiles cleanly**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt/webui-v2 && npm run build 2>&1 | tail -30
```

Expected: clean build, zero TypeScript errors.

- [ ] **Step 3: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt
git add webui-v2/src/routes/docs.tsx
git commit -m "feat(docs): wire skillGenerated flag into DocsScreen, split skill vs repo-doc nodes"
```

---

## Task 7: Full build verification + npm run build

**Files:** No code changes — verification only.

- [ ] **Step 1: Full build**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt/webui-v2 && npm run build 2>&1
```

Expected: zero TypeScript errors, successful Vite build. Build output in `dist/`.

- [ ] **Step 2: Verify dashboard/ is untouched**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt && git diff --name-only HEAD~5 HEAD | grep "^dashboard/"
```

Expected: empty output (no dashboard files changed).

- [ ] **Step 3: Go build**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt && go build ./...
```

Expected: zero output, exit 0.

- [ ] **Step 4: Dev server screenshot (isolated port)**

Start the dev server on a port that is NOT 47274:

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt/webui-v2 && VITE_API_BASE=http://localhost:47274 npx vite --port 15840 &
```

Navigate to `http://localhost:15840/g/polyglot-platform/docs` and take a screenshot.

Expected: Docs screen shows the copy-prompt onboarding state (Sparkles icon, `/generate-docs` prompt, Copy button), NOT a list of stray READMEs labelled "Generated docs".

- [ ] **Step 5: Kill dev server**

```bash
kill $(lsof -ti:15840) 2>/dev/null || true
```

---

## Task 8: Open PR

- [ ] **Step 1: Push branch**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/fix-docs-prompt && git push -u origin feat/docs-prompt
```

- [ ] **Step 2: Create PR**

```bash
gh pr create \
  --title "feat(docs): show copy-prompt CTA when skill hasn't run; separate repo READMEs from generated docs" \
  --body "$(cat <<'EOF'
## What changed

- **Backend** (`handlers_v2_docs.go`): the `/api/v2/groups/:id/docs/tree` response now includes a top-level `skillGenerated` boolean and per-node `isRepoDocs` flag. Detection: if a repo's docs dir contains `overview.md` or a `modules/` subdirectory (the known skill output structure), it is skill-generated. Otherwise it is raw repo markdown.

- **Types + API client** (`types.ts`, `api.ts`): added `DocsTreeResponse` wrapper shape and `isRepoDocs` to `DocNode`.

- **Hook** (`use-docs.ts`): `useDocsTree` now returns `{ skillGenerated, nodes }` instead of a flat `DocNode[]`.

- **Empty state** (`docs-empty.tsx`): `DocsNotGenerated` now shows a prominent copy-paste CTA — the user copies \`/generate-docs\` (the correct skill invocation) to their coding agent. Copy button with clipboard API + fallback select. Accepts \`groupId\` prop for the description copy.

- **Tree** (`docs-tree.tsx`): accepts new `repoDocs` prop. Pre-existing repo markdown files appear in a collapsed "Repository docs" section below the skill-generated docs. Never labelled "Generated docs".

- **Route** (`docs.tsx`): uses `skillGenerated` flag to decide between onboarding state and doc view; splits tree into `skillNodes` + `repoDocNodes`; the "Generated docs" label in the toolbar is hidden when no skill docs exist.

## Detection logic

```
isSkillGeneratedDir(docPath) → true
  when docs/overview.md exists
  OR   docs/modules/ directory exists
```

Pre-existing repo `docs/` folders (e.g. `wasm/cart-pricing/README.md`) never have this structure, so they get `isRepoDocs: true` and flow to the secondary section.

## Verify

1. `npm run build` clean in `webui-v2/` ✓
2. `go build ./...` clean ✓
3. `dashboard/` zero diff ✓
4. Dev server on polyglot-platform (no skill docs yet) → copy-prompt CTA visible, copy button works
5. Pre-existing repo docs shown under "Repository docs" collapse, not "Generated docs"

Fixes #1584
EOF
)"
```

---

## Self-Review

**Spec coverage check:**

| Requirement | Task |
|-------------|------|
| Detect skill-generated vs stray repo READMEs | Task 1 (`isSkillGeneratedDir`) |
| Show copyable prompt when no skill docs | Task 4 (`DocsNotGenerated`) |
| Correct skill invocation phrase (`/generate-docs`) | Task 4 |
| Copy button | Task 4 |
| Pre-existing docs in separate labelled section | Task 5 + Task 6 |
| Do not label repo READMEs as "Generated docs" | Task 6 (toolbar label only shows when `hasSkillDocs`) |
| Keep rendering generated MD when skill HAS run | Task 6 (existing DocsReader path unchanged) |
| npm run build clean | Task 7 |
| dashboard/ zero diff | Task 7 |
| Screenshot | Task 7 |
| PR to main, 6-section, Fixes #1584 | Task 8 |
| Do NOT touch graph/flows/topology/paths/operations | Scope — none of those files appear in this plan |

**Placeholder scan:** No TBDs. All code blocks are complete.

**Type consistency:**
- `DocsTreeResult` (hook interface) has `skillGenerated: boolean` + `nodes: DocNode[]` — matches `DocsTreeResponse` from `api.ts`.
- `DocNode.isRepoDocs` added in `types.ts` — used as `n.isRepoDocs` in `docs.tsx` and as `isRepoDocs` in backend response.
- `DocsTree` now requires `repoDocs` prop — `docs.tsx` passes `repoDocNodes`.
- `DocsNotGenerated` now requires `groupId` prop — `docs.tsx` passes `groupId`.
