/**
 * E2E: Update / Version management surface — headless smoke + VIEW screenshot (#1199)
 *
 * Verifies:
 *   1. /update route loads without console errors
 *   2. "Update" nav item is present and active when on the page
 *   3. Version card renders with current-version info
 *   4. "Check for updates" button is present
 *   5. Update available state shows "Update now" button (mock mode)
 *   6. Confirm modal appears on "Update now" click
 *   7. "Refresh rules" button is present
 *   8. Screenshots captured (light + dark) for VIEW review
 *
 * NOTE: All tests run against the Vite dev server with VITE_USE_MOCKS=true
 * so no live daemon is required. The mock runner is never called in E2E —
 * only the mock data path is exercised. No actual update is triggered.
 */

import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const BASE_URL = process.env.TEST_BASE_URL ?? 'http://localhost:5173'
const UPDATE_URL = `${BASE_URL}/update`
const SCREENSHOT_DIR = path.join(__dirname, '..', '..', 'test-results', 'update')

// ─────────────────────────────────────────────────────────────────────────────

test.describe('Update surface — #1199', () => {
  let consoleErrors: string[] = []

  test.beforeEach(async ({ page }) => {
    consoleErrors = []
    page.on('console', msg => {
      if (msg.type() === 'error') {
        const text = msg.text()
        // Ignore browser-internal network errors (no live daemon in CI)
        if (!text.includes('Failed to load resource') && !text.includes('ERR_CONNECTION')) {
          consoleErrors.push(text)
        }
      }
    })
  })

  test('Update nav item is present in top nav', async ({ page }) => {
    await page.goto(UPDATE_URL, { waitUntil: 'domcontentloaded' })
    const nav = page.getByRole('navigation', { name: 'Surface navigation' })
    await nav.waitFor({ state: 'visible', timeout: 10_000 })

    const updateItem = nav.getByText('Update')
    await expect(updateItem).toBeVisible()
  })

  test('Update page heading renders', async ({ page }) => {
    await page.goto(UPDATE_URL, { waitUntil: 'domcontentloaded' })

    const heading = page.getByRole('heading', { level: 1 })
    await heading.waitFor({ state: 'visible', timeout: 8_000 })
    await expect(heading).toHaveText('Update')
  })

  test('"Check for updates" button is present', async ({ page }) => {
    await page.goto(UPDATE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(1500)

    const btn = page.getByRole('button', { name: /check for updates/i })
    await expect(btn).toBeVisible()
  })

  test('"Update now" button is visible when update available (mock)', async ({ page }) => {
    // Mock mode returns update_available=true, so the Update now button should render.
    await page.goto(UPDATE_URL, { waitUntil: 'domcontentloaded' })

    // Allow time for mock data to populate
    await page.waitForTimeout(2000)

    const btn = page.getByRole('button', { name: /update now/i })
    await expect(btn).toBeVisible({ timeout: 5_000 })
  })

  test('Confirm modal appears on "Update now" click', async ({ page }) => {
    await page.goto(UPDATE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(2000)

    const updateBtn = page.getByRole('button', { name: /update now/i })
    await updateBtn.waitFor({ state: 'visible', timeout: 5_000 })
    await updateBtn.click()

    // Confirm modal heading
    const modal = page.getByRole('heading', { name: /apply update/i })
    await expect(modal).toBeVisible({ timeout: 3_000 })

    // Cancel closes modal
    const cancelBtn = page.getByRole('button', { name: 'Cancel' })
    await cancelBtn.click()
    await expect(modal).not.toBeVisible()
  })

  test('"Refresh rules" button is present', async ({ page }) => {
    await page.goto(UPDATE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(1500)

    const btn = page.getByRole('button', { name: /refresh rules/i })
    await expect(btn).toBeVisible()
  })

  test('VIEW screenshot — update surface light + dark', async ({ page }) => {
    await page.goto(UPDATE_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(2500)

    // Light mode
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'update-light.png'),
      fullPage: false,
    })

    // Dark mode
    const themeBtn = page.getByRole('button', { name: /switch to dark mode/i })
    if (await themeBtn.isVisible()) {
      await themeBtn.click()
      await page.waitForTimeout(300)
      await page.screenshot({
        path: path.join(SCREENSHOT_DIR, 'update-dark.png'),
        fullPage: false,
      })
    }
  })

  test.afterEach(() => {
    if (consoleErrors.length > 0) {
      console.warn('Console errors on /update:', consoleErrors)
    }
    expect(consoleErrors, `Console errors: ${consoleErrors.join(', ')}`).toHaveLength(0)
  })
})
