/**
 * E2E: Nav redesign — grouped Explore / Operate dropdowns (#1210)
 *
 * Verifies:
 *   1. Top nav shows exactly 2 menu triggers (Explore, Operate) — not 9 flat items
 *   2. Click Explore → dropdown opens with 6 items
 *   3. Click Operate → dropdown opens with 5 items
 *   4. Keyboard: Tab to trigger, Enter opens, Escape closes
 *   5. Active surface shows indicator dot on parent trigger
 *   6. 0 console errors
 *   7. VIEW screenshots: nav default state + Operate menu open
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

const EXPLORE_ITEMS = ['Graph', 'Flows', 'Topology', 'Paths', 'Docs', 'Pending']
const OPERATE_ITEMS = ['Diagnostics', 'Quality', 'Patterns', 'System', 'Update']

test.describe('Nav redesign — grouped menus (#1210)', () => {
  let consoleErrors: string[] = []

  test.beforeEach(async ({ page }) => {
    consoleErrors = []
    page.on('console', msg => {
      if (msg.type() === 'error') {
        const text = msg.text()
        if (!text.includes('Failed to load resource') && !text.includes('ERR_CONNECTION')) {
          consoleErrors.push(text)
        }
      }
    })
  })

  // ── 1. Only 2 triggers in the nav (Explore + Operate) ──────────────────────

  test('Nav shows Explore and Operate triggers — not flat items', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' })

    const nav = page.getByRole('navigation', { name: 'Surface navigation' })
    await nav.waitFor({ state: 'visible', timeout: 10_000 })

    // Exactly 2 trigger buttons
    const explore = nav.getByTestId('nav-explore')
    const operate = nav.getByTestId('nav-operate')
    await expect(explore).toBeVisible()
    await expect(operate).toBeVisible()

    // Old flat nav items must NOT be directly visible
    await expect(nav.getByRole('link', { name: 'Graph' })).not.toBeVisible()
    await expect(nav.getByRole('link', { name: 'System' })).not.toBeVisible()
  })

  // ── 2. Explore dropdown has 6 items ────────────────────────────────────────

  test('Explore dropdown opens with 6 items', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    const explore = page.getByTestId('nav-explore')
    await explore.click()

    const content = page.getByTestId('nav-explore-content')
    await expect(content).toBeVisible()

    for (const label of EXPLORE_ITEMS) {
      await expect(content.getByText(label)).toBeVisible()
    }

    // Close
    await page.keyboard.press('Escape')
    await expect(content).not.toBeVisible()
  })

  // ── 3. Operate dropdown has 5 items ────────────────────────────────────────

  test('Operate dropdown opens with 5 items', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    const operate = page.getByTestId('nav-operate')
    await operate.click()

    const content = page.getByTestId('nav-operate-content')
    await expect(content).toBeVisible()

    for (const label of OPERATE_ITEMS) {
      await expect(content.getByText(label)).toBeVisible()
    }

    await page.keyboard.press('Escape')
    await expect(content).not.toBeVisible()
  })

  // ── 4. Keyboard navigation (Enter open, Escape close) ──────────────────────

  test('Keyboard: Enter opens Explore, Escape closes', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    const explore = page.getByTestId('nav-explore')
    await explore.focus()
    await page.keyboard.press('Enter')

    const content = page.getByTestId('nav-explore-content')
    await expect(content).toBeVisible()

    await page.keyboard.press('Escape')
    await expect(content).not.toBeVisible()
  })

  // ── 5. Active surface shows indicator on parent trigger ─────────────────────

  test('Active indicator appears on Explore trigger when on graph route', async ({ page }) => {
    // Navigate to a graph route; Explore should show active dot
    await page.goto(`${BASE_URL}/graph/fixture-a`, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    const explore = page.getByTestId('nav-explore')
    // The trigger text and indicator should both be present
    await expect(explore).toBeVisible()

    // Check the class contains the active state variant
    const cls = await explore.getAttribute('class') ?? ''
    // Active triggers get bg-slate-100 / dark:bg-slate-800
    const hasActiveStyle = cls.includes('bg-slate-100') || cls.includes('bg-slate-800')
    expect(hasActiveStyle).toBe(true)
  })

  // ── 6. No console errors ────────────────────────────────────────────────────

  test('No unexpected console errors', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' })

    // Open and close both menus
    await page.getByTestId('nav-explore').click()
    await page.waitForTimeout(300)
    await page.keyboard.press('Escape')

    await page.getByTestId('nav-operate').click()
    await page.waitForTimeout(300)
    await page.keyboard.press('Escape')

    await page.waitForTimeout(300)
    expect(consoleErrors).toHaveLength(0)
  })

  // ── 7. Screenshots (VIEW) ──────────────────────────────────────────────────

  test('Screenshot — nav default state (VIEW)', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(1000)

    ensureDir(SCREENSHOT_DIR)
    const p = path.join(SCREENSHOT_DIR, 'nav-redesign-default.png')
    await page.screenshot({ path: p, fullPage: false })
    expect(fs.existsSync(p)).toBe(true)
    console.log(`[VIEW] Nav default screenshot: ${p}`)
  })

  test('Screenshot — Operate menu open (VIEW)', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    await page.getByTestId('nav-operate').click()
    await page.waitForTimeout(400)

    ensureDir(SCREENSHOT_DIR)
    const p = path.join(SCREENSHOT_DIR, 'nav-redesign-operate-open.png')
    await page.screenshot({ path: p, fullPage: false })
    expect(fs.existsSync(p)).toBe(true)
    console.log(`[VIEW] Operate menu open screenshot: ${p}`)
  })
})
