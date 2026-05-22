/* ============================================================
   TopBar — per-project header. Single row. 56px tall.
   (prototype `.ag-topbar`)

   Left: breadcrumb — archigraph › <group> › <surface>.
   Right: PROJECT switcher — shows the current project name; clicking
          (or ⌘K) opens the list of indexed projects. Selecting one
          switches the current project, keeping the current screen
          when possible (#1572). It switches PROJECTS only.

   The per-screen nav lives in the LEFT SIDEBAR (chrome/nav-rail.tsx).
   NAVIGATION ONLY — no numeric scope counts here (handoff rule).
   ============================================================ */

import { ChevronRight, ChevronsUpDown } from "lucide-react";
import { useParams } from "react-router-dom";
import { Kbd } from "@/components/ui";
import { useAppStore } from "@/store/use-app-store";
import { useGroups } from "@/hooks/use-groups";

export interface TopBarProps {
  group: string;
  surfaceLabel: string;
}

export function TopBar({ group, surfaceLabel }: TopBarProps) {
  const { groupId } = useParams<{ groupId: string }>();
  const setCommandOpen = useAppStore((s) => s.setCommandOpen);
  const { data: groups = [] } = useGroups();

  const current = groups.find((g) => g.id === (groupId ?? group));
  const projectName = current?.name ?? group;

  return (
    <header className="flex items-center justify-between h-14 shrink-0 px-4 border-b border-border bg-bg">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-md">
        <span className="text-text-3">archigraph</span>
        <ChevronRight size={12} className="text-text-4" />
        <span className="font-mono text-text-2">{group}</span>
        <ChevronRight size={12} className="text-text-4" />
        <span className="font-medium text-text">{surfaceLabel}</span>
      </nav>

      <button
        onClick={() => setCommandOpen(true)}
        aria-label="Switch project"
        className="inline-flex items-center gap-2 h-8 pl-3 pr-2 rounded-md border border-border bg-surface text-text-2 text-md hover:bg-surface-2 transition-colors max-w-[260px]"
      >
        <span className="size-2 rounded-full bg-accent shrink-0" aria-hidden />
        <span className="font-medium text-text truncate">{projectName}</span>
        <span className="text-text-4">·</span>
        <Kbd>⌘K</Kbd>
        <ChevronsUpDown size={13} className="text-text-4 shrink-0" />
      </button>
    </header>
  );
}
