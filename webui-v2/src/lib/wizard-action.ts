/* wizard-action.ts — action-first wizard logic (#5336), shared with the CLI.
 *
 * The dashboard scan wizard mirrors the CLI's four-action first step:
 *   single   — index one repository
 *   group    — index a group of related repositories
 *   monorepo — index a monorepo's packages
 *   add-group — add a repository to an existing group
 *
 * The "detect" reply from POST /api/v2/scan/inspect carries a `suggestedAction`
 * computed by the SAME Go classifier (detect.ClassifyPath) the CLI uses, plus
 * the child git repos / sibling git repos / packages it found. These pure
 * helpers turn that reply into UI defaults so the two surfaces agree. They are
 * dependency-free and unit-tested (the component itself isn't covered by the
 * src/lib vitest scope). */

import type { ScanInspectReply } from "@/data/types";

export type WizardAction = "single" | "group" | "monorepo" | "add-group";

/** The four actions, in display order, with their labels (parity with the CLI). */
export const WIZARD_ACTIONS: { value: WizardAction; label: string }[] = [
  { value: "single", label: "Index a single repository" },
  { value: "group", label: "Index a group of related repositories" },
  { value: "monorepo", label: "Index a monorepo" },
  { value: "add-group", label: "Add a repository to an existing group" },
];

/**
 * defaultActionFor maps a scan reply's `suggestedAction` to the wizard's default
 * action. Falls back to "single" when unknown/empty — matching the CLI's
 * defaultAction(). "add-group" is never auto-suggested (it depends on existing
 * groups, a user intent rather than a folder property).
 */
export function defaultActionFor(scan: ScanInspectReply | null): WizardAction {
  switch (scan?.suggestedAction) {
    case "group":
      return "group";
    case "monorepo":
      return "monorepo";
    case "single":
      return "single";
    default:
      return "single";
  }
}

/**
 * groupCandidatesFor derives the candidate group member repos from a scan reply,
 * as RELATIVE names (matching how the checkbox list renders childGitRepos):
 *   - child git repos when present (the ivivo case: backend + frontend), else
 *   - the repo itself ("." ) plus its sibling basenames when absPath is a repo,
 *   - otherwise empty.
 *
 * This is the data the "group" action presents as a multiselect — the same
 * precedence (option 1a) the CLI's groupCandidates() uses.
 */
export function groupCandidatesFor(scan: ScanInspectReply | null): string[] {
  if (!scan?.valid) return [];
  if ((scan.childGitRepos?.length ?? 0) > 0) {
    return [...scan.childGitRepos].sort();
  }
  if (scan.isGitRepo) {
    const siblings = (scan.siblingGitRepos ?? []).map(basename);
    return [basename(scan.absPath), ...siblings].sort();
  }
  return [];
}

function basename(p: string): string {
  const parts = p.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || p;
}
