import { useState, useCallback } from 'react'
import { MarkdownRenderer } from './MarkdownRenderer'
import { DocsBreadcrumbs } from './DocsBreadcrumbs'
import { DocsPrevNext } from './DocsPrevNext'
import { DocsTOC } from './DocsTOC'
import { useTocScrollSpy } from '@/hooks/docs/useTocScrollSpy'
import type { DocContentResponse, TocHeading } from '@/types/docs'

interface DocsContentProps {
  group: string
  content: DocContentResponse
}

/**
 * Content area — max-width constraint for readability (~720px), breadcrumbs,
 * markdown, and prev/next nav.
 * Pairs with <DocsTOC> in the right rail (provided here as portal-rendered).
 */
export function DocsContent({ group, content }: DocsContentProps) {
  const [headings, setHeadings] = useState<TocHeading[]>([])
  const activeId = useTocScrollSpy(headings)

  const handleHeadingsFound = useCallback((found: TocHeading[]) => {
    setHeadings(found)
  }, [])

  return (
    <div className="flex gap-8 min-h-full">
      {/* Main content */}
      <article className="flex-1 min-w-0 px-8 py-8" id="main-content">
        <DocsBreadcrumbs group={group} crumbs={content.breadcrumbs} />

        <MarkdownRenderer
          markdown={content.markdown}
          hovercards={content.hovercards}
          onHeadingsFound={handleHeadingsFound}
        />

        <DocsPrevNext group={group} prev={content.prev} next={content.next} />
      </article>

      {/* Right-rail TOC */}
      {headings.length > 0 && (
        <aside
          className="hidden xl:block w-56 flex-shrink-0 py-8 pr-4"
          aria-label="Table of contents"
        >
          <div className="sticky top-16">
            <DocsTOC headings={headings} activeId={activeId} />
          </div>
        </aside>
      )}
    </div>
  )
}
