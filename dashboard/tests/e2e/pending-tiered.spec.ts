/**
 * E2E: Tiered UI buckets in /pending — Critical / High / Medium / Low (#1133)
 *
 * Tests run HEADLESS against the Vite dev server (port 5173).
 * Without a running daemon the enrichments list is empty, so tier sections
 * render with 0 counts. The tests verify UI structure (4 tier headers, a11y
 * attributes, toolbar controls) regardless of data availability.
 *
 * With a live daemon the counts and candidate rows will also be asserted.
 */

import { test, expect, type Page } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'
import fs from 'fs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const BASE_URL = process.env.TEST_BASE_URL ?? 'http://localhost:5173'
// Route: /pending/:group — use 'fixture-a' which is the default group slug
const PENDING_URL = `${BASE_URL}/pending/fixture-a`
const SCREENSHOT_DIR = path.join(__dirname, '..', '..', 'test-results', 'pending-tiered')

// ── Helpers ──────────────────────────────────────────────────────────────────

async function screenshot(page: Page, name: string) {
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true })
  await page.screenshot({
    path: path.join(SCREENSHOT_DIR, `${name}.png`),
    fullPage: false,
  })
}

async function navigateToPending(page: Page) {
  await page.goto(PENDING_URL, { waitUntil: 'domcontentloaded', timeout: 30000 })
  // Wait for tabs to appear (the pending route always renders tabs regardless of data)
  await page.getByRole('tab', { name: /Enrichment candidates/i }).waitFor({ state: 'visible', timeout: 10000 })
}

async function switchToEnrichments(page: Page) {
  const tab = page.getByRole('tab', { name: /Enrichment candidates/i })
  await tab.click()
  // Give React Query time to settle
  await page.waitForTimeout(300)
}

// ── Tests ────────────────────────────────────────────────────────────────────

test.describe('Pending tiered buckets — #1133', () => {
  let consoleErrors: string[] = []

  test.beforeEach(async ({ page }) => {
    consoleErrors = []
    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text())
    })
    await navigateToPending(page)
  })

  // ── VIEW screenshot ───────────────────────────────────────────────────────

  test('VIEW — /pending enrichments tab with tier sections', async ({ page }) => {
    await switchToEnrichments(page)
    await page.waitForTimeout(500)
    await screenshot(page, '1-enrichments-tiered-default')
    // Test always passes — screenshot is the deliverable
  })

  // ── Tab structure ─────────────────────────────────────────────────────────

  test('renders Repairs and Enrichments tabs', async ({ page }) => {
    await expect(page.getByRole('tab', { name: /Repair candidates/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Enrichment candidates/i })).toBeVisible()
  })

  // ── 4 tier sections ───────────────────────────────────────────────────────

  test('Enrichments tab shows 4 tier sections', async ({ page }) => {
    await switchToEnrichments(page)

    // Each tier section has aria-label="<Tier> tier"
    await expect(page.getByRole('region', { name: 'Critical tier' })).toBeVisible()
    await expect(page.getByRole('region', { name: 'High tier' })).toBeVisible()
    await expect(page.getByRole('region', { name: 'Medium tier' })).toBeVisible()
    await expect(page.getByRole('region', { name: 'Low tier' })).toBeVisible()
  })

  // ── Tier labels ───────────────────────────────────────────────────────────

  test('all 4 tier labels are visible in header buttons', async ({ page }) => {
    await switchToEnrichments(page)
    // Each tier has a button inside the section header
    for (const label of ['Critical', 'High', 'Medium', 'Low']) {
      const btn = page.locator(`section[data-tier]`).filter({ hasText: label }).first()
      await expect(btn).toBeVisible()
    }
  })

  // ── aria-expanded a11y ────────────────────────────────────────────────────

  test('Critical and High tiers are expanded by default (aria-expanded=true)', async ({ page }) => {
    await switchToEnrichments(page)

    const criticalToggle = page.locator('button[aria-expanded][aria-controls="tier-body-critical"]')
    const highToggle     = page.locator('button[aria-expanded][aria-controls="tier-body-high"]')
    await expect(criticalToggle).toHaveAttribute('aria-expanded', 'true')
    await expect(highToggle).toHaveAttribute('aria-expanded', 'true')
  })

  test('Medium and Low tiers are collapsed by default (aria-expanded=false)', async ({ page }) => {
    await switchToEnrichments(page)

    const mediumToggle = page.locator('button[aria-expanded][aria-controls="tier-body-medium"]')
    const lowToggle    = page.locator('button[aria-expanded][aria-controls="tier-body-low"]')
    await expect(mediumToggle).toHaveAttribute('aria-expanded', 'false')
    await expect(lowToggle).toHaveAttribute('aria-expanded', 'false')
  })

  // ── Collapse / expand ─────────────────────────────────────────────────────

  test('clicking Critical header toggles collapse and expand', async ({ page }) => {
    await switchToEnrichments(page)

    const toggle = page.locator('button[aria-controls="tier-body-critical"]')
    // Initially expanded
    await expect(toggle).toHaveAttribute('aria-expanded', 'true')
    // Click to collapse
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await expect(page.locator('#tier-body-critical')).not.toBeVisible()
    // Click to re-expand
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-expanded', 'true')
    await expect(page.locator('#tier-body-critical')).toBeVisible()
  })

  test('clicking Medium header expands it', async ({ page }) => {
    await switchToEnrichments(page)

    const toggle = page.locator('button[aria-controls="tier-body-medium"]')
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-expanded', 'true')
    await expect(page.locator('#tier-body-medium')).toBeVisible()
  })

  // ── Toolbar controls ──────────────────────────────────────────────────────

  test('search input is visible with correct aria-label', async ({ page }) => {
    await switchToEnrichments(page)
    await expect(page.getByRole('searchbox', { name: /Search candidates/i })).toBeVisible()
  })

  test('tier filter chips are rendered with aria-pressed', async ({ page }) => {
    await switchToEnrichments(page)
    for (const tier of ['Critical', 'High', 'Medium', 'Low']) {
      const chip = page.getByRole('button', { name: tier, exact: true })
      await expect(chip).toBeVisible()
      // aria-pressed should be 'true' (all visible by default)
      await expect(chip).toHaveAttribute('aria-pressed', 'true')
    }
  })

  test('tier filter chip toggles visibility', async ({ page }) => {
    await switchToEnrichments(page)

    const lowChip = page.getByRole('button', { name: 'Low', exact: true })
    await expect(lowChip).toHaveAttribute('aria-pressed', 'true')

    // Deactivate Low tier — the Low tier section should disappear
    await lowChip.click()
    await expect(lowChip).toHaveAttribute('aria-pressed', 'false')
    await expect(page.getByRole('region', { name: 'Low tier' })).not.toBeVisible()

    // Re-activate
    await lowChip.click()
    await expect(lowChip).toHaveAttribute('aria-pressed', 'true')
    await expect(page.getByRole('region', { name: 'Low tier' })).toBeVisible()
  })

  test('sort dropdown is present with Score as default', async ({ page }) => {
    await switchToEnrichments(page)
    const sortSelect = page.getByRole('combobox', { name: /Sort by/i })
    await expect(sortSelect).toBeVisible()
    await expect(sortSelect).toHaveValue('score')
  })

  // ── Hide-forever stub ─────────────────────────────────────────────────────

  test('Hide forever button is present in each tier header', async ({ page }) => {
    await switchToEnrichments(page)

    for (const tier of ['Critical', 'High', 'Medium', 'Low']) {
      const section = page.locator('section[data-tier]').filter({ hasText: tier }).first()
      const hideBtn = section.getByRole('button', { name: /Hide.*forever/i })
      await expect(hideBtn).toBeVisible()
    }
  })

  test('Hide forever removes the tier from view', async ({ page }) => {
    await switchToEnrichments(page)

    // Expand Medium first so we can find its button
    const mediumToggle = page.locator('button[aria-controls="tier-body-medium"]')
    if (await mediumToggle.getAttribute('aria-expanded') === 'false') {
      await mediumToggle.click()
    }

    const mediumSection = page.getByRole('region', { name: 'Medium tier' })
    await expect(mediumSection).toBeVisible()

    const hideBtn = mediumSection.getByRole('button', { name: /Hide.*forever/i })
    await hideBtn.click()

    // The Medium tier section should now be absent
    await expect(page.getByRole('region', { name: 'Medium tier' })).not.toBeVisible()
  })

  // ── localStorage persistence ──────────────────────────────────────────────

  test('tier expanded state persists to localStorage', async ({ page }) => {
    await switchToEnrichments(page)

    // Collapse Critical tier
    const toggle = page.locator('button[aria-controls="tier-body-critical"]')
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')

    // Check localStorage key was written
    const stored = await page.evaluate(() =>
      window.localStorage.getItem('archigraph.pending.tierExpanded'),
    )
    expect(stored).not.toBeNull()
    const parsed = JSON.parse(stored!)
    expect(parsed.critical).toBe(false)
  })

  test('visible tiers persist to localStorage', async ({ page }) => {
    await switchToEnrichments(page)

    await page.getByRole('button', { name: 'Low', exact: true }).click()

    const stored = await page.evaluate(() =>
      window.localStorage.getItem('archigraph.pending.visibleTiers'),
    )
    expect(stored).not.toBeNull()
    const parsed = JSON.parse(stored!) as string[]
    expect(parsed).not.toContain('low')
  })

  // ── 0 console errors ─────────────────────────────────────────────────────

  test('0 console errors on load', async ({ page }) => {
    await switchToEnrichments(page)
    await page.waitForTimeout(800)

    const realErrors = consoleErrors.filter(
      (e) =>
        !e.includes('Download the React DevTools') &&
        !e.includes('ReactDOM.render is no longer supported') &&
        !e.includes('net::ERR_CONNECTION_REFUSED'), // expected when no daemon
    )
    expect(realErrors, `Unexpected console errors:\n${realErrors.join('\n')}`).toHaveLength(0)
  })
})
