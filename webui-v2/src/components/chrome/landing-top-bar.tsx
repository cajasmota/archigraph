/* ============================================================
   LandingTopBar — minimal top bar for the Landing screen (no group
   context, so no breadcrumb / NavRail). Logo + wordmark on the left;
   theme toggle + info popover on the right.

   Built from primitives (Popover, Button) + the appearance store.
   ============================================================ */

import { Moon, Sun, Info, ExternalLink } from "lucide-react";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui";
import { useAppStore } from "@/store/use-app-store";

function Wordmark() {
  return (
    <div className="flex items-center gap-2">
      <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden>
        <defs>
          <linearGradient id="ag-lp-mark" x1="0" x2="1" y1="0" y2="1">
            <stop offset="0" stopColor="var(--accent)" />
            <stop offset="1" stopColor="var(--accent-strong)" />
          </linearGradient>
        </defs>
        <circle cx="6" cy="6" r="2.6" fill="url(#ag-lp-mark)" />
        <circle cx="18" cy="6" r="2.0" fill="var(--accent)" opacity=".7" />
        <circle cx="12" cy="18" r="2.6" fill="var(--accent-strong)" />
        <path d="M7.6 7.6l3 8M16 7.6l-3 8M8 6h8" stroke="var(--accent)" strokeWidth="1.4" fill="none" />
      </svg>
      <span className="font-mono text-md font-semibold text-text">Grafel</span>
    </div>
  );
}

const iconBtn =
  "inline-flex items-center justify-center size-8 rounded-md text-text-3 " +
  "hover:bg-surface-2 hover:text-text transition-colors " +
  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent-ring)]";

export interface LandingTopBarProps {
  /** Daemon version string for the info popover (from /api/v2/meta). */
  version?: string;
}

export function LandingTopBar({ version }: LandingTopBarProps) {
  const theme = useAppStore((s) => s.theme);
  const toggleTheme = useAppStore((s) => s.toggleTheme);

  return (
    <header className="flex items-center justify-between h-16 shrink-0 px-6 border-b border-border bg-bg">
      <Wordmark />
      <div className="flex items-center gap-1">
        <button
          type="button"
          className={iconBtn}
          onClick={toggleTheme}
          title={theme === "dark" ? "Light mode" : "Dark mode"}
          aria-label={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
        >
          {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
        </button>

        <Popover>
          <PopoverTrigger asChild>
            <button type="button" className={iconBtn} title="About Grafel" aria-label="About Grafel">
              <Info size={15} />
            </button>
          </PopoverTrigger>
          <PopoverContent align="end" className="w-64">
            <div className="flex items-center justify-between">
              <span className="font-semibold text-text">Grafel</span>
              <span className="font-mono text-sm text-text-3">{version ?? "—"}</span>
            </div>
            <p className="mt-1 text-sm text-text-3">Map your codebase, navigate any part.</p>
            <p className="mt-1 text-xs text-text-4 italic">Where grep gets lost, the Grafel shows the way.</p>
            <dl className="mt-3 space-y-1.5 text-sm">
              <div className="flex items-center justify-between">
                <dt className="text-text-3">Dashboard</dt>
                <dd className="font-mono text-text-2">{window.location.host}</dd>
              </div>
            </dl>
            <a
              href="https://github.com/cajasmota/grafel"
              target="_blank"
              rel="noreferrer"
              className="mt-3 inline-flex items-center gap-1.5 text-sm text-accent-strong hover:underline"
            >
              <ExternalLink size={12} />
              View on GitHub
            </a>
          </PopoverContent>
        </Popover>
      </div>
    </header>
  );
}
