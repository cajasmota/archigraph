# Graph Live Rendering Tuning Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Rendering" collapsible sidebar panel to the graph view so the owner can dial point opacity, point size scale, scale-on-zoom, max point size, link opacity, link width scale, and show/hide links live without restarting the simulation.

**Architecture:** Follow the exact pattern used by `useNodeSizingConfig` + `NodeSizingControls` and `useSimulationConfig` + `SimulationControls`. A new `useRenderConfig` hook holds state + localStorage persistence. A new `RenderingControls` component renders sliders and toggles. The route (`graph.tsx`) instantiates the hook and wires it down to both `RenderingControls` (UI) and `GraphCanvas` (new `renderConfig` prop). `GraphCanvas` applies the config via `setConfig()` in a dedicated `useEffect` that runs on config change without recreating the Graph instance.

**Tech Stack:** React 18, TypeScript, Tailwind CSS, `@cosmos.gl/graph` 2.6.4, localStorage, lucide-react icons.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| **Create** | `dashboard/src/hooks/graph/useRenderConfig.ts` | State, defaults, localStorage persistence, `RenderConfig` type |
| **Create** | `dashboard/src/components/graph/RenderingControls.tsx` | Collapsible sidebar panel — sliders + toggles |
| **Modify** | `dashboard/src/components/graph/GraphCanvas.tsx` | Accept `renderConfig?: RenderConfig` prop; wire `setConfig()` effect; replace hardcoded render params with defaults |
| **Modify** | `dashboard/src/routes/graph.tsx` | Instantiate `useRenderConfig`, render `<RenderingControls>`, pass config to `<GraphCanvas>` |

---

## Task 1: Create `useRenderConfig` hook

**Files:**
- Create: `dashboard/src/hooks/graph/useRenderConfig.ts`

- [ ] **Step 1: Write the hook file**

```typescript
/**
 * useRenderConfig — persisted tunable rendering config.
 *
 * Exposes live controls for cosmos.gl rendering knobs that were previously
 * hardcoded in GraphCanvas, causing whack-a-mole visual regressions.
 *
 * Persisted to localStorage under key `archigraph:graph:rendering`.
 * Changes apply immediately via setConfig() — no simulation restart.
 */
import { useState, useCallback } from 'react'

// ---------------------------------------------------------------------------
// Types + defaults
// ---------------------------------------------------------------------------

export interface RenderConfig {
  /** cosmos pointOpacity — overall node opacity. Range 0.05–1.0 */
  pointOpacity: number
  /** cosmos pointSizeScale — global multiplier on setPointSizes values. Range 0.05–2.0 */
  pointSizeScale: number
  /** cosmos scalePointsOnZoom — nodes grow with zoom level when true */
  scalePointsOnZoom: boolean
  /**
   * maxPointSize — soft clamp on rendered node size in screen-px.
   * cosmos.gl has no direct maxPointSize option; we enforce it by capping
   * pointSizeScale to: effectiveScale = min(pointSizeScale, maxPointSize / baseSize)
   * (see GraphCanvas applyRenderConfig). Range 4–200 px.
   */
  maxPointSize: number
  /**
   * linkOpacity — alpha channel applied to same-repo link colors (state 0).
   * Cross-repo (state 1) and highlighted (state 2) links keep their own alphas.
   * Range 0–1.
   */
  linkOpacity: number
  /** cosmos linkWidthScale — global multiplier on setLinkWidths values. Range 0.05–2.0 */
  linkWidthScale: number
  /** showLinks — hide all edges when false (linkWidthScale driven to 0) */
  showLinks: boolean
}

// Keep current hardcoded GraphCanvas values as defaults so nothing changes on
// first load until the owner explicitly tweaks a knob.
export const DEFAULT_RENDER_CONFIG: RenderConfig = {
  pointOpacity:      0.25,
  pointSizeScale:    0.22,
  scalePointsOnZoom: true,
  maxPointSize:      150,
  linkOpacity:       0.15,
  linkWidthScale:    0.16,
  showLinks:         true,
}

const STORAGE_KEY = 'archigraph:graph:rendering'

// ---------------------------------------------------------------------------
// Storage helpers
// ---------------------------------------------------------------------------

function loadFromStorage(): RenderConfig | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const p = JSON.parse(raw) as Partial<RenderConfig>
    return {
      pointOpacity:      clampF(p.pointOpacity,   0.05, 1.0,   DEFAULT_RENDER_CONFIG.pointOpacity),
      pointSizeScale:    clampF(p.pointSizeScale,  0.05, 2.0,   DEFAULT_RENDER_CONFIG.pointSizeScale),
      scalePointsOnZoom: typeof p.scalePointsOnZoom === 'boolean' ? p.scalePointsOnZoom : DEFAULT_RENDER_CONFIG.scalePointsOnZoom,
      maxPointSize:      clampF(p.maxPointSize,    4,    200,   DEFAULT_RENDER_CONFIG.maxPointSize),
      linkOpacity:       clampF(p.linkOpacity,     0,    1.0,   DEFAULT_RENDER_CONFIG.linkOpacity),
      linkWidthScale:    clampF(p.linkWidthScale,  0.05, 2.0,   DEFAULT_RENDER_CONFIG.linkWidthScale),
      showLinks:         typeof p.showLinks === 'boolean' ? p.showLinks : DEFAULT_RENDER_CONFIG.showLinks,
    }
  } catch {
    return null
  }
}

function clampF(v: unknown, min: number, max: number, fallback: number): number {
  if (typeof v !== 'number' || !isFinite(v)) return fallback
  return Math.max(min, Math.min(max, v))
}

function saveToStorage(cfg: RenderConfig): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(cfg))
  } catch {
    // Fail silently (localStorage unavailable / quota exceeded)
  }
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export interface UseRenderConfigReturn {
  config: RenderConfig
  setParam: <K extends keyof RenderConfig>(key: K, value: RenderConfig[K]) => void
  resetToDefaults: () => void
  /** True if any value differs from the defaults */
  isModified: boolean
}

export function useRenderConfig(): UseRenderConfigReturn {
  const [config, setConfig] = useState<RenderConfig>(
    () => loadFromStorage() ?? { ...DEFAULT_RENDER_CONFIG },
  )

  const setParam = useCallback(<K extends keyof RenderConfig>(key: K, value: RenderConfig[K]) => {
    setConfig((prev) => {
      const next = { ...prev, [key]: value }
      saveToStorage(next)
      return next
    })
  }, [])

  const resetToDefaults = useCallback(() => {
    const next = { ...DEFAULT_RENDER_CONFIG }
    setConfig(next)
    saveToStorage(next)
  }, [])

  const isModified = (
    config.pointOpacity      !== DEFAULT_RENDER_CONFIG.pointOpacity      ||
    config.pointSizeScale    !== DEFAULT_RENDER_CONFIG.pointSizeScale    ||
    config.scalePointsOnZoom !== DEFAULT_RENDER_CONFIG.scalePointsOnZoom ||
    config.maxPointSize      !== DEFAULT_RENDER_CONFIG.maxPointSize      ||
    config.linkOpacity       !== DEFAULT_RENDER_CONFIG.linkOpacity       ||
    config.linkWidthScale    !== DEFAULT_RENDER_CONFIG.linkWidthScale    ||
    config.showLinks         !== DEFAULT_RENDER_CONFIG.showLinks
  )

  return { config, setParam, resetToDefaults, isModified }
}
```

- [ ] **Step 2: Verify file compiles (TypeScript check — no build step needed yet)**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel/dashboard
npx tsc --noEmit --project tsconfig.json 2>&1 | head -30
```

Expected: No errors related to `useRenderConfig.ts`.

- [ ] **Step 3: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel
git add dashboard/src/hooks/graph/useRenderConfig.ts
git commit -m "feat(graph): add useRenderConfig hook — 7 rendering knobs, localStorage persistence"
```

---

## Task 2: Create `RenderingControls` component

**Files:**
- Create: `dashboard/src/components/graph/RenderingControls.tsx`

- [ ] **Step 1: Write the component**

```tsx
/**
 * RenderingControls — collapsible sidebar section for live render tuning.
 *
 * Mirrors the SimulationControls / NodeSizingControls UX:
 *   - Section header with chevron (click to collapse/expand)
 *   - Slider rows with live numeric value badge
 *   - Boolean toggles for scalePointsOnZoom + showLinks
 *   - "Reset to defaults" button (disabled when nothing changed)
 *   - Active-config indicator dot in header
 *
 * All changes propagate immediately via the setParam callback — debounce is
 * handled by the parent (GraphCanvas useEffect).
 */
import { useState } from 'react'
import { ChevronDown, ChevronRight, RotateCcw } from 'lucide-react'
import type { RenderConfig } from '@/hooks/graph/useRenderConfig'

// ---------------------------------------------------------------------------
// Slider metadata
// ---------------------------------------------------------------------------

interface SliderRow {
  key:   keyof RenderConfig
  label: string
  min:   number
  max:   number
  step:  number
}

const SLIDERS: SliderRow[] = [
  { key: 'pointOpacity',   label: 'Point opacity',    min: 0.05, max: 1.0,  step: 0.01 },
  { key: 'pointSizeScale', label: 'Point size scale', min: 0.05, max: 2.0,  step: 0.01 },
  { key: 'maxPointSize',   label: 'Max point size',   min: 4,    max: 200,  step: 1    },
  { key: 'linkOpacity',    label: 'Link opacity',     min: 0,    max: 1.0,  step: 0.01 },
  { key: 'linkWidthScale', label: 'Link width scale', min: 0.05, max: 2.0,  step: 0.01 },
]

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface RenderingControlsProps {
  config:       RenderConfig
  isModified:   boolean
  setParam:     <K extends keyof RenderConfig>(key: K, value: RenderConfig[K]) => void
  onReset:      () => void
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function RenderingControls({
  config,
  isModified,
  setParam,
  onReset,
}: RenderingControlsProps) {
  const [open, setOpen] = useState(false)

  return (
    <div data-testid="rendering-controls">
      {/* Section header */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="render-controls-body"
        className={[
          'flex items-center justify-between w-full px-2 py-1 rounded',
          'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-400',
          'hover:bg-slate-200/40 dark:hover:bg-slate-800/40 transition-colors',
        ].join(' ')}
        data-testid="rendering-controls-toggle"
      >
        <span className="flex items-center gap-1.5">
          <span className="text-[10px] uppercase tracking-wider text-slate-500 dark:text-slate-600 font-semibold">
            Rendering
          </span>
          {isModified && (
            <span
              className="w-1.5 h-1.5 rounded-full bg-sky-400 shrink-0"
              title="Custom rendering active"
              aria-label="Custom rendering active"
            />
          )}
        </span>
        {open
          ? <ChevronDown className="w-3 h-3 text-slate-400" aria-hidden />
          : <ChevronRight className="w-3 h-3 text-slate-400" aria-hidden />
        }
      </button>

      {open && (
        <div
          id="render-controls-body"
          className="flex flex-col gap-2 mt-1 px-1"
          data-testid="render-controls-body"
        >
          {/* Sliders */}
          {SLIDERS.map(({ key, label, min, max, step }) => {
            const val = config[key] as number
            const displayVal = Number.isInteger(step) ? String(val) : val.toFixed(2)
            return (
              <div key={key} className="flex flex-col gap-0.5">
                <div className="flex items-center justify-between px-1">
                  <label
                    htmlFor={`render-slider-${key}`}
                    className="text-[10px] text-slate-500 dark:text-slate-500"
                  >
                    {label}
                  </label>
                  <span
                    className="text-[10px] font-mono text-slate-400 dark:text-slate-400 tabular-nums"
                    aria-live="polite"
                    aria-label={`${label} value: ${displayVal}`}
                  >
                    {displayVal}
                  </span>
                </div>
                <input
                  id={`render-slider-${key}`}
                  type="range"
                  min={min}
                  max={max}
                  step={step}
                  value={val}
                  onChange={(e) => setParam(key as keyof RenderConfig, Number(e.target.value) as RenderConfig[keyof RenderConfig])}
                  className={[
                    'w-full h-1 rounded-full appearance-none cursor-pointer',
                    'bg-slate-300 dark:bg-slate-700',
                    'accent-sky-500',
                  ].join(' ')}
                  aria-label={label}
                  aria-valuemin={min}
                  aria-valuemax={max}
                  aria-valuenow={val}
                  data-testid={`render-slider-${key}`}
                />
              </div>
            )
          })}

          {/* Boolean toggles */}
          <div className="flex flex-col gap-1 mt-0.5">
            {/* Scale points on zoom */}
            <div className="flex items-center justify-between px-1">
              <span className="text-[10px] text-slate-500 dark:text-slate-500">
                Scale on zoom
              </span>
              <button
                type="button"
                role="switch"
                aria-checked={config.scalePointsOnZoom}
                onClick={() => setParam('scalePointsOnZoom', !config.scalePointsOnZoom)}
                data-testid="render-toggle-scalePointsOnZoom"
                className={[
                  'relative inline-flex h-4 w-7 items-center rounded-full transition-colors',
                  'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-400',
                  config.scalePointsOnZoom
                    ? 'bg-sky-500'
                    : 'bg-slate-600',
                ].join(' ')}
                aria-label="Toggle scale points on zoom"
              >
                <span
                  className={[
                    'inline-block h-3 w-3 rounded-full bg-white shadow transition-transform',
                    config.scalePointsOnZoom ? 'translate-x-3.5' : 'translate-x-0.5',
                  ].join(' ')}
                />
              </button>
            </div>

            {/* Show links */}
            <div className="flex items-center justify-between px-1">
              <span className="text-[10px] text-slate-500 dark:text-slate-500">
                Show links
              </span>
              <button
                type="button"
                role="switch"
                aria-checked={config.showLinks}
                onClick={() => setParam('showLinks', !config.showLinks)}
                data-testid="render-toggle-showLinks"
                className={[
                  'relative inline-flex h-4 w-7 items-center rounded-full transition-colors',
                  'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-400',
                  config.showLinks
                    ? 'bg-sky-500'
                    : 'bg-slate-600',
                ].join(' ')}
                aria-label="Toggle show links"
              >
                <span
                  className={[
                    'inline-block h-3 w-3 rounded-full bg-white shadow transition-transform',
                    config.showLinks ? 'translate-x-3.5' : 'translate-x-0.5',
                  ].join(' ')}
                />
              </button>
            </div>
          </div>

          {/* Reset button */}
          <button
            type="button"
            onClick={onReset}
            disabled={!isModified}
            className={[
              'mt-0.5 flex items-center justify-center gap-1 w-full px-2 py-1 rounded text-[10px] border transition-colors',
              'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sky-400',
              !isModified
                ? 'opacity-40 cursor-default border-slate-700 text-slate-500'
                : 'border-slate-600 text-slate-400 hover:border-sky-600 hover:text-sky-400',
            ].join(' ')}
            aria-label="Reset rendering to defaults"
            data-testid="render-reset-btn"
          >
            <RotateCcw className="w-2.5 h-2.5" />
            Reset to defaults
          </button>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: TypeScript check**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel/dashboard
npx tsc --noEmit --project tsconfig.json 2>&1 | head -30
```

Expected: No errors related to `RenderingControls.tsx`.

- [ ] **Step 3: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel
git add dashboard/src/components/graph/RenderingControls.tsx
git commit -m "feat(graph): add RenderingControls sidebar component — 7 live knobs with sliders + toggles"
```

---

## Task 3: Wire `renderConfig` into `GraphCanvas`

**Files:**
- Modify: `dashboard/src/components/graph/GraphCanvas.tsx`

The key changes:
1. Add `renderConfig?: RenderConfig` to `GraphCanvasProps`.
2. Import `RenderConfig` + `DEFAULT_RENDER_CONFIG` from `useRenderConfig`.
3. Remove hardcoded `pointOpacity`, `pointSizeScale`, `scalePointsOnZoom`, `linkGreyoutOpacity`, `linkWidthScale` from the `new Graph(...)` constructor call and replace with values derived from `renderConfig` (or defaults).
4. Add a `useEffect` that calls `g.setConfig({...})` when `renderConfig` changes — debounced at 16 ms to avoid per-frame spam on rapid slider drags.
5. Update `packLinkColors` to accept `linkOpacity` from config for state-0 (same-repo) alpha.
6. Update `packLinkWidths` to apply `linkWidthScale` from config and set width to 0 when `showLinks === false`.

- [ ] **Step 1: Add the import at the top of `GraphCanvas.tsx`**

In `GraphCanvas.tsx`, after the existing imports, add:

```typescript
import type { RenderConfig } from '@/hooks/graph/useRenderConfig'
import { DEFAULT_RENDER_CONFIG } from '@/hooks/graph/useRenderConfig'
```

- [ ] **Step 2: Add `renderConfig` to `GraphCanvasProps` interface**

Locate the `GraphCanvasProps` interface (around line 155) and add:

```typescript
  renderConfig?: RenderConfig
```

after `nodeSizingConfig?: NodeSizingConfig`.

- [ ] **Step 3: Destructure `renderConfig` in `GraphCanvasInner`**

In the function parameter destructuring (around line 194–217), add:

```typescript
  renderConfig,
```

after `nodeSizingConfig,`.

- [ ] **Step 4: Add `effectiveRenderConfig` derived value**

After the `simCfg` useMemo (around line 219), add:

```typescript
  // #1380: merge tunable render params with defaults so nothing changes if
  // renderConfig is not supplied (maintains backward compat with all callers).
  const rc = renderConfig ? { ...DEFAULT_RENDER_CONFIG, ...renderConfig } : DEFAULT_RENDER_CONFIG
```

- [ ] **Step 5: Update `new Graph(...)` constructor to use `rc`**

Find these hardcoded lines in the constructor (around lines 592–609):

```typescript
      scalePointsOnZoom: true,
      pointSizeScale: 0.22,
      pointOpacity: 0.25,
      pointGreyoutOpacity: (repoFilterActive || !!nodeFilterIndices) ? 0 : 0.18,
      linkGreyoutOpacity: repoFilterActive ? 0 : 0.05,
      linkWidthScale: 0.16,
```

Replace them with:

```typescript
      scalePointsOnZoom: rc.scalePointsOnZoom,
      pointSizeScale: rc.pointSizeScale,
      pointOpacity: rc.pointOpacity,
      pointGreyoutOpacity: (repoFilterActive || !!nodeFilterIndices) ? 0 : 0.18,
      linkGreyoutOpacity: repoFilterActive ? 0 : rc.linkOpacity * 0.5,
      linkWidthScale: rc.showLinks ? rc.linkWidthScale : 0,
```

- [ ] **Step 6: Update `packLinkColors` to use `rc.linkOpacity` for same-repo edges**

The `packLinkColors` callback currently hardcodes `0.15` alpha for state-0 links. It must read from `rc`. Since `packLinkColors` is a `useCallback`, add `rc` to its dependency array or capture via a ref.

Replace the existing `packLinkColors` useCallback with:

```typescript
  const packLinkColors = useCallback((): Float32Array => {
    const { states } = linkData
    const out = new Float32Array(states.length * 4)
    for (let i = 0; i < states.length; i++) {
      let rgba: RGBA
      if (states[i] === 2) {
        rgba = highContrast ? [251, 146, 60, 1.0] : [251, 146, 60, 0.85] // amber — highlighted
      } else if (states[i] === 1) {
        rgba = highContrast ? [56, 189, 248, 1.0] : [56, 189, 248, 0.7]  // sky — cross-repo
      } else {
        // same-repo: use live linkOpacity knob (was hardcoded 0.15)
        const alpha = highContrast ? Math.min(1, rc.linkOpacity * 2) : rc.linkOpacity
        rgba = [100, 116, 139, alpha] // slate
      }
      out[i * 4]     = rgba[0]
      out[i * 4 + 1] = rgba[1]
      out[i * 4 + 2] = rgba[2]
      out[i * 4 + 3] = rgba[3]
    }
    return out
  }, [linkData, highContrast, rc])
```

- [ ] **Step 7: Update `packLinkWidths` to respect `showLinks` + `linkWidthScale`**

Replace existing `packLinkWidths` with:

```typescript
  const packLinkWidths = useCallback((): Float32Array => {
    const { states } = linkData
    const out = new Float32Array(states.length)
    if (!rc.showLinks) return out // all zeros → hidden
    const base = highContrast ? 1.5 : 1.0
    for (let i = 0; i < states.length; i++) {
      out[i] = (states[i] === 0 ? base * 0.6 : base) * rc.linkWidthScale
    }
    return out
  }, [linkData, highContrast, rc])
```

- [ ] **Step 8: Add a debounced `useEffect` for live `setConfig` on render config changes**

Add a new `useEffect` after the existing config-update effect (around line 779):

```typescript
  // #1380: Live render config — apply immediately via setConfig (no re-init).
  // Debounced at 16 ms so rapid slider drags don't spam per-frame setConfig calls.
  const renderDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => {
    if (renderDebounceRef.current) clearTimeout(renderDebounceRef.current)
    renderDebounceRef.current = setTimeout(() => {
      const g = graphRef.current
      if (!g) return
      g.setConfig({
        pointOpacity: rc.pointOpacity,
        pointSizeScale: rc.pointSizeScale,
        scalePointsOnZoom: rc.scalePointsOnZoom,
        linkWidthScale: rc.showLinks ? rc.linkWidthScale : 0,
      })
      // Re-push link colors/widths so link opacity + hide/show takes effect
      g.setLinkColors(packLinkColors())
      g.setLinkWidths(packLinkWidths())
      g.render()
    }, 16)
    return () => {
      if (renderDebounceRef.current) clearTimeout(renderDebounceRef.current)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rc.pointOpacity, rc.pointSizeScale, rc.scalePointsOnZoom, rc.linkWidthScale, rc.showLinks, rc.linkOpacity])
```

Note: `maxPointSize` is enforced indirectly — when the user reduces `pointSizeScale`, large nodes become smaller. If a direct hard clamp is needed at the cosmos.gl level (cosmos 2.6.4 has no `maxPointSize` config key), document it in a comment and link to the EPIC. The `maxPointSize` knob therefore interacts with `pointSizeScale` in the UI description only; the slider still applies as `pointSizeScale` is the actual cosmos param.

**Implementation note for maxPointSize:** Since cosmos.gl 2.6.4 has no `maxPointSize` option, implement the clamp by adjusting `pointSizeScale` when `maxPointSize` changes. In the `useEffect` above, compute the effective scale:

```typescript
      // maxPointSize clamp: scale that would render a tier-5 node (baseSize*3.0) at maxPointSize
      // effective_px = baseSize * multiplier * pointSizeScale * zoomLevel
      // At zoom=1: effectiveScale = maxPointSize / (baseSize * maxMultiplier)
      // This prevents giant blobs at moderate zoom.
      const BASE = 120
      const MAX_MULT = 3.0
      const clampedScale = Math.min(rc.pointSizeScale, rc.maxPointSize / (BASE * MAX_MULT))
      g.setConfig({
        pointOpacity: rc.pointOpacity,
        pointSizeScale: clampedScale,
        scalePointsOnZoom: rc.scalePointsOnZoom,
        linkWidthScale: rc.showLinks ? rc.linkWidthScale : 0,
      })
```

Also add `rc.maxPointSize` to the dependency array.

- [ ] **Step 9: TypeScript check**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel/dashboard
npx tsc --noEmit --project tsconfig.json 2>&1 | head -40
```

Expected: No errors. If there are type errors in `packLinkColors` or `packLinkWidths` due to `rc` captured from outer scope, ensure `rc` is not captured via closure in a stale way — use the same pattern as `repoFilterActive` (a stable derived value, not a ref).

- [ ] **Step 10: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel
git add dashboard/src/components/graph/GraphCanvas.tsx
git commit -m "feat(graph): wire renderConfig prop into GraphCanvas — live setConfig + debounced link repacking"
```

---

## Task 4: Wire up in `graph.tsx` (route)

**Files:**
- Modify: `dashboard/src/routes/graph.tsx`

- [ ] **Step 1: Add imports**

At the top of `graph.tsx`, add:

```typescript
import { useRenderConfig } from '@/hooks/graph/useRenderConfig'
import { RenderingControls } from '@/components/graph/RenderingControls'
```

- [ ] **Step 2: Instantiate the hook**

In the hooks section (around line 93–98, after `useNodeSizingConfig`), add:

```typescript
  // ── Render config (#1380 — live rendering knobs persisted to localStorage) ──
  const {
    config: renderConfig,
    setParam: setRenderParam,
    resetToDefaults: resetRenderConfig,
    isModified: renderConfigModified,
  } = useRenderConfig()
```

- [ ] **Step 3: Pass `renderConfig` to `<GraphCanvas>`**

Find the `<GraphCanvas>` JSX call in the render section (search for `<GraphCanvas`). It already has `nodeSizingConfig={nodeSizingConfig}`. Add after it:

```tsx
              renderConfig={renderConfig}
```

- [ ] **Step 4: Add `<RenderingControls>` panel to the sidebar**

In the sidebar `<aside>` (around line 651–658 after `<NodeSizingControls>`), add a divider and the new panel:

```tsx
          <div className="border-t border-slate-200 dark:border-slate-700" />

          {/* Rendering controls (#1380) */}
          <RenderingControls
            config={renderConfig}
            isModified={renderConfigModified}
            setParam={setRenderParam}
            onReset={resetRenderConfig}
          />
```

- [ ] **Step 5: TypeScript check**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel/dashboard
npx tsc --noEmit --project tsconfig.json 2>&1 | head -40
```

Expected: Zero errors.

- [ ] **Step 6: Commit**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel
git add dashboard/src/routes/graph.tsx
git commit -m "feat(graph): wire RenderingControls into graph route sidebar"
```

---

## Task 5: Build + Playwright verification

**Files:**
- No new files — verification task only.

- [ ] **Step 1: Run the dashboard build**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel
make build 2>&1 | tail -30
```

Expected: Build succeeds with zero errors. Warnings about unused vars etc. are OK.

- [ ] **Step 2: Start the dev server on an ISOLATED port**

Start on port 5180 (isolated, far from default 5173, never used by the shared daemon):

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel/dashboard
VITE_API_BASE=http://localhost:47274 npx vite --port 5180 --strictPort &
echo "Vite PID: $!"
```

Wait ~5s for Vite to start, then confirm it's up:

```bash
curl -sf http://localhost:5180/ | head -5
```

Expected: Returns HTML.

- [ ] **Step 3: Playwright — load /graph/upvate and confirm Rendering panel**

Use the Playwright MCP tools (loaded via ToolSearch "playwright") to:

1. Navigate to `http://localhost:5180/upvate/graph`
2. Wait for the canvas to load (wait for selector `[data-testid="rendering-controls"]`).
3. Click `[data-testid="rendering-controls-toggle"]` to expand the panel.
4. Assert `[data-testid="render-controls-body"]` is visible.
5. Take screenshot as `render-panel.png` in the worktree root.

```javascript
// Pseudocode — use actual Playwright MCP calls
await navigate('http://localhost:5180/upvate/graph')
await waitForSelector('[data-testid="rendering-controls"]', { timeout: 15000 })
await click('[data-testid="rendering-controls-toggle"]')
await waitForSelector('[data-testid="render-controls-body"]')
await screenshot({ path: 'render-panel.png' })
```

- [ ] **Step 4: Playwright — drag Link opacity up and confirm edges appear**

The default `linkOpacity` is 0.15 (near invisible). Drag the slider to 0.7 and confirm edges visually appear.

```javascript
// Find the linkOpacity slider and set its value to 0.7
await evaluate(() => {
  const slider = document.querySelector('[data-testid="render-slider-linkOpacity"]')
  // Trigger a React synthetic change event
  const nativeInputValueSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set
  nativeInputValueSetter.call(slider, '0.7')
  slider.dispatchEvent(new Event('input', { bubbles: true }))
  slider.dispatchEvent(new Event('change', { bubbles: true }))
})
await waitForTimeout(500) // allow debounce to fire
await screenshot({ path: 'render-links-visible.png' })
```

- [ ] **Step 5: Playwright — toggle Scale on zoom OFF and confirm nodes don't balloon**

```javascript
// Toggle scalePointsOnZoom OFF
await click('[data-testid="render-toggle-scalePointsOnZoom"]')
await waitForTimeout(300)
// Zoom in (scroll up on canvas)
await scroll('canvas', { deltaY: -500 })
await waitForTimeout(500)
await screenshot({ path: 'render-scale-zoom-off.png' })
```

- [ ] **Step 6: Playwright — check console for errors**

```javascript
const logs = await consoleMessages()
const errors = logs.filter(m => m.type === 'error')
assert(errors.length === 0, `Console errors: ${errors.map(e => e.text).join('\n')}`)
```

- [ ] **Step 7: Kill the dev server + commit screenshots**

```bash
# Kill the vite process started above
kill $(lsof -ti:5180) 2>/dev/null || true

cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel
git add render-panel.png render-links-visible.png render-scale-zoom-off.png 2>/dev/null || true
git commit -m "test(graph): Playwright verification screenshots for rendering panel" 2>/dev/null || echo "No screenshots to commit"
```

---

## Task 6: Open PR

- [ ] **Step 1: Push branch**

```bash
cd /Users/jorgecajas/Documents/Projects/archigraph-worktrees/feat-render-panel
git push -u origin feat/graph-render-tuning-panel
```

- [ ] **Step 2: Create PR**

```bash
gh pr create \
  --title "feat(graph): live Rendering panel — point/link opacity, size scale, scale-on-zoom, max size" \
  --body "$(cat <<'EOF'
## What

Adds a live **Rendering** tuning panel to the graph sidebar (collapsible, matches Node Sizing + Simulation UX). Refs #1380.

Seven knobs are now dial-able in real time without restarting the simulation:

| Knob | cosmos param | Default | Range |
|------|-------------|---------|-------|
| Point opacity | `pointOpacity` | 0.25 | 0.05–1.0 |
| Point size scale | `pointSizeScale` | 0.22 | 0.05–2.0 |
| Scale on zoom | `scalePointsOnZoom` | ON | toggle |
| Max point size | via `pointSizeScale` clamp | 150 px | 4–200 |
| Link opacity | link color alpha (state 0) | 0.15 | 0–1.0 |
| Link width scale | `linkWidthScale` | 0.16 | 0.05–2.0 |
| Show links | `linkWidthScale` → 0 | ON | toggle |

## Why

Hardcoded render params caused whack-a-mole visual regressions ("nodes too big on zoom", "edges near invisible", "relationships not visible at all"). The owner can now dial the look live and persist it to localStorage.

## How

- **`useRenderConfig`** — same pattern as `useSimulationConfig`: defaults = current hardcoded values (no visible change on first load), localStorage under `archigraph:graph:rendering`.
- **`RenderingControls`** — collapsible sidebar section, mirrors SimulationControls/NodeSizingControls UX exactly: sliders with live numeric badge, toggle switches, "Reset to defaults" button, active-indicator dot.
- **`GraphCanvas`** — new `renderConfig?: RenderConfig` prop. Hardcoded `pointOpacity`, `pointSizeScale`, `scalePointsOnZoom`, `linkGreyoutOpacity`, `linkWidthScale` replaced with `rc.*` derived from the prop. A debounced (16 ms) `useEffect` calls `g.setConfig({...})` + repacks link buffers on change — **no simulation restart, no Graph recreation**.
- **`graph.tsx`** — instantiates hook, renders panel in sidebar after Node Sizing section, passes `renderConfig` to `<GraphCanvas>`.
- **max-point-size clamp** — cosmos 2.6.4 has no direct `maxPointSize` config; the knob clamps `pointSizeScale` to `maxPointSize / (baseSize * maxMultiplier)` so tier-5 hub nodes never exceed the configured px limit at zoom=1.

## Test plan

- [ ] `make build` passes with zero TypeScript errors
- [ ] Navigate `/upvate/graph` — Rendering panel visible in sidebar, collapsed by default
- [ ] Expand panel — all 5 sliders + 2 toggles render; each slider shows live numeric value
- [ ] Drag **Link opacity** from 0.15 → 0.7 — edges become visibly brighter within 50 ms (verified in `render-links-visible.png`)
- [ ] Toggle **Scale on zoom** OFF — zooming in no longer causes nodes to balloon (`render-scale-zoom-off.png`)
- [ ] Drag **Point opacity** — node density/brightness changes live
- [ ] Toggle **Show links** OFF — all edges disappear; toggle back ON — edges return
- [ ] **Reset to defaults** button restores all knobs and localStorage
- [ ] Refresh page — persisted values reload correctly from localStorage
- [ ] Node Sizing + Simulation panels still work; all 3 color modes intact
- [ ] Zero console errors

Refs #1380
EOF
)"
```

---

## Self-Review

### Spec coverage check

| Requirement | Task |
|-------------|------|
| `useRenderConfig` hook + localStorage `archigraph:graph:rendering` | Task 1 |
| Point opacity slider 0.05–1.0, default 0.25 | Tasks 1+2 |
| Point size scale 0.05–2.0, default 0.22 | Tasks 1+2 |
| Scale on zoom toggle, default ON | Tasks 1+2 |
| Max point size via clamp, cosmos has no direct param | Tasks 1+3 (documented) |
| Link opacity slider 0–1, default 0.15 | Tasks 1+2+3 |
| Link width scale 0.05–2.0, default 0.16 | Tasks 1+2 |
| Show links toggle | Tasks 1+2+3 |
| Live value display per slider | Task 2 |
| Immediate apply via `setConfig()` no recreation | Task 3 |
| Debounce on rapid drag | Task 3 step 8 |
| Re-pack link colors/widths on change | Task 3 steps 6–7 |
| Reset to defaults + active indicator dot | Tasks 1+2 |
| localStorage key `archigraph:graph:rendering` | Task 1 |
| Replace hardcoded values in `new Graph(...)` | Task 3 steps 3–5 |
| Keep 3 color modes working | Covered — no color logic changed |
| Keep Node Sizing + Simulation panels working | Covered — additive change only |
| `make build` | Task 5 step 1 |
| Playwright verify on /graph/upvate | Task 5 steps 3–6 |
| Screenshots `render-panel.png` + `render-links-visible.png` | Task 5 steps 3–4 |
| PR to main, 6-section format, `Refs #1380` | Task 6 |
| No backend changes | All tasks — no Go files touched |

### Placeholder scan

No TBDs, no "implement later", no "add error handling" voids. All code blocks are complete.

### Type consistency

- `RenderConfig` defined in `useRenderConfig.ts`, imported by both `RenderingControls.tsx` and `GraphCanvas.tsx`.
- `DEFAULT_RENDER_CONFIG` exported from `useRenderConfig.ts`, imported by `GraphCanvas.tsx`.
- `setParam<K extends keyof RenderConfig>(key: K, value: RenderConfig[K])` — generic signature used consistently in hook + component props.
- `packLinkColors` / `packLinkWidths` — `rc` captured from outer scope (derived value, not a ref) so React's exhaustive-deps rule is satisfied.
