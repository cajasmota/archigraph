/* ============================================================
   RefLine.tsx — canonical single-line entity reference row.

   Issue #1910: unified format across Defined-in / Called-by /
   Downstream in the endpoint detail pane (and future detail pages).

   Row layout:
     [repo chip]  file:line (mono, dim)  entity name (regular)

   Props:
     repo   — owning repository slug (shown as a small colored chip)
     file   — source file path (basename:line shown in mono dim text)
     line   — source line number
     name   — entity / caller name (regular weight)
     kind?  — optional kind label shown as a small tag after the name
   ============================================================ */

import { cn } from "@/lib/utils";

export interface RefLineProps {
  repo: string;
  file: string;
  line: number;
  name: string;
  kind?: string;
  /** Accessibility: full title on hover (defaults to "repo · file:line  name") */
  title?: string;
  className?: string;
}

/** Stable hash → pastel color index (1-9) for the repo chip. */
function repoColorIndex(repo: string): number {
  let h = 0;
  for (let i = 0; i < repo.length; i++) {
    h = (h * 31 + repo.charCodeAt(i)) & 0xffffffff;
  }
  return (Math.abs(h) % 9) + 1;
}

/** Basename of a file path: "src/orders/order.service.ts" → "order.service.ts" */
function basename(filePath: string): string {
  const last = filePath.lastIndexOf("/");
  return last >= 0 ? filePath.slice(last + 1) : filePath;
}

/**
 * RefLine — one-line entity reference used in Defined-in, Called-by, and
 * Downstream sections. Keeps all three sections visually consistent and
 * scannable: repo chip on the left, file:line in mono dim, name in regular
 * weight on the right.
 */
export function RefLine({
  repo,
  file,
  line,
  name,
  kind,
  title,
  className,
}: RefLineProps) {
  const ci = repoColorIndex(repo);
  const fileLabel = file ? `${basename(file)}:${line}` : line > 0 ? `:${line}` : "";
  const derivedTitle =
    title ?? `${repo} · ${file}:${line}  ${name}${kind ? ` (${kind})` : ""}`;

  return (
    <div
      className={cn(
        "flex items-center gap-2 py-1 px-4 min-w-0",
        "hover:bg-surface-2 transition-colors duration-75",
        className,
      )}
      title={derivedTitle}
    >
      {/* Repo chip — distinct color per repo slug for quick scanning */}
      {repo && (
        <span
          className={cn(
            "shrink-0 inline-flex items-center h-[18px] px-1.5 rounded",
            "text-[10px] font-semibold font-mono leading-none select-none",
            // Use pastel CSS token for background, darker ink for text.
          )}
          style={{
            background: `var(--pastel-${ci})`,
            color: `var(--pastel-${ci}-ink)`,
          }}
          title={repo}
        >
          {repo}
        </span>
      )}

      {/* file:line — mono, dimmed */}
      {fileLabel && (
        <span
          className="font-mono text-[11px] text-text-4 shrink-0 tabular-nums truncate max-w-[160px]"
          title={file ? `${file}:${line}` : String(line)}
        >
          {fileLabel}
        </span>
      )}

      {/* Entity name — regular weight, takes remaining space */}
      <span
        className="text-xs text-text truncate flex-1 font-mono"
        title={name}
      >
        {name}
      </span>

      {/* Optional kind tag */}
      {kind && (
        <span className="shrink-0 text-[10px] text-text-4 font-mono px-1 rounded bg-surface-2">
          {kind}
        </span>
      )}
    </div>
  );
}
