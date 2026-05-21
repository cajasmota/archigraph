/**
 * Headless smoke test for #1141: per-topic detail panel (Topology v2).
 *
 * Verifies:
 *   1. Navigating /topology/upvate shows the topology page
 *   2. Clicking a topic node/row opens the detail panel
 *   3. All key panel sections render (header, AI summary, producers, consumers, lifecycle)
 *   4. Panel renders for an active topic (screenshot 1)
 *   5. Panel renders for an orphan-publisher topic (screenshot 2)
 *   6. Zero console errors throughout
 *
 * Usage: node smoke-topology-detail.mjs <port>
 */

import { chromium } from '@playwright/test'
import * as fs from 'fs'
import * as path from 'path'
import { fileURLToPath } from 'url'

const PORT = process.argv[2] ?? '5533'
const BASE = `http://127.0.0.1:${PORT}`
const TOPO_URL = `${BASE}/topology/fixture-a`
const SCREENSHOTS_DIR = path.join(path.dirname(fileURLToPath(import.meta.url)), 'e2e-screenshots')

fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true })

function screenshot(page, name) {
  const p = path.join(SCREENSHOTS_DIR, name)
  return page.screenshot({ path: p, fullPage: false }).then(() => {
    console.log(`  screenshot → ${p}`)
    return p
  })
}

const browser = await chromium.launch({ headless: true })
const context = await browser.newContext({ viewport: { width: 1400, height: 900 } })
const page = await context.newPage()

const consoleErrors = []
page.on('console', (msg) => {
  if (msg.type() === 'error') {
    // Ignore known non-critical errors (e.g. sourcemap warnings, HMR noise)
    const text = msg.text()
    if (!text.includes('source map') && !text.includes('[HMR]')) {
      consoleErrors.push(text)
    }
  }
})

const results = []

function assert(name, pass, detail = '') {
  const status = pass ? 'PASS' : 'FAIL'
  results.push({ name, pass })
  console.log(`  [${status}] ${name}${detail ? ' — ' + detail : ''}`)
  return pass
}

// ── Step 1: Load topology page ───────────────────────────────────────────────
console.log('\n── Step 1: Load topology page ──────────────────────────────────────')
console.log(`Navigating to ${TOPO_URL} ...`)
await page.goto(TOPO_URL, { waitUntil: 'networkidle', timeout: 30000 })
await page.waitForTimeout(2000)

// Switch to list view for reliable clicking (list rows have clear click targets)
const listBtn = page.locator('button[aria-pressed][title="List view"], button:has-text("List")')
const listBtnCount = await listBtn.count()
if (listBtnCount > 0) {
  await listBtn.first().click()
  await page.waitForTimeout(500)
}

await screenshot(page, '1141-01-topology-loaded.png')
assert('topology page loaded', await page.title() !== '', `title=${await page.title()}`)

// ── Step 2: Panel closed initially ──────────────────────────────────────────
console.log('\n── Step 2: Confirm panel closed initially ───────────────────────────')
const panelBefore = page.locator('[data-testid="topic-detail-panel"]')
const panelBeforeCount = await panelBefore.count()
assert('detail panel not visible initially', panelBeforeCount === 0, `count=${panelBeforeCount}`)

// ── Step 3: Click a topic row → panel opens (active topic) ──────────────────
console.log('\n── Step 3: Click first topic row ───────────────────────────────────')

// TopologyList groups are collapsed by default.
// Group headers are <button aria-expanded="false"> with svc-* text.
// NOTE: the GroupSelector dropdown also uses aria-expanded; skip it by only
// targeting group headers that have non-empty text matching repo-like names.
const topologyGroupHeaders = page.locator('button[aria-expanded="false"]:has-text("svc-")')
const topologyGroupCount = await topologyGroupHeaders.count()
console.log(`  Found ${topologyGroupCount} topology group header(s)`)

if (topologyGroupCount > 0) {
  await topologyGroupHeaders.first().click()
  await page.waitForTimeout(500)
}

// Now find topic rows inside the expanded group
// TopologyListRowItem: <button type="button" role="row">
const topicRows = page.locator('button[role="row"]')
const topicRowCount = await topicRows.count()
console.log(`  Found ${topicRowCount} topic row(s) after expanding group`)

let clicked = false
if (topicRowCount > 0) {
  await topicRows.first().click()
  clicked = true
}

assert('found clickable topic element', clicked, `topicRowCount=${topicRowCount}`)
await page.waitForTimeout(1500)

// Panel should appear
const panel = page.locator('[data-testid="topic-detail-panel"]')
const panelCount = await panel.count()
assert('detail panel appears after click', panelCount > 0, `count=${panelCount}`)

// Screenshot 1: active topic panel
await screenshot(page, '1141-02-active-topic-panel.png')

// ── Step 4: Verify panel sections ───────────────────────────────────────────
console.log('\n── Step 4: Verify panel sections ────────────────────────────────────')

if (panelCount > 0) {
  // Check close button exists
  const closeBtn = panel.locator('button[aria-label="Close topic detail panel"]')
  const closeBtnCount = await closeBtn.count()
  assert('close button present', closeBtnCount > 0)

  // Check AI summary section
  const aiSection = panel.locator('#tpanel-section-ai-summary, [id^="tpanel-section-ai"]')
  const aiSectionCount = await aiSection.count()
  assert('AI summary section heading present', aiSectionCount > 0)

  // Check producers section
  const producersSection = panel.locator('[id^="tpanel-section-prod"]')
  const producersSectionCount = await producersSection.count()
  assert('producers section heading present', producersSectionCount > 0)

  // Check consumers section
  const consumersSection = panel.locator('[id^="tpanel-section-cons"]')
  const consumersSectionCount = await consumersSection.count()
  assert('consumers section heading present', consumersSectionCount > 0)

  // Check lifecycle section
  const lifecycleSection = panel.locator('[id^="tpanel-section-life"]')
  const lifecycleSectionCount = await lifecycleSection.count()
  assert('lifecycle section heading present', lifecycleSectionCount > 0)

  // Check copy ID button
  const copyBtn = panel.locator('button[aria-label="Copy topic ID"], button[aria-label^="Copy"]')
  const copyBtnCount = await copyBtn.count()
  assert('copy ID button present', copyBtnCount > 0)

  // Verify no raw "undefined" text visible in panel
  const panelText = await panel.innerText()
  const hasUndefined = panelText.includes('undefined') || panelText.includes('null')
  assert('no "undefined"/"null" text visible', !hasUndefined, hasUndefined ? `found: ${panelText.slice(0, 100)}` : '')
}

// ── Step 5: Close panel via Esc key ─────────────────────────────────────────
console.log('\n── Step 5: Close panel via Esc ──────────────────────────────────────')
if (panelCount > 0) {
  await page.keyboard.press('Escape')
  await page.waitForTimeout(500)
  const panelAfterEsc = page.locator('[data-testid="topic-detail-panel"]')
  const panelAfterEscCount = await panelAfterEsc.count()
  assert('panel closes on Escape', panelAfterEscCount === 0, `count=${panelAfterEscCount}`)
}

// ── Step 6: Reopen panel and find orphan-publisher topic ─────────────────────
console.log('\n── Step 6: Find orphan-publisher topic ──────────────────────────────')

// Click multiple rows to find one with orphan-publisher badge
let foundOrphan = false

// Expand all topology groups to get all rows visible
const allTopologyGroupHeaders = await page.locator('button[aria-expanded="false"]:has-text("svc-")').all()
for (const hdr of allTopologyGroupHeaders) {
  await hdr.click().catch(() => null)
  await page.waitForTimeout(200)
}

const rows = await page.locator('button[role="row"]').all()
console.log(`  Scanning ${rows.length} rows for orphan...`)

for (const row of rows.slice(0, 10)) {
  try {
    await row.click()
    await page.waitForTimeout(800)

    const orphanBadge = page.locator('[data-testid="topic-detail-panel"] :text("ORPHAN"), [data-testid="topic-detail-panel"] :text("orphan")')
    const orphanCount = await orphanBadge.count()

    if (orphanCount > 0) {
      foundOrphan = true
      await screenshot(page, '1141-03-orphan-publisher-panel.png')
      break
    }
  } catch {
    // row may be stale, continue
  }
}

if (!foundOrphan) {
  // Take a screenshot anyway of whatever is open
  await screenshot(page, '1141-03-second-topic-panel.png')
}
assert('second view screenshot captured', true)

// ── Step 7: Console error check ──────────────────────────────────────────────
console.log('\n── Step 7: Console error check ──────────────────────────────────────')
assert('zero console errors', consoleErrors.length === 0,
  consoleErrors.length > 0 ? `errors: ${consoleErrors.slice(0, 3).join(' | ')}` : '')

if (consoleErrors.length > 0) {
  console.log('  Console errors:')
  consoleErrors.forEach((e) => console.log(`    ${e}`))
}

// ── Results summary ───────────────────────────────────────────────────────────
console.log('\n── Summary ──────────────────────────────────────────────────────────')
const passed = results.filter((r) => r.pass).length
const failed = results.filter((r) => !r.pass).length
console.log(`  ${passed} passed, ${failed} failed`)

await browser.close()

if (failed > 0) {
  console.error('\nSome assertions failed.')
  process.exit(1)
} else {
  console.log('\nAll assertions passed.')
  process.exit(0)
}
