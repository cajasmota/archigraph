package enrichers

// MigrationSequenceEnricher parses migration filenames to add sequence metadata.
// Port of Python migration_sequence_enricher.py.

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// MigrationPattern identifies which filename convention was matched.
type MigrationPattern string

const (
	MigrationPatternRails         MigrationPattern = "rails"
	MigrationPatternDjango        MigrationPattern = "django"
	MigrationPatternFlyway        MigrationPattern = "flyway"
	MigrationPatternGolangMigrate MigrationPattern = "golang_migrate"
	MigrationPatternAlembic       MigrationPattern = "alembic"
	MigrationPatternUnknown       MigrationPattern = "unknown"
)

// MigrationAnnotation is parsed metadata for a single migration entity.
type MigrationAnnotation struct {
	EntityID       string
	SequenceNumber interface{}
	MigrationName  string
	PatternMatched MigrationPattern
}

// MigrationEntity is the input record for migration annotation.
type MigrationEntity struct {
	EntityID   string
	SourceFile string
}

var (
	railsMigrationRe   = regexp.MustCompile(`^(\d{14})_([^.]+)\.rb$`)
	djangoMigrationRe  = regexp.MustCompile(`^(\d{4})_([^.]+)\.py$`)
	flywayMigrationRe  = regexp.MustCompile(`^V(\d+(?:\.\d+)*)__([^.]+)\.sql$`)
	golangMigrateRe    = regexp.MustCompile(`^(\d{1,14})_([^.]+)\.(up|down)\.sql$`)
	alembicMigrationRe = regexp.MustCompile(`^([A-Za-z0-9]{12,})_([^.]+)\.py$`)
)

// alembicRevisionRe and alembicDownRevisionRe extract the module-level
// `revision` and `down_revision` string assignments from an Alembic migration
// file body. Alembic stores the DAG ordering in these variables (not in the
// filename): `down_revision = None` for the root migration, otherwise the id of
// the parent revision. We match both single- and double-quoted string literals
// and the `None` sentinel. Anchored with (?m) so they match a top-level
// assignment on its own line, tolerating leading whitespace.
var (
	alembicRevisionRe     = regexp.MustCompile(`(?m)^\s*revision\s*(?::[^=]*)?=\s*['"]([A-Za-z0-9_]+)['"]`)
	alembicDownRevisionRe = regexp.MustCompile(`(?m)^\s*down_revision\s*(?::[^=]*)?=\s*(?:['"]([A-Za-z0-9_]+)['"]|None)`)
)

// ParseAlembicRevisions extracts the (revision, downRevision) pair from an
// Alembic migration file's source. revision is the id this migration defines;
// downRevision is the parent it must run AFTER (empty string when the source
// declares `down_revision = None`, i.e. this is the base migration). Either
// return value is empty when the corresponding assignment is absent or
// unparseable. Pure: no I/O, deterministic.
func ParseAlembicRevisions(source string) (revision, downRevision string) {
	if m := alembicRevisionRe.FindStringSubmatch(source); m != nil {
		revision = m[1]
	}
	if m := alembicDownRevisionRe.FindStringSubmatch(source); m != nil {
		// m[1] is empty when the literal was `None`.
		downRevision = m[1]
	}
	return revision, downRevision
}

func parseMigrationFilename(basename string) (seq interface{}, name string, pattern MigrationPattern, ok bool) {
	if m := railsMigrationRe.FindStringSubmatch(basename); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, strings.ReplaceAll(m[2], "_", " "), MigrationPatternRails, true
	}
	if m := djangoMigrationRe.FindStringSubmatch(basename); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, strings.ReplaceAll(m[2], "_", " "), MigrationPatternDjango, true
	}
	if m := flywayMigrationRe.FindStringSubmatch(basename); m != nil {
		return m[1], strings.ReplaceAll(m[2], "_", " "), MigrationPatternFlyway, true
	}
	if m := golangMigrateRe.FindStringSubmatch(basename); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, strings.ReplaceAll(m[2], "_", " "), MigrationPatternGolangMigrate, true
	}
	if m := alembicMigrationRe.FindStringSubmatch(basename); m != nil {
		return m[1], strings.ReplaceAll(m[2], "_", " "), MigrationPatternAlembic, true
	}
	return nil, "", MigrationPatternUnknown, false
}

// AnnotateMigrationSequences parses migration filenames and returns annotations.
func AnnotateMigrationSequences(entities []MigrationEntity) ([]MigrationAnnotation, int) {
	var annotations []MigrationAnnotation
	unknownCount := 0
	for _, entity := range entities {
		if entity.SourceFile == "" {
			continue
		}
		basename := filepath.Base(entity.SourceFile)
		seq, name, pattern, ok := parseMigrationFilename(basename)
		if !ok {
			unknownCount++
			continue
		}
		annotations = append(annotations, MigrationAnnotation{
			EntityID:       entity.EntityID,
			SequenceNumber: seq,
			MigrationName:  name,
			PatternMatched: pattern,
		})
	}
	return annotations, unknownCount
}
