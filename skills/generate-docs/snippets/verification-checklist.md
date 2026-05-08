# Verification checklist

Run this checklist on every markdown file produced by any pass. The orchestrator re-runs it before accepting the pass output. A failed check is a hard stop — fix and re-run.

## Backtick discipline (ADR-0007)

- [ ] Every heading containing a CamelCase or `snake_case` word has that word in backticks.
- [ ] Every prose mention of a code identifier has it in backticks.
- [ ] Every file-path mention is in backticks (not a markdown link, unless the link target is a real doc page).

A useful regex to flag candidate violations (catches CamelCase or snake_case outside backticks in headings):

```bash
grep -nE '^#+ .*[^`]\b([A-Z][a-zA-Z0-9]*[a-z][a-zA-Z0-9]*[A-Z][a-zA-Z0-9]*|[a-z]+_[a-z_]+)\b' <file>
```

False positives are possible (acronyms, capitalized prose words) — review each match before "fixing". The check is "review, then decide", not blind regex replace.

## Code-block tags

- [ ] Every fenced code block has a language tag.
- [ ] Languages match the actual content (no `bash` blocks containing Python, etc.).

## Structural

- [ ] No empty sections — either fill them or delete them.
- [ ] No placeholder text remaining (`<one line>`, `<entity>`, `TODO`, `FIXME`).
- [ ] Tables have a header row + alignment row.
- [ ] Internal links use relative paths and resolve to existing files.

## Forbidden terms

- [ ] No mention of any prior tooling name. Only `archigraph` is the tool's name in the doc text.

## Source-snippet hygiene (Pass 4 mostly)

- [ ] Every quoted source snippet cites its file path in backticks.
- [ ] No snippet exceeds ~10 lines unless absolutely necessary.
- [ ] No snippet contains secrets, real customer data, or proprietary credentials.

## Cross-repo links (Pass 8 will validate fully)

- [ ] Every cross-repo link follows `cross-link-format.md`.
- [ ] Pending link candidates are marked clearly as unconfirmed.

## Plan adherence

- [ ] The file's section headings match the relevant `output-templates/*.md`.
- [ ] No drive-by additions of sections the template did not include.
