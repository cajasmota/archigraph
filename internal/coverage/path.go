package coverage

import (
	"path"
	"strings"
)

// Normalize turns an LCOV SF: path into a repo-relative, forward-slash path so
// it can be joined against an entity's source_file.
//
// LCOV paths come in three flavors depending on the tool and CWD at run time:
//   - already repo-relative ("src/foo.ts")
//   - workspace-absolute ("/home/ci/project/src/foo.ts")
//   - prefixed with a build/monorepo root ("packages/web/src/foo.ts")
//
// rootPrefix is an optional, configurable prefix (mirroring the directory the
// coverage tool treated as its base) that is stripped when present. Leading
// "./" and a leading slash are always removed, and backslashes are converted to
// forward slashes so Windows-emitted reports normalize cleanly.
func Normalize(raw, rootPrefix string) string {
	p := strings.ReplaceAll(raw, "\\", "/")
	p = strings.TrimSpace(p)

	rootPrefix = strings.ReplaceAll(rootPrefix, "\\", "/")
	rootPrefix = strings.Trim(rootPrefix, "/")
	if rootPrefix != "" {
		// Strip "<rootPrefix>/" anywhere it occurs as a path boundary, but
		// most commonly as an absolute or leading-relative prefix.
		if idx := strings.Index(p, "/"+rootPrefix+"/"); idx >= 0 {
			p = p[idx+len(rootPrefix)+2:]
		} else if strings.HasPrefix(p, rootPrefix+"/") {
			p = strings.TrimPrefix(p, rootPrefix+"/")
		}
	}

	// Drop leading "./" and absolute leading slash.
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")

	// path.Clean collapses "a/./b" and "a/../b" and removes a trailing slash.
	p = path.Clean(p)
	if p == "." {
		return ""
	}
	return p
}

// samePath reports whether two paths refer to the same file after a tolerant
// normalization (slash direction, leading "./", surrounding whitespace). Used
// to join an attribution key against an entity's source_file when both have
// already been Normalize-d but may differ in leading-segment depth.
func samePath(a, b string) bool {
	a = strings.TrimPrefix(path.Clean(strings.ReplaceAll(a, "\\", "/")), "./")
	b = strings.TrimPrefix(path.Clean(strings.ReplaceAll(b, "\\", "/")), "./")
	if a == b {
		return true
	}
	// Tolerate one being a tail of the other (e.g. report has a deeper root
	// prefix than the entity's stored source_file, or vice-versa).
	return strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a)
}
