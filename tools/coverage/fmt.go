package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

// cmdFmt rewrites docs/coverage/registry.json in the canonical
// (2-space-indented, sorted) form produced by saveRegistry. It is the
// safety net against whole-file re-serialization: a hand-edit of a
// `framework_specific` block that accidentally recompacts the file (or
// a JSON formatter run over it) is undone by `fmt`, leaving only the
// intended semantic change in `git diff`.
//
// With --check it writes nothing and exits non-zero when the on-disk
// bytes differ from canonical — this is the CI guard (see
// .github/workflows/coverage-docs.yml) that stops a recompacted
// registry from landing (regression: #2907).
func cmdFmt(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	path := registryFlag(fs)
	check := fs.Bool("check", false, "verify the file is already canonical; write nothing, exit non-zero if not")
	if err := fs.Parse(args); err != nil {
		return err
	}
	reg, err := loadRegistry(*path)
	if err != nil {
		return err
	}
	// Mirror saveRegistry exactly: canonical form is sorted-then-marshalled.
	// Without this sort, --check would miss cite-order drift (a cell updated
	// by `update` re-sorts its cites, so a registry with unsorted cites is not
	// canonical even though its indentation is fine).
	sortRegistry(reg)
	want, err := marshalRegistry(reg)
	if err != nil {
		return err
	}
	if *check {
		got, err := os.ReadFile(*path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			return fmt.Errorf("%s is not canonical — run 'go run ./tools/coverage fmt' and commit", *path)
		}
		fmt.Fprintf(out, "ok: %s is canonical\n", *path)
		return nil
	}
	if err := saveRegistry(*path, reg); err != nil {
		return err
	}
	fmt.Fprintf(out, "formatted %s\n", *path)
	return nil
}
