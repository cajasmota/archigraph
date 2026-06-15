package php

// driver_sql.go — schema_extraction for PHP relational raw-driver records:
// lang.php.driver.mysql, lang.php.driver.postgres, lang.php.driver.sqlite.
//
// Raw PHP SQL drivers (PDO, mysqli, pg_connect, SQLite3) embed schema DDL as
// string literals passed to execute/query calls. This extractor scans for
// CREATE TABLE statements in those literals and emits SCOPE.Schema entities
// for table + column definitions (heuristic → partial status).
//
// Coverage cells:
//   lang.php.driver.mysql    : schema_extraction → partial
//   lang.php.driver.postgres : schema_extraction → partial
//   lang.php.driver.sqlite   : schema_extraction → partial

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("custom_php_sql_driver_schema", &phpSQLDriverSchemaExtractor{})
}

type phpSQLDriverSchemaExtractor struct{}

func (e *phpSQLDriverSchemaExtractor) Language() string { return "custom_php_sql_driver_schema" }

var (
	// phpDriverGateRe gates the extractor: must contain a recognised PHP SQL
	// driver connection call so we don't misattribute ORM-generated DDL.
	phpDriverGateRe = regexp.MustCompile(
		`(?m)new\s+PDO\s*\(\s*['"](?:mysql|pgsql|sqlite):|` +
			`mysqli_connect\s*\(|new\s+mysqli\s*\(|` +
			`pg_connect\s*\(|new\s+SQLite3\s*\(`)

	// phpExecuteRe matches PDO/mysqli/pg_query execute calls whose first
	// argument starts a string literal.
	phpExecuteRe = regexp.MustCompile(
		`(?:->|::|_)(execute|query|exec|prepare|executemany)\s*\(\s*['"` + "`" + `]`)

	// phpCreateTableRe matches CREATE TABLE head + table name.
	phpCreateTableRe = regexp.MustCompile(
		`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?[` + "`" + `"']?([A-Za-z_][A-Za-z0-9_.]*)[` + "`" + `"']?\s*\(`)

	// phpSQLColumnRe matches a column definition line inside CREATE TABLE.
	phpSQLColumnRe = regexp.MustCompile(
		`(?im)^\s*[` + "`" + `"']?([A-Za-z_][A-Za-z0-9_]*)[` + "`" + `"']?\s+([A-Za-z][A-Za-z0-9_]*)`)

	// phpSQLConstraintLead skips table-level constraint lines.
	phpSQLConstraintLead = map[string]bool{
		"PRIMARY": true, "FOREIGN": true, "UNIQUE": true, "CONSTRAINT": true,
		"CHECK": true, "KEY": true, "INDEX": true, "FULLTEXT": true, "SPATIAL": true,
	}

	// phpDriverLabel maps gate pattern token to driver label.
	phpDriverPatterns = []struct {
		re    *regexp.Regexp
		label string
	}{
		{regexp.MustCompile(`(?m)new\s+PDO\s*\(\s*['"]mysql:|mysqli_connect|new\s+mysqli`), "mysql"},
		{regexp.MustCompile(`(?m)new\s+PDO\s*\(\s*['"]pgsql:|pg_connect`), "postgres"},
		{regexp.MustCompile(`(?m)new\s+PDO\s*\(\s*['"]sqlite:|new\s+SQLite3`), "sqlite"},
	}
)

func (e *phpSQLDriverSchemaExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/php")
	_, span := tracer.Start(ctx, "php_sql_driver_schema.extract",
		trace.WithAttributes(
			attribute.String("file", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 || file.Language != "php" {
		return nil, nil
	}

	src := string(file.Content)

	// Gate: must look like a raw SQL driver file
	if phpDriverGateRe.FindStringIndex(src) == nil {
		span.SetAttributes(attribute.Int("entity_count", 0))
		return nil, nil
	}

	// Determine which driver(s) are present
	driverLabel := "sql"
	for _, dp := range phpDriverPatterns {
		if dp.re.FindStringIndex(src) != nil {
			driverLabel = dp.label
			break
		}
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)
	add := func(ent types.EntityRecord) { ormAdd(seen, &entities, ent) }

	// Scan for CREATE TABLE statements in the source (they may be in string
	// literals or heredocs; we scan the full source text heuristically).
	for _, ctm := range phpCreateTableRe.FindAllStringSubmatchIndex(src, -1) {
		tableName := src[ctm[2]:ctm[3]]
		ln := lineOf(src, ctm[0])

		// Emit table-level schema entity
		ent := makeEntity(tableName, "SCOPE.Schema", "table", file.Path, file.Language, ln)
		setProps(&ent, "framework", driverLabel, "provenance", "INFERRED_FROM_PHP_SQL_CREATE_TABLE",
			"table_name", tableName)
		add(ent)

		// Extract columns from the DDL body (up to 4000 chars after CREATE TABLE)
		bodyStart := ctm[1]
		bodyEnd := bodyStart + 4000
		if bodyEnd > len(src) {
			bodyEnd = len(src)
		}
		body := src[bodyStart:bodyEnd]
		// Trim at closing paren of CREATE TABLE body
		if closeIdx := strings.Index(body, ");"); closeIdx != -1 {
			body = body[:closeIdx]
		}

		for _, colMatch := range phpSQLColumnRe.FindAllStringSubmatch(body, -1) {
			colName := colMatch[1]
			colType := colMatch[2]
			// Skip constraint lines
			upper := strings.ToUpper(colName)
			if phpSQLConstraintLead[upper] {
				continue
			}
			colEnt := makeEntity(tableName+"."+colName, "SCOPE.Schema", "column", file.Path, file.Language, ln)
			setProps(&colEnt, "framework", driverLabel, "provenance", "INFERRED_FROM_PHP_SQL_COLUMN",
				"table_name", tableName, "column_name", colName, "column_type", colType)
			add(colEnt)
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
