// webui-v2/e2e/flows-repo-colors.spec.ts
//
// Playwright E2E tests for #2116 — per-step repo color + vertical legend.
//
// Three assertions:
//   1. Cross-repo flow → step cards in different repos have distinct
//      left-edge accent-stripe colors (borderLeftColor differs).
//   2. Legend lists repos in a vertical column (flex-col, not flex-row).
//   3. No kind chips in the legend (no "HTTP Fetch", "DB Query", etc. text).
//
// All API responses are mocked so this runs standalone without a daemon.
import { test, expect, type Page } from "@playwright/test";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// ─── Cross-repo mock flow ────────────────────────────────────────────────────
// Two repos so we get distinct accent stripes.

const CROSS_REPO_FLOW = {
  process_id: "proc-xrepo-2116",
  label: "ApiGateway.handleRequest → DataService.persist",
  repo: "client-fixture-b",
  entry_id: "x0",
  entry_name: "ApiGateway.handleRequest",
  entry_kind: "http_handler",
  terminal_id: "x3",
  step_count: 4,
  cross_stack: true,
  is_cross_repo: true,
  chain_labels: ["handleRequest", "parseBody", "fetchUser", "persist"],
  is_dag: false,
  steps: [
    {
      entity_id: "x0",
      name: "ApiGateway.handleRequest",
      step_index: 0,
      source_file: "src/gateway.ts",
      start_line: 10,
      repo: "client-fixture-b",
      edge_kind: null,
      step_kind: "http_fetch",
    },
    {
      entity_id: "x1",
      name: "RequestParser.parseBody",
      step_index: 1,
      source_file: "src/parser.ts",
      start_line: 22,
      repo: "client-fixture-b",
      edge_kind: "CALLS",
      step_kind: "transform",
    },
    {
      entity_id: "x2",
      name: "UserService.fetchUser",
      step_index: 2,
      source_file: "src/user.ts",
      start_line: 31,
      // Different repo — accent stripe should differ from client-fixture-b
      repo: "client-fixture-c",
      edge_kind: "CALLS",
      step_kind: "db_query",
    },
    {
      entity_id: "x3",
      name: "DataService.persist",
      step_index: 3,
      source_file: "src/data.ts",
      start_line: 67,
      repo: "client-fixture-c",
      edge_kind: "CALLS",
      step_kind: "db_write",
    },
  ],
};

async function mockXrepoPage(page: Page) {
  const pid = CROSS_REPO_FLOW.process_id;
  const flowsBody = JSON.stringify({
    processes: [CROSS_REPO_FLOW],
    count: 1,
    entry_kind_groups: [{ kind: "http_handler", count: 1 }],
  });
  const detailBody = JSON.stringify({
    process: CROSS_REPO_FLOW,
    chain_entities: CROSS_REPO_FLOW.steps,
    source_snippets: {},
  });

  await page.route("**/api/v2/meta**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ version: "dev", daemon_running: true, groups: ["demo"] }) }));
  await page.route("**/api/v2/groups", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify([{ id: "demo", name: "Demo", repos: ["client-fixture-b", "client-fixture-c"], entityCount: 20, fidelity: 0.95, indexedAt: Date.now(), health: "healthy" }]) }));
  await page.route("**/api/v2/groups/demo**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ id: "demo", name: "Demo", entities: 20, fidelity: 0.95, indexedAt: Date.now(), health: "healthy", features: { watchers: false, gitHooks: false }, docsPath: "", repos: [] }) }));
  await page.route("**/api/flows/demo?**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json", body: flowsBody }));
  await page.route("**/api/flows/demo", (r) =>
    r.fulfill({ status: 200, contentType: "application/json", body: flowsBody }));
  await page.route(`**/api/flows/demo/${encodeURIComponent(pid)}**`, (r) =>
    r.fulfill({ status: 200, contentType: "application/json", body: detailBody }));
  await page.route(`**/api/flows/demo/${pid}**`, (r) =>
    r.fulfill({ status: 200, contentType: "application/json", body: detailBody }));
  await page.route("**/api/flows/demo/dead-ends**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ dead_ends: [], count: 0 }) }));
  await page.route("**/api/flows/demo/truncated**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ processes: [], count: 0, entry_kind_groups: [] }) }));
  await page.route("**/api/system**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ status: "running", version: "dev", commit_sha: "abc", built_at: "2026-01-01", stale_build: false, pid: 1, rss_mb: 64 }) }));
  await page.route("**/api/groups/demo**", (r) =>
    r.fulfill({ status: 200, contentType: "application/json",
      body: JSON.stringify({ id: "demo", name: "Demo", repos: ["client-fixture-b", "client-fixture-c"], entityCount: 20, fidelity: 0.95, indexedAt: Date.now(), health: "healthy" }) }));
}

// ─── Tests ───────────────────────────────────────────────────────────────────

test.describe("#2116 — per-step repo color + vertical legend", () => {
  test("cross-repo steps have distinct left-edge accent stripe colors", async ({ page }) => {
    await mockXrepoPage(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/g/demo/flows");

    await page.waitForTimeout(800);

    const flowRow = page
      .locator("button")
      .filter({ hasText: /ApiGateway|handleRequest/ })
      .first();
    await expect(flowRow).toBeVisible({ timeout: 5000 });
    await flowRow.click();
    await page.waitForTimeout(1000);

    await page.screenshot({
      path: path.join(__dirname, "../../2116-01-repo-stripe-colors.png"),
      fullPage: false,
    });

    // Collect borderLeftColor values for step nodes across the two repos.
    // Step nodes carry data-repo and data-step-node attributes.
    const stripeColors = await page.evaluate((): { repo: string; color: string }[] => {
      const nodes = Array.from(document.querySelectorAll("button[data-step-node]"));
      return nodes.map((el) => {
        const style = window.getComputedStyle(el as HTMLElement);
        return {
          repo: (el as HTMLElement).dataset.repo ?? "",
          color: style.borderLeftColor,
        };
      });
    });

    // Must have nodes from both repos
    const repoB = stripeColors.filter((n) => n.repo === "client-fixture-b");
    const repoC = stripeColors.filter((n) => n.repo === "client-fixture-c");

    expect(repoB.length).toBeGreaterThan(0);
    expect(repoC.length).toBeGreaterThan(0);

    // The stripe colors for the two repos must differ.
    const colorB = repoB[0].color;
    const colorC = repoC[0].color;
    expect(colorB).not.toBe(colorC);
  });

  test("vertical legend lists repos in a column, not in a horizontal row", async ({ page }) => {
    await mockXrepoPage(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/g/demo/flows");

    await page.waitForTimeout(800);

    const flowRow = page
      .locator("button")
      .filter({ hasText: /ApiGateway|handleRequest/ })
      .first();
    await expect(flowRow).toBeVisible({ timeout: 5000 });
    await flowRow.click();
    await page.waitForTimeout(1000);

    await page.screenshot({
      path: path.join(__dirname, "../../2116-02-vertical-legend.png"),
      fullPage: false,
    });

    // Legend element
    const legend = page.locator("[data-testid='flow-legend']");
    await expect(legend).toBeVisible({ timeout: 5000 });

    // Both repo slugs should appear as legend entries.
    const legendText = await legend.innerText();
    expect(legendText).toContain("client-fixture-b");
    expect(legendText).toContain("client-fixture-c");

    // The legend must be a vertical column — flex-col. Measure bounding rects
    // of the repo legend items: they should be stacked (different Y, same X).
    const repoBItem = legend.locator("[data-repo-legend='client-fixture-b']");
    const repoCItem = legend.locator("[data-repo-legend='client-fixture-c']");
    await expect(repoBItem).toBeVisible({ timeout: 3000 });
    await expect(repoCItem).toBeVisible({ timeout: 3000 });

    const bBox = await repoBItem.boundingBox();
    const cBox = await repoCItem.boundingBox();
    expect(bBox).not.toBeNull();
    expect(cBox).not.toBeNull();

    // Vertical: C row should be BELOW B row (higher Y).
    expect(cBox!.y).toBeGreaterThan(bBox!.y);
    // And they should have approximately the same X (within 10px).
    expect(Math.abs(cBox!.x - bBox!.x)).toBeLessThan(10);
  });

  test("no kind chips (HTTP Fetch, DB Query, etc.) appear in legend", async ({ page }) => {
    await mockXrepoPage(page);
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/g/demo/flows");

    await page.waitForTimeout(800);

    const flowRow = page
      .locator("button")
      .filter({ hasText: /ApiGateway|handleRequest/ })
      .first();
    await expect(flowRow).toBeVisible({ timeout: 5000 });
    await flowRow.click();
    await page.waitForTimeout(1000);

    await page.screenshot({
      path: path.join(__dirname, "../../2116-03-no-kind-chips.png"),
      fullPage: false,
    });

    const legend = page.locator("[data-testid='flow-legend']");
    await expect(legend).toBeVisible({ timeout: 5000 });
    const legendText = await legend.innerText();

    // Kind chip labels that the old legend showed — must NOT be present.
    const kindLabels = ["HTTP Fetch", "DB Query", "DB Write", "Publish", "Function", "Render"];
    for (const label of kindLabels) {
      expect(legendText).not.toContain(label);
    }

    // Cross-repo dashed line indicator should still be present.
    expect(legendText).toContain("cross-repo");
  });
});
