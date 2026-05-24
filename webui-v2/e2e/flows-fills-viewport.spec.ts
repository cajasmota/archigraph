// webui-v2/e2e/flows-fills-viewport.spec.ts
//
// Regression guard for #2115 — Flows diagram empty whitespace below.
//
// Asserts that after selecting a flow the DAG canvas SVG/canvas area
// covers > 65% of the viewport height below the topbar.  Past fixes
// kept regressing because the FlowDag viewport had a hard pixel-height
// cap (flex-none + height: 400px) instead of participating in the flex
// layout.  This test catches that regression.
import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// ─── Shared mock flow ────────────────────────────────────────────────────────

const FILL_FLOW = {
  process_id: "proc-fill-2115",
  label: "AuthController.login → SessionRepository.create",
  repo: "client-fixture-a",
  entry_id: "f0",
  entry_name: "AuthController.login",
  entry_kind: "http_handler",
  terminal_id: "f4",
  step_count: 5,
  cross_stack: false,
  is_cross_repo: false,
  chain_labels: ["login", "validateCreds", "createToken", "auditLog", "create"],
  is_dag: false,
  steps: [
    { entity_id: "f0", name: "AuthController.login",            step_index: 0, source_file: "src/auth.ts",    start_line: 10, repo: "client-fixture-a", edge_kind: null,    step_kind: "http_fetch" },
    { entity_id: "f1", name: "AuthService.validateCredentials", step_index: 1, source_file: "src/auth.ts",    start_line: 42, repo: "client-fixture-a", edge_kind: "CALLS", step_kind: "validation" },
    { entity_id: "f2", name: "TokenService.createToken",        step_index: 2, source_file: "src/token.ts",   start_line: 18, repo: "client-fixture-a", edge_kind: "CALLS", step_kind: "transform" },
    { entity_id: "f3", name: "AuditService.log",                step_index: 3, source_file: "src/audit.ts",   start_line: 55, repo: "client-fixture-a", edge_kind: "CALLS", step_kind: "db_write" },
    { entity_id: "f4", name: "SessionRepository.create",        step_index: 4, source_file: "src/session.ts", start_line: 29, repo: "client-fixture-a", edge_kind: "CALLS", step_kind: "db_write" },
  ],
};

async function mockFlowsPage(page: Page) {
  const pid = FILL_FLOW.process_id;

  await page.route("**/api/v2/meta**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ version: "dev", daemon_running: true, groups: ["demo"] }) }));
  await page.route("**/api/v2/groups", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify([{ id: "demo", name: "Demo", repos: ["client-fixture-a"], entityCount: 10, fidelity: 0.95, indexedAt: Date.now(), health: "healthy" }]) }));
  await page.route("**/api/v2/groups/demo**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ id: "demo", name: "Demo", entities: 10, fidelity: 0.95, indexedAt: Date.now(), health: "healthy", features: { watchers: false, gitHooks: false }, docsPath: "", repos: [] }) }));
  await page.route("**/api/flows/demo?**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ processes: [FILL_FLOW], count: 1, entry_kind_groups: [{ kind: "http_handler", count: 1 }] }) }));
  await page.route("**/api/flows/demo", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ processes: [FILL_FLOW], count: 1, entry_kind_groups: [{ kind: "http_handler", count: 1 }] }) }));
  await page.route(`**/api/flows/demo/${encodeURIComponent(pid)}**`, (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ process: FILL_FLOW, chain_entities: FILL_FLOW.steps, source_snippets: {} }) }));
  await page.route(`**/api/flows/demo/${pid}**`, (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ process: FILL_FLOW, chain_entities: FILL_FLOW.steps, source_snippets: {} }) }));
  await page.route("**/api/flows/demo/dead-ends**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ dead_ends: [], count: 0 }) }));
  await page.route("**/api/flows/demo/truncated**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ processes: [], count: 0, entry_kind_groups: [] }) }));
  await page.route("**/api/system**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ status: "running", version: "dev", commit_sha: "abc", built_at: "2026-01-01", stale_build: false, pid: 1, rss_mb: 64 }) }));
  await page.route("**/api/groups/demo**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ id: "demo", name: "Demo", repos: ["client-fixture-a"], entityCount: 10, fidelity: 0.95, indexedAt: Date.now(), health: "healthy" }) }));
}

// ─── Tests ───────────────────────────────────────────────────────────────────

test.describe("#2115 — Flows diagram fills viewport", () => {
  test("DAG canvas covers > 65% of viewport height after flow selection", async ({ page }) => {
    await mockFlowsPage(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/g/demo/flows");

    // Wait for the list to load
    await page.waitForTimeout(800);

    // Click the flow row
    const flowRow = page.locator("button").filter({ hasText: /AuthController|login/ }).first();
    await expect(flowRow).toBeVisible({ timeout: 5000 });
    await flowRow.click();

    // Wait for detail panel to render
    await page.waitForTimeout(800);

    // Screenshot before assertion
    await page.screenshot({
      path: path.join(__dirname, "../../2115-dag-fill-viewport.png"),
      fullPage: false,
    });

    // Find the DAG viewport element — it has the canvas grid background and
    // contains the SVG overlay. We identify it by the data-testid on the
    // parent DetailPanel body area, or by querying the SVG inside the canvas.
    //
    // The fix puts FlowDag viewport as flex-1 min-h-0. After the fix, the
    // element should occupy substantially more than 65% of the visible viewport
    // below the topbar (which is ~42px for the tab strip + ~120px header =
    // roughly 160-180px chrome). At 900px viewport height, 65% = 585px;
    // below the chrome means the DAG area height should be > (900 - 180) * 0.65
    // = 468px minimum.
    //
    // We measure via getBoundingClientRect of the SVG inside the FlowDag.
    const dagSvg = page.locator("svg").first();
    await expect(dagSvg).toBeVisible({ timeout: 5000 });

    const viewportHeight = 900;
    // Account for topbar chrome: tab strip (42px) + detail header (~150px).
    const chromeHeight = 200;
    const availableHeight = viewportHeight - chromeHeight;
    const threshold = availableHeight * 0.65;

    // Get the bounding box of the DAG canvas wrapper (the div with the
    // radial-gradient background). We find it by looking for the container
    // that wraps the SVG and has overflow:hidden and the grab cursor.
    //
    // Alternative: use evaluate to measure how much vertical space the DAG
    // element occupies.
    const dagHeight = await page.evaluate((): number => {
      // Walk all divs to find the one with the canvas grid background.
      const all = Array.from(document.querySelectorAll("div"));
      for (const el of all) {
        const style = window.getComputedStyle(el);
        if (
          style.backgroundImage?.includes("radial-gradient") &&
          style.overflow === "hidden" &&
          style.cursor?.includes("grab")
        ) {
          return el.getBoundingClientRect().height;
        }
      }
      return 0;
    });

    // Expect the DAG to fill at least 65% of the available space below chrome.
    expect(dagHeight).toBeGreaterThan(threshold);
  });
});
