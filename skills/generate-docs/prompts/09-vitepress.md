# Pass 9 — VitePress site (optional)

Run this pass only if `plan.passes.9_vitepress.enabled` is true (i.e., the user said yes in Pass 0 question 11).

## What you produce

A minimal VitePress site at `~/.archigraph/groups/<group>/docs/` that consumes the markdown produced in Passes 3-7 from each repo and from the group state directory. The site is read-only — it does not duplicate content; it builds against the existing files via symlinks or a pre-build copy step the user controls.

## Files to write

```
~/.archigraph/groups/<group>/docs/
  .vitepress/
    config.ts
  index.md
  package.json
```

### `.vitepress/config.ts`

```typescript
import { defineConfig } from 'vitepress'

export default defineConfig({
  title: '<Group display name>',
  description: '<one-line mission from domain.md>',
  base: '/',
  themeConfig: {
    nav: [
      { text: 'Group', link: '/group-synthesis' },
      { text: 'Repos', items: [
        // one entry per repo, link to /repos/<slug>/overview
      ]},
      { text: 'Cross-cutting', link: '/cross-cutting/auth' },
    ],
    sidebar: {
      '/repos/<slug>/': [
        { text: 'Overview', link: '/repos/<slug>/overview' },
        { text: 'Modules', items: [ /* one per module-slug */ ] },
        { text: 'Reference', items: [
          { text: 'API', link: '/repos/<slug>/reference/api' },
          { text: 'Config', link: '/repos/<slug>/reference/config' },
          { text: 'Deployment', link: '/repos/<slug>/reference/deployment' },
          { text: 'Scripts', link: '/repos/<slug>/reference/scripts' },
          { text: 'Dependencies', link: '/repos/<slug>/reference/dependencies' },
        ]},
      ],
      '/cross-cutting/': [
        { text: 'Auth', link: '/cross-cutting/auth' },
        { text: 'Logging', link: '/cross-cutting/logging' },
        { text: 'Errors', link: '/cross-cutting/errors' },
        { text: 'Observability', link: '/cross-cutting/observability' },
      ],
    },
    search: { provider: 'local' },
  },
})
```

Replace placeholders with values from `domain.md` and `plan.json`.

### `index.md`

A landing page that summarizes the group in three short sections and links to `group-synthesis.md`. Keep it under 200 words.

### `package.json`

```json
{
  "name": "<group>-docs",
  "private": true,
  "type": "module",
  "scripts": {
    "docs:dev": "vitepress dev .",
    "docs:build": "vitepress build .",
    "docs:preview": "vitepress preview ."
  },
  "devDependencies": {
    "vitepress": "^1.0.0"
  }
}
```

## Bringing repo docs into the site

Two strategies — pick whichever the user prefers:

1. **Symlink** each `<repo>/docs` into `~/.archigraph/groups/<group>/docs/repos/<slug>/`. Fast, no duplication, requires unix-likes.
2. **Pre-build copy**: write a small `scripts/sync-docs.sh` that copies before `docs:build`. Cross-platform.

Default to symlinks; mention the alternative in `index.md`.

## Verification

- `npx vitepress build .` exits zero.
- Every link from `group-synthesis.md` and `cross-links.md` resolves under the built site.
- `_graph-searchability.md` rule still satisfied — VitePress slugifies headings the same way archigraph does, so backticked identifiers in headings produce anchor URLs that match the graph slugs.

When done, hand back to the orchestrator with a one-line summary and the path to the built site (or the dev-server command).
