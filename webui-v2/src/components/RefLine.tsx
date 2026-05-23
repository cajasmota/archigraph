/* ============================================================
   RefLine.tsx — canonical single-line entity reference row.

   Issue #1910: unified format across Defined-in / Called-by /
   Downstream in the endpoint detail pane (and future detail pages).

   Issue #1934: file path is a full clickable link with RTL
   ellipsis on overflow; per-row kind/framework chips removed.

   Row layout:
     [repo chip]  full/file/path:line (mono link, RTL trunc)  entity name (regular)

   Props:
     repo   — owning repository slug (shown as a small colored chip)
     file   — source file path (full relative path shown as a link)
     line   — source line number
     name   — entity / caller name (regular weight)
   ============================================================ */

import { cn } from "@/lib/utils";

export interface RefLineProps {
  repo: string;
  file: string;
  line: number;
  name: string;
  /** Accessibility: full title on hover (defaults to "repo · file:line  name") */
  title?: string;
  className?: string;
  /** Called when the file path link is clicked. Receives "file:line" string. */
  onFileClick?: (fileRef: string) => void;
}

/** Stable hash → pastel color index (1-9) for the repo chip. */
function repoColorIndex(repo: string): number {
  let h = 0;
  for (let i = 0; i < repo.length; i++) {
    h = (h * 31 + repo.charCodeAt(i)) & 0xffffffff;
  }
  return (Math.abs(h) % 9) + 1;
}

/**
 * RefLine — one-line entity reference used in Defined-in, Called-by, and
 * Downstream sections. Keeps all three sections visually consistent and
 * scannable: repo chip on the left, full file:line path as a clickable link
 * (RTL ellipsis so filename+line stays visible on overflow), entity name on
 * the right.
 *
 * Issue #1934: kind/framework per-row chips removed — they live in the
 * endpoint header chip strip and are redundant at row level.
 */
export function RefLine({
  repo,
  file,
  line,
  name,
  title,
  className,
  onFileClick,
}: RefLineProps) {
  const ci = repoColorIndex(repo);
  // Full path including line number — never trimmed to basename.
  const fileLabel = file ? `${file}:${line}` : line > 0 ? `:${line}` : "";
  const derivedTitle = title ?? `${repo} · ${file}:${line}  ${name}`;

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

      {/* Full file:line path — clickable link with RTL overflow so filename+line
          remains visible when the path is long and the container is narrow.
          `direction: rtl` causes overflow to clip on the LEFT side; the
          rightmost content (filename:line) is always fully visible. */}
      {fileLabel && (
        <button
          type="button"
          onClick={() => onFileClick?.(fileLabel)}
          title={fileLabel}
          className={cn(
            "font-mono text-[11px] tabular-nums text-left",
            "min-w-0 overflow-hidden whitespace-nowrap text-ellipsis",
            "text-accent hover:underline cursor-pointer",
          )}
          style={{ direction: "rtl", unicodeBidi: "plaintext" as const }}
        >
          {fileLabel}
        </button>
      )}

      {/* Entity name — regular weight, takes remaining space */}
      <span
        className="text-xs text-text truncate flex-1 font-mono"
        title={name}
      >
        {name}
      </span>
    </div>
  );
}
