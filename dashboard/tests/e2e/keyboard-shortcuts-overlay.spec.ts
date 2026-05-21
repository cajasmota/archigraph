/**
 * E2E: Keyboard shortcuts overlay (#1245)
 *
 * Verifies:
 *   1. Press '?' → overlay opens
 *   2. Esc closes overlay
 *   3. All 6 categories rendered
 *   4. Search input filters shortcuts
 *   5. Category collapse toggle works
 *   6. '?' ignored when focus in input
 *   7. Cmd+K → 'Keyboard shortcuts' action opens overlay
 *   8. No console errors
 *   9. Screenshots (VIEW): open, filtered, collapsed
 */

import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'
import fs from 'fs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const BASE_URL = process.env.TEST_BASE_URL ?? 'http://localhost:5173'
const SCREENSHOT_DIR = path.join(__dirname, '..', '..', 'e2e-screenshots')

function ensureDir(dir: string) {
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
}

test.describe('Keyboard shortcuts overlay (#1245)', () => {
  let consoleErrors: string[] = []

  test.beforeEach(async ({ page }) => {
    consoleErrors = []
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        const text = msg.text()
        if (!text.includes('Failed to load resource') && !text.includes('ERR_CONNECTION')) {
          consoleErrors.push(text)
        }
      }
    })
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)
  })

  // ── 1. '?' opens overlay ──────────────────────────────────────────────────

  test("Press '?' opens the shortcuts overlay", async ({ page }) => {
    await expect(page.getByTestId('shortcuts-overlay')).not.toBeVisible()

    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()
    await expect(page.getByTestId('shortcuts-overlay-search')).toBeFocused()
  })

  // ── 2. Esc closes overlay ─────────────────────────────────────────────────

  test('Esc closes the shortcuts overlay', async ({ page }) => {
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()

    await page.keyboard.press('Escape')
    await expect(page.getByTestId('shortcuts-overlay')).not.toBeVisible()
  })

  // ── 3. X button closes overlay ────────────────────────────────────────────

  test('Close button (X) closes the overlay', async ({ page }) => {
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()

    await page.getByTestId('shortcuts-overlay-close').click()
    await expect(page.getByTestId('shortcuts-overlay')).not.toBeVisible()
  })

  // ── 4. All 6 categories rendered ──────────────────────────────────────────

  test('All 6 shortcut categories are rendered', async ({ page }) => {
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()

    for (const id of ['global', 'graph', 'topology', 'lists', 'modals', 'indexing']) {
      await expect(page.getByTestId(`shortcuts-category-${id}`)).toBeVisible()
    }
  })

  // ── 5. Search filters shortcuts ───────────────────────────────────────────

  test("Search 'zoom' filters to only zoom shortcuts", async ({ page }) => {
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()

    await page.getByTestId('shortcuts-overlay-search').fill('zoom')
    await page.waitForTimeout(200)

    // global category should not be visible (no zoom shortcuts there)
    await expect(page.getByTestId('shortcuts-category-global')).not.toBeVisible()
    // graph category should be visible (zoom in/out)
    await expect(page.getByTestId('shortcuts-category-graph')).toBeVisible()
  })

  // ── 6. Category collapse toggle ───────────────────────────────────────────

  test('Clicking a category heading collapses and expands it', async ({ page }) => {
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()

    // Collapse global
    await page.getByTestId('shortcuts-category-toggle-global').click()
    // The section should be hidden — check that no shortcut rows are visible inside global
    const globalSection = page.getByTestId('shortcuts-category-global')
    const rows = globalSection.getByTestId('shortcut-row')
    await expect(rows.first()).not.toBeVisible()

    // Expand again
    await page.getByTestId('shortcuts-category-toggle-global').click()
    await expect(rows.first()).toBeVisible()
  })

  // ── 7. '?' ignored when focus in input ───────────────────────────────────

  test("'?' key is ignored when an input is focused", async ({ page }) => {
    // Focus the cmd-palette search chip button area — use a text input present on screen
    // Open cmd palette to get an input focused
    await page.keyboard.press('Meta+k')
    await expect(page.getByTestId('cmd-palette')).toBeVisible()

    // Now press '?' — overlay should NOT open behind the palette
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).not.toBeVisible()

    // Close palette
    await page.keyboard.press('Escape')
  })

  // ── 8. Cmd+K → 'Keyboard shortcuts' action opens overlay ──────────────────

  test("Cmd+K then select 'Keyboard shortcuts' action opens overlay", async ({ page }) => {
    await page.keyboard.press('Meta+k')
    await expect(page.getByTestId('cmd-palette')).toBeVisible()

    await page.getByTestId('cmd-palette-input').fill('shortcuts')
    await page.waitForTimeout(200)

    // Click the keyboard shortcuts action
    await page.getByTestId('cmd-item-action-keyboard-shortcuts').click()

    // Palette should close and overlay should open
    await expect(page.getByTestId('cmd-palette')).not.toBeVisible()
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()
  })

  // ── 9. No console errors ──────────────────────────────────────────────────

  test('No unexpected console errors', async ({ page }) => {
    await page.keyboard.press('?')
    await page.waitForTimeout(300)

    await page.getByTestId('shortcuts-overlay-search').fill('zoom')
    await page.waitForTimeout(200)

    await page.keyboard.press('Escape')
    await page.waitForTimeout(300)

    expect(consoleErrors).toHaveLength(0)
  })

  // ── 10. Screenshots (VIEW) ────────────────────────────────────────────────

  test('Screenshot — overlay open (VIEW)', async ({ page }) => {
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()
    await page.waitForTimeout(400)

    ensureDir(SCREENSHOT_DIR)
    const p = path.join(SCREENSHOT_DIR, 'shortcuts-overlay-open.png')
    await page.screenshot({ path: p, fullPage: false })
    expect(fs.existsSync(p)).toBe(true)
    console.log(`[VIEW] Shortcuts overlay open: ${p}`)
  })

  test("Screenshot — overlay filtered by 'zoom' (VIEW)", async ({ page }) => {
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()
    await page.getByTestId('shortcuts-overlay-search').fill('zoom')
    await page.waitForTimeout(300)

    ensureDir(SCREENSHOT_DIR)
    const p = path.join(SCREENSHOT_DIR, 'shortcuts-overlay-filtered.png')
    await page.screenshot({ path: p, fullPage: false })
    expect(fs.existsSync(p)).toBe(true)
    console.log(`[VIEW] Shortcuts overlay filtered: ${p}`)
  })

  test('Screenshot — global category collapsed (VIEW)', async ({ page }) => {
    await page.keyboard.press('?')
    await expect(page.getByTestId('shortcuts-overlay')).toBeVisible()
    await page.getByTestId('shortcuts-category-toggle-global').click()
    await page.waitForTimeout(200)

    ensureDir(SCREENSHOT_DIR)
    const p = path.join(SCREENSHOT_DIR, 'shortcuts-overlay-collapsed.png')
    await page.screenshot({ path: p, fullPage: false })
    expect(fs.existsSync(p)).toBe(true)
    console.log(`[VIEW] Shortcuts overlay collapsed: ${p}`)
  })
})
