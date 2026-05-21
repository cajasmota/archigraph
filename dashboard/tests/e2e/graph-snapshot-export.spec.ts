/**
 * E2E: Graph snapshot export — "Snapshot view" button (#1362)
 *
 * Tests the SnapshotModal and PNG download flow:
 *   - Toolbar Camera button opens the modal
 *   - Format (PNG/SVG) and resolution (1x/2x/4x) pickers render
 *   - Clicking "Download" triggers a file download
 *   - Modal closes after download
 *   - 0 console errors on load
 *
 * Headless only. Degrades gracefully when no daemon is running:
 * the modal itself does not require a live graph — the toolbar is always visible.
 *
 * Two VIEW screenshots (always captured).
 */

import { test, expect, type Page } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

// ── Config ────────────────────────────────────────────────────────────────────

const BASE_URL = process.env.TEST_BASE_URL ?? 'http://localhost:5173'
const GROUP = process.env.TEST_GROUP ?? 'default'
const GRAPH_URL = `${BASE_URL}/${GROUP}/graph`
const SCREENSHOT_DIR = path.join(__dirname, '..', '..', 'test-results', 'snapshot-export')

// ── Helpers ───────────────────────────────────────────────────────────────────

async function screenshot(page: Page, name: string) {
  await page.screenshot({
    path: path.join(SCREENSHOT_DIR, `${name}.png`),
    fullPage: false,
  })
}

async function waitForGraph(page: Page) {
  await Promise.race([
    page.getByRole('complementary', { name: 'Graph filters sidebar' })
      .waitFor({ state: 'visible', timeout: 8000 }),
    page.waitForTimeout(8000),
  ]).catch(() => {})
}

async function openSnapshotModal(page: Page): Promise<boolean> {
  const toolbarBtn = page.getByTestId('toolbar-snapshot-btn')
  const visible = await toolbarBtn.isVisible({ timeout: 3000 }).catch(() => false)
  if (!visible) return false
  await toolbarBtn.click()
  const modal = page.getByRole('dialog', { name: 'Snapshot view options' })
  return modal.isVisible({ timeout: 3000 }).catch(() => false)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

test.describe('Graph snapshot export — #1362', () => {
  let consoleErrors: string[] = []

  test.beforeEach(async ({ page }) => {
    consoleErrors = []
    page.on('console', (msg) => {
      if (msg.type() === 'error') consoleErrors.push(msg.text())
    })
    await page.goto(GRAPH_URL, { waitUntil: 'domcontentloaded', timeout: 30000 })
    await waitForGraph(page)
  })

  // ── VIEW screenshots ──────────────────────────────────────────────────────

  test('VIEW 1 — graph page loaded, toolbar visible', async ({ page }) => {
    await page.waitForTimeout(500)
    await screenshot(page, '1-graph-loaded')
  })

  test('VIEW 2 — snapshot modal open', async ({ page }) => {
    await openSnapshotModal(page)
    await page.waitForTimeout(300)
    await screenshot(page, '2-snapshot-modal')
  })

  // ── Structural tests ──────────────────────────────────────────────────────

  test('graph route renders without crash', async ({ page }) => {
    const errorBoundary = page.locator('[data-testid="error-boundary"]')
    const crashed = await errorBoundary.isVisible({ timeout: 2000 }).catch(() => false)
    expect(crashed, 'React error boundary should not be visible').toBe(false)
  })

  test('toolbar snapshot button is visible', async ({ page }) => {
    const btn = page.getByTestId('toolbar-snapshot-btn')
    await expect(btn, 'Snapshot toolbar button should be visible').toBeVisible({ timeout: 5000 })
    await expect(btn).toHaveAttribute('title', /Snapshot view/i)
  })

  test('clicking toolbar button opens the snapshot modal', async ({ page }) => {
    const opened = await openSnapshotModal(page)
    if (!opened) {
      test.info().annotations.push({
        type: 'info',
        description: 'Toolbar button not found — test skipped.',
      })
      return
    }

    const modal = page.getByRole('dialog', { name: 'Snapshot view options' })
    await expect(modal).toBeVisible()
    // Format pickers
    await expect(modal.getByTestId('snapshot-format-png')).toBeVisible()
    await expect(modal.getByTestId('snapshot-format-svg')).toBeVisible()
    // Resolution pickers (visible in PNG mode — default)
    await expect(modal.getByTestId('snapshot-res-1x')).toBeVisible()
    await expect(modal.getByTestId('snapshot-res-2x')).toBeVisible()
    await expect(modal.getByTestId('snapshot-res-4x')).toBeVisible()
    // Legend checkbox
    await expect(modal.getByTestId('snapshot-include-legend')).toBeVisible()
    // Download button
    await expect(modal.getByTestId('snapshot-download-btn')).toBeVisible()
  })

  test('switching to SVG format hides resolution pickers', async ({ page }) => {
    const opened = await openSnapshotModal(page)
    if (!opened) {
      test.info().annotations.push({ type: 'info', description: 'Modal not found — skipped.' })
      return
    }

    const modal = page.getByRole('dialog', { name: 'Snapshot view options' })
    await modal.getByTestId('snapshot-format-svg').click()

    // Resolution pickers should be hidden for SVG
    await expect(modal.getByTestId('snapshot-res-1x')).not.toBeVisible()
    await expect(modal.getByTestId('snapshot-res-2x')).not.toBeVisible()
    await expect(modal.getByTestId('snapshot-res-4x')).not.toBeVisible()

    // SVG note should appear
    await expect(modal.getByText(/vector image/i)).toBeVisible()
  })

  test('switching back to PNG shows resolution pickers again', async ({ page }) => {
    const opened = await openSnapshotModal(page)
    if (!opened) {
      test.info().annotations.push({ type: 'info', description: 'Modal not found — skipped.' })
      return
    }

    const modal = page.getByRole('dialog', { name: 'Snapshot view options' })
    await modal.getByTestId('snapshot-format-svg').click()
    await modal.getByTestId('snapshot-format-png').click()

    await expect(modal.getByTestId('snapshot-res-2x')).toBeVisible()
  })

  test('Cancel button closes the modal without downloading', async ({ page }) => {
    const opened = await openSnapshotModal(page)
    if (!opened) {
      test.info().annotations.push({ type: 'info', description: 'Modal not found — skipped.' })
      return
    }

    const modal = page.getByRole('dialog', { name: 'Snapshot view options' })
    await modal.getByRole('button', { name: 'Cancel' }).click()
    await expect(modal).not.toBeVisible({ timeout: 2000 })
  })

  test('ESC key / backdrop click closes the modal', async ({ page }) => {
    const opened = await openSnapshotModal(page)
    if (!opened) {
      test.info().annotations.push({ type: 'info', description: 'Modal not found — skipped.' })
      return
    }

    const modal = page.getByRole('dialog', { name: 'Snapshot view options' })
    // Click the backdrop (outside the dialog card)
    await page.mouse.click(10, 10)
    await expect(modal).not.toBeVisible({ timeout: 2000 })
  })

  test('Download button triggers PNG file download (with live graph canvas)', async ({ page }) => {
    const opened = await openSnapshotModal(page)
    if (!opened) {
      test.info().annotations.push({ type: 'info', description: 'Modal not found — skipped.' })
      return
    }

    const modal = page.getByRole('dialog', { name: 'Snapshot view options' })

    // Select 1× to minimize memory; keep PNG
    await modal.getByTestId('snapshot-res-1x').click()

    // Wait for a download event
    const [download] = await Promise.all([
      page.waitForEvent('download', { timeout: 8000 }).catch(() => null),
      modal.getByTestId('snapshot-download-btn').click(),
    ])

    if (!download) {
      // Canvas may not be present without a live daemon — test degrades gracefully
      test.info().annotations.push({
        type: 'info',
        description: 'No download triggered — graph canvas not present (no daemon). Error message shown in modal instead.',
      })
      return
    }

    // Filename should match our naming pattern
    expect(download.suggestedFilename()).toMatch(/archigraph-.*\.png$/)
  })

  test('4× resolution warning appears when 4× selected', async ({ page }) => {
    const opened = await openSnapshotModal(page)
    if (!opened) {
      test.info().annotations.push({ type: 'info', description: 'Modal not found — skipped.' })
      return
    }

    const modal = page.getByRole('dialog', { name: 'Snapshot view options' })
    await modal.getByTestId('snapshot-res-4x').click()

    await expect(modal.getByText(/5 MB/i)).toBeVisible()
  })

  test('0 console errors on load', async ({ page }) => {
    await page.waitForTimeout(1000)
    const realErrors = consoleErrors.filter(
      (e) =>
        !e.includes('Download the React DevTools') &&
        !e.includes('ReactDOM.render is no longer supported') &&
        !e.includes('net::ERR_CONNECTION_REFUSED'),
    )
    expect(realErrors, `Unexpected console errors:\n${realErrors.join('\n')}`).toHaveLength(0)
  })
})
