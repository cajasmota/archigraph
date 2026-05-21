/**
 * E2E: Skills surface — headless smoke + VIEW screenshot (#1354)
 *
 * Verifies:
 *   1. /skills route loads without unexpected console errors
 *   2. "Skills" nav item appears in the Operate dropdown
 *   3. Page renders heading and tabs
 *   4. Installed tab shows skills list (mocked via route intercept)
 *   5. Marketplace tab renders catalog cards
 *   6. Marketplace search filters results
 *   7. Install button is present and clickable on uninstalled skills
 *   8. Remove button is present on installed skills
 *   9. Screenshots captured for VIEW review
 */

import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'
import fs from 'fs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const BASE_URL = process.env.TEST_BASE_URL ?? 'http://localhost:5173'
const SKILLS_URL = `${BASE_URL}/skills`
const SCREENSHOT_DIR = path.join(__dirname, '..', '..', 'e2e-screenshots')

function ensureDir(dir: string) {
  if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock API payloads
// ─────────────────────────────────────────────────────────────────────────────

const INSTALLED_MOCK = {
  skills: [
    {
      slug: 'using-archigraph',
      name: 'using-archigraph',
      description: 'Teaches an AI agent how to use archigraph MCP tools effectively.',
      type: 'behavior',
      when_to_use: 'Invoke when opening a codebase that has archigraph indexed.',
      version: 'bundled',
      last_invoked_at: new Date(Date.now() - 60_000).toISOString(),
      total_invocations: 12,
      update_available: false,
    },
    {
      slug: 'generate-docs',
      name: 'generate-docs',
      description: 'Generate comprehensive documentation for a codebase.',
      type: 'action',
      when_to_use: 'When you want to create or refresh /docs.',
      version: 'bundled',
      last_invoked_at: new Date(Date.now() - 3_600_000).toISOString(),
      total_invocations: 5,
      update_available: false,
    },
    {
      slug: 'archigraph-aware-review',
      name: 'archigraph-aware-review',
      description: 'Reviews a PR with structural awareness.',
      type: 'action',
      when_to_use: 'Before merging a PR that touches core business logic.',
      version: 'bundled',
      last_invoked_at: undefined,
      total_invocations: 0,
      update_available: false,
    },
  ],
  skills_dir: 'skills',
}

const AVAILABLE_MOCK = {
  skills: [
    {
      slug: 'using-archigraph',
      name: 'using-archigraph',
      description: 'Teaches an AI agent how to use archigraph MCP tools effectively.',
      type: 'behavior',
      when_to_use: 'Invoke when opening a codebase that has archigraph indexed.',
      version: 'bundled',
      source: 'archigraph-bundled',
      install_url: undefined,
      installed: true,
    },
    {
      slug: 'generate-docs',
      name: 'generate-docs',
      description: 'Generate comprehensive documentation for a codebase.',
      type: 'action',
      when_to_use: 'When you want to create or refresh /docs.',
      version: 'bundled',
      source: 'archigraph-bundled',
      install_url: undefined,
      installed: true,
    },
    {
      slug: 'openapi-diff',
      name: 'openapi-diff',
      description: 'Compares two OpenAPI specs and highlights breaking changes.',
      type: 'action',
      when_to_use: 'When upgrading a downstream API version.',
      version: '0.2.1',
      source: 'community',
      install_url: 'https://skills.archigraph.dev/community/openapi-diff',
      installed: false,
    },
    {
      slug: 'changelog-generator',
      name: 'changelog-generator',
      description: 'Produces a human-readable CHANGELOG from git history.',
      type: 'action',
      when_to_use: 'Before a release tag.',
      version: '1.0.0',
      source: 'community',
      install_url: 'https://skills.archigraph.dev/community/changelog-generator',
      installed: false,
    },
    {
      slug: 'security-audit',
      name: 'security-audit',
      description: 'Deep security audit combining archigraph auth-coverage data with OWASP.',
      type: 'action',
      when_to_use: 'Before a public API launch.',
      version: '0.1.3',
      source: 'community',
      install_url: 'https://skills.archigraph.dev/community/security-audit',
      installed: false,
    },
  ],
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

test.describe('Skills surface — #1354', () => {
  let consoleErrors: string[] = []

  test.beforeEach(async ({ page }) => {
    consoleErrors = []
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        const text = msg.text()
        if (
          !text.includes('Failed to load resource') &&
          !text.includes('ERR_CONNECTION') &&
          !text.includes('ERR_FAILED') &&
          !text.includes('net::ERR')
        ) {
          consoleErrors.push(text)
        }
      }
    })

    // Intercept skills API calls and return mock data
    await page.route('**/api/skills/installed', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(INSTALLED_MOCK),
      })
    })
    await page.route('**/api/skills/available', (route) => {
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(AVAILABLE_MOCK),
      })
    })
    await page.route('**/api/skills/install', (route) => {
      const { slug } = route.request().postDataJSON() as { slug: string }
      route.fulfill({
        status: 201,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true, slug, dir: `skills/${slug}` }),
      })
    })
    await page.route('**/api/skills/uninstall', (route) => {
      const { slug } = route.request().postDataJSON() as { slug: string }
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true, slug }),
      })
    })
  })

  // ── 1. No unexpected console errors ──────────────────────────────────────

  test('No unexpected console errors on /skills', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(2000)
    expect(consoleErrors).toHaveLength(0)
  })

  // ── 2. Nav item in Operate dropdown ──────────────────────────────────────

  test('"Skills" appears in Operate dropdown', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    const nav = page.getByRole('navigation', { name: 'Surface navigation' })
    await expect(nav).toBeVisible()

    const operateTrigger = nav.getByTestId('nav-operate')
    await expect(operateTrigger).toBeVisible()
    await operateTrigger.click()
    await page.waitForTimeout(300)

    const content = page.getByTestId('nav-operate-content')
    await expect(content).toBeVisible()
    await expect(content.getByText('Skills')).toBeVisible()
  })

  // ── 3. Page heading and tabs ──────────────────────────────────────────────

  test('Page renders heading and both tabs', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    await expect(page.getByRole('heading', { level: 1, name: /skills/i })).toBeVisible()
    await expect(page.getByTestId('tab-installed')).toBeVisible()
    await expect(page.getByTestId('tab-marketplace')).toBeVisible()
  })

  // ── 4. Installed tab shows skills ─────────────────────────────────────────

  test('Installed tab lists installed skills', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(800)

    // Should default to Installed tab
    await expect(page.getByTestId('tab-installed')).toBeVisible()

    // All three bundled skills should be visible
    await expect(page.getByTestId('skill-installed-using-archigraph')).toBeVisible()
    await expect(page.getByTestId('skill-installed-generate-docs')).toBeVisible()
    await expect(page.getByTestId('skill-installed-archigraph-aware-review')).toBeVisible()
  })

  // ── 5. Remove button on installed cards ──────────────────────────────────

  test('Installed cards have Remove buttons', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(800)

    const removeBtn = page.getByTestId('btn-remove-using-archigraph')
    await expect(removeBtn).toBeVisible()
    await expect(removeBtn).toBeEnabled()
  })

  // ── 6. Marketplace tab loads catalog ─────────────────────────────────────

  test('Marketplace tab shows catalog cards', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    await page.getByTestId('tab-marketplace').click()
    await page.waitForTimeout(600)

    // Community skill cards
    await expect(page.getByTestId('skill-catalog-openapi-diff')).toBeVisible()
    await expect(page.getByTestId('skill-catalog-changelog-generator')).toBeVisible()
    await expect(page.getByTestId('skill-catalog-security-audit')).toBeVisible()
  })

  // ── 7. Marketplace search filters results ────────────────────────────────

  test('Marketplace search filters catalog cards', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    await page.getByTestId('tab-marketplace').click()
    await page.waitForTimeout(600)

    const search = page.getByTestId('marketplace-search')
    await expect(search).toBeVisible()

    // Type a query that only matches openapi-diff
    await search.fill('openapi')
    await page.waitForTimeout(300)

    await expect(page.getByTestId('skill-catalog-openapi-diff')).toBeVisible()
    // Other cards should be hidden
    await expect(page.getByTestId('skill-catalog-security-audit')).not.toBeVisible()

    // Clear and verify all return
    await search.fill('')
    await page.waitForTimeout(300)
    await expect(page.getByTestId('skill-catalog-security-audit')).toBeVisible()
  })

  // ── 8. Install button on uninstalled catalog skills ───────────────────────

  test('Uninstalled catalog skills have Install button', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    await page.getByTestId('tab-marketplace').click()
    await page.waitForTimeout(600)

    const installBtn = page.getByTestId('btn-install-openapi-diff')
    await expect(installBtn).toBeVisible()
    await expect(installBtn).toBeEnabled()
  })

  // ── 9. Already-installed skills show "Installed" badge, not Install button

  test('Already-installed skills show Installed badge in marketplace', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    await page.getByTestId('tab-marketplace').click()
    await page.waitForTimeout(600)

    const uaCard = page.getByTestId('skill-catalog-using-archigraph')
    await expect(uaCard).toBeVisible()
    await expect(uaCard.getByText('Installed')).toBeVisible()
    // Install button should NOT be present
    await expect(page.getByTestId('btn-install-using-archigraph')).not.toBeVisible()
  })

  // ── 10. Screenshots (VIEW) ────────────────────────────────────────────────

  test('Screenshot — Installed tab (VIEW)', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(1500)

    ensureDir(SCREENSHOT_DIR)
    const screenshotPath = path.join(SCREENSHOT_DIR, 'skills-installed-tab.png')
    await page.screenshot({ path: screenshotPath, fullPage: true })

    expect(fs.existsSync(screenshotPath)).toBe(true)
    console.log(`[VIEW] Screenshot saved: ${screenshotPath}`)
  })

  test('Screenshot — Marketplace tab (VIEW)', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    await page.getByTestId('tab-marketplace').click()
    await page.waitForTimeout(800)

    ensureDir(SCREENSHOT_DIR)
    const screenshotPath = path.join(SCREENSHOT_DIR, 'skills-marketplace-tab.png')
    await page.screenshot({ path: screenshotPath, fullPage: true })

    expect(fs.existsSync(screenshotPath)).toBe(true)
    console.log(`[VIEW] Screenshot saved: ${screenshotPath}`)
  })

  test('Screenshot — Marketplace search filtered (VIEW)', async ({ page }) => {
    await page.goto(SKILLS_URL, { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(500)

    await page.getByTestId('tab-marketplace').click()
    await page.waitForTimeout(600)

    const search = page.getByTestId('marketplace-search')
    await search.fill('security')
    await page.waitForTimeout(300)

    ensureDir(SCREENSHOT_DIR)
    const screenshotPath = path.join(SCREENSHOT_DIR, 'skills-marketplace-search.png')
    await page.screenshot({ path: screenshotPath, fullPage: false })

    expect(fs.existsSync(screenshotPath)).toBe(true)
    console.log(`[VIEW] Search screenshot saved: ${screenshotPath}`)
  })
})
