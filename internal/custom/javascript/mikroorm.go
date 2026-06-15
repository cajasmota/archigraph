package javascript

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extreg.Register("custom_js_mikroorm", &mikroORMExtractor{})
}

type mikroORMExtractor struct{}

func (e *mikroORMExtractor) Language() string { return "custom_js_mikroorm" }

var (
	// @Entity() class User {} — also @Entity({ tableName: 'users' }).
	reMikroEntity = regexp.MustCompile(
		`@Entity\s*\([^@]*?\)\s*(?:export\s+)?(?:abstract\s+)?class\s+([A-Z][A-Za-z0-9_]*)`,
	)
	// @Embeddable() class Address {}
	reMikroEmbeddable = regexp.MustCompile(
		`@Embeddable\s*\(\s*\)\s*(?:export\s+)?class\s+([A-Z][A-Za-z0-9_]*)`,
	)
	// @Property() / @PrimaryKey() / @Enum() field declarations.
	reMikroProperty = regexp.MustCompile(
		`@(Property|PrimaryKey|Enum|Unique|Formula)\s*\([^@]*?\)\s+(\w+)`,
	)
	// @ManyToOne / @OneToMany / @OneToOne / @ManyToMany relations.
	reMikroRelation = regexp.MustCompile(
		`@(ManyToOne|OneToMany|OneToOne|ManyToMany)\s*\([^@]*?\)\s+(\w+)`,
	)
	// The `() => Target` / `entity: () => Target` thunk inside a MikroORM relation
	// decorator names the related entity class (group 1). Issue #4365.
	reMikroRelationTarget = regexp.MustCompile(
		`=>\s*([A-Z][A-Za-z0-9_]*)`,
	)
	// Relation decorators with lazy: true OR LoadStrategy.LAZY/EXTRA_LAZY in options.
	// Issue #3071 — lazy_loading_recognition for MikroORM.
	reMikroLazyRelation = regexp.MustCompile(
		`@(ManyToOne|OneToMany|OneToOne|ManyToMany)\s*\(([^@]*?(?:lazy\s*:\s*true|LoadStrategy\.(?:LAZY|EXTRA_LAZY))[^@]*?)\)\s+(\w+)`,
	)
	// MikroORM migration class: class Migration20240101 extends Migration {}
	reMikroMigrationClass = regexp.MustCompile(
		`(?:export\s+)?class\s+([A-Za-z0-9_]+)\s+extends\s+Migration\b`,
	)
	// this.addSql('...') / this.addSql("...") / this.addSql(`...`) — raw SQL DDL
	// inside a migration. Each alternative captures the body up to the matching
	// closing quote so SQL containing the other quote styles is preserved.
	reMikroAddSql = regexp.MustCompile(
		"this\\s*\\.\\s*addSql\\s*\\(\\s*(?:'([^']*)'|\"([^\"]*)\"|`([^`]*)`)",
	)
)

func (e *mikroORMExtractor) Extract(ctx context.Context, file extreg.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("grafel/custom/javascript")
	_, span := tracer.Start(ctx, "indexer.mikroorm_extractor.extract",
		trace.WithAttributes(
			attribute.String("language", file.Language),
			attribute.String("framework", "mikro-orm"),
			attribute.String("file_path", file.Path),
		),
	)
	defer span.End()

	if len(file.Content) == 0 {
		return nil, nil
	}
	src := string(file.Content)
	lang := strings.ToLower(file.Language)
	if lang != "typescript" && lang != "javascript" {
		return nil, nil
	}

	var entities []types.EntityRecord
	seen := make(map[string]bool)
	addEntity := func(ent types.EntityRecord) {
		key := fmt.Sprintf("%s:%s:%s", ent.Kind, ent.Name, ent.Subtype)
		if seen[key] {
			return
		}
		seen[key] = true
		entities = append(entities, ent)
	}

	// @Entity / @Embeddable classes. Track each class's byte offset and its index
	// in the entities slice so @Property / relation fields below can hang a
	// CONTAINS membership edge off the owning class node (issue #4365) — the same
	// fix #4328 applied to TypeORM/mongoose, generalised to MikroORM's decorator
	// form.
	type ownerInfo struct {
		name string
		off  int
		idx  int
	}
	var owners []ownerInfo

	// @Entity classes.
	for _, m := range reMikroEntity.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Schema", "entity", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "mikro-orm", "provenance", "INFERRED_FROM_MIKROORM_ENTITY")
		if !seen[fmt.Sprintf("%s:%s:%s", ent.Kind, ent.Name, ent.Subtype)] {
			owners = append(owners, ownerInfo{name: name, off: m[0], idx: len(entities)})
		}
		addEntity(ent)
	}

	// @Embeddable classes.
	for _, m := range reMikroEmbeddable.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Schema", "embeddable", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "mikro-orm", "provenance", "INFERRED_FROM_MIKROORM_EMBEDDABLE")
		if !seen[fmt.Sprintf("%s:%s:%s", ent.Kind, ent.Name, ent.Subtype)] {
			owners = append(owners, ownerInfo{name: name, off: m[0], idx: len(entities)})
		}
		addEntity(ent)
	}

	// owningEntity returns the @Entity/@Embeddable class whose declaration most
	// closely precedes a body offset, and whether one was found.
	owningEntity := func(offset int) (ownerInfo, bool) {
		best := ownerInfo{idx: -1}
		found := false
		for _, o := range owners {
			if o.off <= offset {
				best = o
				found = true
			}
		}
		return best, found
	}

	// @Property / @PrimaryKey / @Enum fields. Each field becomes a CONTAINS member
	// of its owning @Entity (issue #4365) so it is not an orphan.
	for _, m := range reMikroProperty.FindAllStringSubmatchIndex(src, -1) {
		decorator := src[m[2]:m[3]]
		fieldName := src[m[4]:m[5]]
		ent := makeEntity(fieldName, "SCOPE.Component", "field", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "mikro-orm", "decorator", decorator, "field_name", fieldName,
			"provenance", "INFERRED_FROM_MIKROORM_PROPERTY")
		if owner, ok := owningEntity(m[0]); ok && owner.idx >= 0 {
			setProps(&ent, "owner_class", owner.name)
			entities[owner.idx].Relationships = append(entities[owner.idx].Relationships,
				containsFieldEdge(owner.name, ent.ID, fieldName, "mikro-orm"))
		}
		addEntity(ent)
	}

	// Relations. The relation field is a CONTAINS member of its owning entity, and
	// the `() => Target` thunk class is captured as a REFERENCES edge (issue #4365).
	for _, m := range reMikroRelation.FindAllStringSubmatchIndex(src, -1) {
		relType := src[m[2]:m[3]]
		fieldName := src[m[4]:m[5]]
		// Decorator args blob spans the matched decorator up to the field name; the
		// `() => Target` arrow inside names the related entity class.
		argsBlob := src[m[0]:m[5]]
		target := ""
		if tm := reMikroRelationTarget.FindStringSubmatch(argsBlob); tm != nil {
			target = tm[1]
		}
		ent := makeEntity(relType+":"+fieldName, "SCOPE.Component", "relation", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "mikro-orm", "relation_type", relType, "field_name", fieldName,
			"provenance", "INFERRED_FROM_MIKROORM_RELATION")
		if target != "" {
			setProps(&ent, "target_entity", target)
			ent.Relationships = append(ent.Relationships,
				referencesClassEdge(ent.ID, target, "mikro-orm", fieldName))
		}
		if owner, ok := owningEntity(m[0]); ok && owner.idx >= 0 {
			setProps(&ent, "owner_class", owner.name)
			entities[owner.idx].Relationships = append(entities[owner.idx].Relationships,
				containsFieldEdge(owner.name, ent.ID, fieldName, "mikro-orm"))
		}
		addEntity(ent)
	}

	// Lazy relations: relation decorator with lazy: true or LoadStrategy.LAZY/EXTRA_LAZY.
	// Issue #3071 — lazy_loading_recognition for MikroORM.
	for _, m := range reMikroLazyRelation.FindAllStringSubmatchIndex(src, -1) {
		relType := src[m[2]:m[3]]
		opts := src[m[4]:m[5]]
		fieldName := src[m[6]:m[7]]
		strategy := "lazy"
		if strings.Contains(opts, "EXTRA_LAZY") {
			strategy = "extra_lazy"
		} else if strings.Contains(opts, "LoadStrategy") {
			strategy = "lazy"
		}
		ent := makeEntity("lazy:"+relType+":"+fieldName, "SCOPE.Pattern", "lazy_relation", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "mikro-orm", "relation_type", relType, "field_name", fieldName,
			"lazy_loading", strategy, "provenance", "INFERRED_FROM_MIKROORM_LAZY_RELATION")
		addEntity(ent)
	}

	// Migration classes.
	isMigration := false
	for _, m := range reMikroMigrationClass.FindAllStringSubmatchIndex(src, -1) {
		isMigration = true
		name := src[m[2]:m[3]]
		ent := makeEntity(name, "SCOPE.Operation", "migration", file.Path, file.Language, lineOf(src, m[0]))
		setProps(&ent, "framework", "mikro-orm", "provenance", "INFERRED_FROM_MIKROORM_MIGRATION")
		addEntity(ent)
	}

	// addSql() raw DDL — only treat as migration ops within a migration class.
	if isMigration {
		for _, m := range reMikroAddSql.FindAllStringSubmatchIndex(src, -1) {
			sqlText := ""
			for g := 2; g+1 < len(m); g += 2 {
				if m[g] >= 0 {
					sqlText = src[m[g]:m[g+1]]
					break
				}
			}
			off := m[0]
			emit := func(subtype, target string) {
				ent := makeEntity(subtype+":"+target, "SCOPE.Evolution", subtype, file.Path, file.Language, lineOf(src, off))
				setProps(&ent, "framework", "mikro-orm", "table", target,
					"provenance", "INFERRED_FROM_MIKROORM_ADDSQL")
				addEntity(ent)
			}
			if cm := reSQLCreateTable.FindStringSubmatch(sqlText); cm != nil {
				emit("create_table", cm[1])
			}
			if cm := reSQLDropTable.FindStringSubmatch(sqlText); cm != nil {
				emit("drop_table", cm[1])
			}
			if cm := reSQLAlterTable.FindStringSubmatch(sqlText); cm != nil {
				emit(alterTableOpSubtype(cm[2]), cm[1])
			}
			if cm := reSQLCreateIndex.FindStringSubmatch(sqlText); cm != nil {
				emit("create_index", cm[1])
			}
		}
	}

	span.SetAttributes(attribute.Int("entity_count", len(entities)))
	return entities, nil
}
