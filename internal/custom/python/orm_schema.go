package python

// orm_schema.go — field-level (schema) extraction for Peewee, Pony ORM,
// Beanie, MongoEngine, and Tortoise ORM.
//
// Issue #3072 — ORM schema extraction for peewee/pony/beanie/mongoengine/tortoise.
// Pattern: mirrors the SQLAlchemy class-body scanner in sqlalchemy.go;
// each extractor detects model class declarations then scans the class body
// for field/attribute/column definitions and emits SCOPE.Schema entities.

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func init() {
	extractor.Register("python_peewee_schema", &PeeweeSchemaExtractor{})
	extractor.Register("python_pony_schema", &PonySchemaExtractor{})
	extractor.Register("python_beanie_schema", &BeanieSchemaExtractor{})
	extractor.Register("python_mongoengine_schema", &MongoEngineSchemaExtractor{})
	extractor.Register("python_tortoise_schema", &TortoiseSchemaExtractor{})
}

// ============================================================================
// Peewee — field-level extraction
// ============================================================================

// PeeweeSchemaExtractor emits SCOPE.Schema entities for field assignments
// inside Peewee Model subclasses.
// e.g.  name = CharField(max_length=100)
type PeeweeSchemaExtractor struct{}

func (e *PeeweeSchemaExtractor) Language() string { return "python_peewee_schema" }

var (
	// peeweeSchemaModelRe matches a Peewee model class declaration.
	peeweeSchemaModelRe = regexp.MustCompile(
		`(?m)^class\s+([A-Z][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*:`)

	// peeweeFieldRe matches field assignments in the class body:
	//   name = CharField(...)
	//   age  = IntegerField()
	// Captures: (attr_name, FieldTypeName)
	peeweeFieldRe = regexp.MustCompile(
		`(?m)^\s+(\w+)\s*=\s*([A-Za-z_][A-Za-z0-9_]*Field)\s*\(`)

	// peeweeSchemaBaseIndicators are common Peewee base class names.
	peeweeSchemaBaseIndicators = []string{"Model", "peewee.Model", "pw.Model"}
)

func isPeeweeSchemaModel(bases string) bool {
	for _, ind := range peeweeSchemaBaseIndicators {
		if strings.Contains(bases, ind) {
			return true
		}
	}
	return false
}

func (e *PeeweeSchemaExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("custom.python_peewee_schema")
	_, span := tracer.Start(ctx, "custom.python_peewee_schema")
	defer span.End()
	span.SetAttributes(attribute.String("file", file.Path))

	if len(file.Content) == 0 {
		return nil, nil
	}
	source := string(file.Content)
	if !strings.Contains(source, "peewee") {
		return nil, nil
	}

	var out []types.EntityRecord

	for _, idx := range allMatchesIndex(peeweeSchemaModelRe, source) {
		className := source[idx[2]:idx[3]]
		bases := source[idx[4]:idx[5]]
		if !isPeeweeSchemaModel(bases) {
			continue
		}

		classLine := lineOf(source, idx[0])
		body := extractClassBody(source, idx[0])

		// Emit the model entity
		modelEnt := entity(className, "SCOPE.Schema", "", file.Path, classLine,
			map[string]string{"framework": "peewee", "pattern_type": "model", "class_name": className})

		// Emit each field entity
		for _, fIdx := range allMatchesIndex(peeweeFieldRe, body) {
			attrName := body[fIdx[2]:fIdx[3]]
			fieldType := body[fIdx[4]:fIdx[5]]
			fieldLine := classLine + strings.Count(body[:fIdx[0]], "\n")
			out = append(out, entity(className+"."+attrName, "SCOPE.Schema", "column", file.Path, fieldLine,
				map[string]string{
					"framework":    "peewee",
					"pattern_type": "field",
					"field_type":   fieldType,
					"parent_class": className,
				}))
			// Issue #4366 — column membership.
			modelEnt.Relationships = append(modelEnt.Relationships,
				containsFieldEdge(className, className+"."+attrName, attrName, "peewee"))
		}
		out = append(out, modelEnt)
	}

	span.SetAttributes(attribute.Int("entity_count", len(out)))
	return out, nil
}

// ============================================================================
// Pony ORM — attribute-level extraction
// ============================================================================

// PonySchemaExtractor emits SCOPE.Schema entities for attribute definitions
// inside Pony db.Entity subclasses.
// e.g.  name = Required(str)   or   age = Optional(int, default=0)
type PonySchemaExtractor struct{}

func (e *PonySchemaExtractor) Language() string { return "python_pony_schema" }

var (
	// ponySchemaEntityRe matches a Pony entity class declaration.
	ponySchemaEntityRe = regexp.MustCompile(
		`(?m)^class\s+([A-Z][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*:`)

	// ponyAttrRe matches scalar attribute assignments in the class body:
	//   name     = Required(str)
	//   age      = Optional(int, default=0)
	//   created  = Optional(datetime)
	// Captures: (attr_name, Pony_descriptor)
	ponyAttrRe = regexp.MustCompile(
		`(?m)^\s+(\w+)\s*=\s*(Required|Optional|PrimaryKey|Set|Discriminator)\s*\(`)
)

// isPonyEntity is defined in orm_relationships.go (shared with the relationship extractor).

func (e *PonySchemaExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("custom.python_pony_schema")
	_, span := tracer.Start(ctx, "custom.python_pony_schema")
	defer span.End()
	span.SetAttributes(attribute.String("file", file.Path))

	if len(file.Content) == 0 {
		return nil, nil
	}
	source := string(file.Content)
	if !strings.Contains(source, "pony") && !strings.Contains(source, "ponyorm") {
		return nil, nil
	}

	var out []types.EntityRecord

	for _, idx := range allMatchesIndex(ponySchemaEntityRe, source) {
		className := source[idx[2]:idx[3]]
		bases := source[idx[4]:idx[5]]
		if !isPonyEntity(bases) {
			continue
		}

		classLine := lineOf(source, idx[0])
		body := extractClassBody(source, idx[0])

		// Emit the entity model
		modelEnt := entity(className, "SCOPE.Schema", "", file.Path, classLine,
			map[string]string{"framework": "pony", "pattern_type": "entity", "class_name": className})

		// Emit each attribute entity
		for _, aIdx := range allMatchesIndex(ponyAttrRe, body) {
			attrName := body[aIdx[2]:aIdx[3]]
			descriptor := body[aIdx[4]:aIdx[5]]
			attrLine := classLine + strings.Count(body[:aIdx[0]], "\n")
			out = append(out, entity(className+"."+attrName, "SCOPE.Schema", "column", file.Path, attrLine,
				map[string]string{
					"framework":    "pony",
					"pattern_type": "attribute",
					"descriptor":   descriptor,
					"parent_class": className,
				}))
			// Issue #4366 — attribute membership.
			modelEnt.Relationships = append(modelEnt.Relationships,
				containsFieldEdge(className, className+"."+attrName, attrName, "pony"))
		}
		out = append(out, modelEnt)
	}

	span.SetAttributes(attribute.Int("entity_count", len(out)))
	return out, nil
}

// ============================================================================
// Beanie — document field extraction
// ============================================================================

// BeanieSchemaExtractor emits SCOPE.Schema entities for annotated field
// definitions inside Beanie Document subclasses.
// Beanie uses Pydantic-style annotations:  name: str   or   age: Optional[int]
type BeanieSchemaExtractor struct{}

func (e *BeanieSchemaExtractor) Language() string { return "python_beanie_schema" }

var (
	// beanieDocumentRe matches a Beanie Document class declaration.
	beanieDocumentRe = regexp.MustCompile(
		`(?m)^class\s+([A-Z][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*:`)

	// beanieFieldRe matches Pydantic-style annotated field declarations:
	//   name: str
	//   age: Optional[int] = None
	//   tags: List[str] = []
	// Captures: (attr_name, type_annotation_prefix)
	beanieFieldRe = regexp.MustCompile(
		`(?m)^\s{4}(\w+)\s*:\s*([A-Za-z_][A-Za-z0-9_\[\], |]*)`)
)

// isBeanieDocument is defined in orm_relationships.go (shared with the relationship extractor).

func (e *BeanieSchemaExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("custom.python_beanie_schema")
	_, span := tracer.Start(ctx, "custom.python_beanie_schema")
	defer span.End()
	span.SetAttributes(attribute.String("file", file.Path))

	if len(file.Content) == 0 {
		return nil, nil
	}
	source := string(file.Content)
	if !strings.Contains(source, "beanie") {
		return nil, nil
	}

	var out []types.EntityRecord

	for _, idx := range allMatchesIndex(beanieDocumentRe, source) {
		className := source[idx[2]:idx[3]]
		bases := source[idx[4]:idx[5]]
		if !isBeanieDocument(bases) {
			continue
		}

		classLine := lineOf(source, idx[0])
		body := extractClassBody(source, idx[0])

		// Emit the document model entity
		modelEnt := entity(className, "SCOPE.Schema", "", file.Path, classLine,
			map[string]string{"framework": "beanie", "pattern_type": "document", "class_name": className})

		// Emit each field entity (annotated attributes, skip dunder / class vars)
		for _, fIdx := range allMatchesIndex(beanieFieldRe, body) {
			attrName := body[fIdx[2]:fIdx[3]]
			// Skip dunder attributes and common Pydantic internals
			if strings.HasPrefix(attrName, "_") || attrName == "model_config" || attrName == "Settings" {
				continue
			}
			typeAnnotation := strings.TrimSpace(body[fIdx[4]:fIdx[5]])
			fieldLine := classLine + strings.Count(body[:fIdx[0]], "\n")
			out = append(out, entity(className+"."+attrName, "SCOPE.Schema", "column", file.Path, fieldLine,
				map[string]string{
					"framework":       "beanie",
					"pattern_type":    "field",
					"type_annotation": typeAnnotation,
					"parent_class":    className,
				}))
			// Issue #4366 — field membership.
			modelEnt.Relationships = append(modelEnt.Relationships,
				containsFieldEdge(className, className+"."+attrName, attrName, "beanie"))
		}
		out = append(out, modelEnt)
	}

	span.SetAttributes(attribute.Int("entity_count", len(out)))
	return out, nil
}

// ============================================================================
// MongoEngine — document field extraction
// ============================================================================

// MongoEngineSchemaExtractor emits SCOPE.Schema entities for field assignments
// inside MongoEngine Document/EmbeddedDocument subclasses.
// e.g.  name = StringField(required=True)
type MongoEngineSchemaExtractor struct{}

func (e *MongoEngineSchemaExtractor) Language() string { return "python_mongoengine_schema" }

var (
	// mongoengineDocumentRe matches a MongoEngine Document class declaration.
	mongoengineDocumentRe = regexp.MustCompile(
		`(?m)^class\s+([A-Z][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*:`)

	// mongoengineFieldRe matches MongoEngine field assignments:
	//   name = StringField(required=True)
	//   age  = IntField(default=0)
	// Captures: (attr_name, FieldTypeName)
	mongoengineFieldRe = regexp.MustCompile(
		`(?m)^\s+(\w+)\s*=\s*([A-Za-z_][A-Za-z0-9_]*Field)\s*\(`)
)

// isMongoEngineDoc is defined in orm_relationships.go (shared with the relationship extractor).

func (e *MongoEngineSchemaExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("custom.python_mongoengine_schema")
	_, span := tracer.Start(ctx, "custom.python_mongoengine_schema")
	defer span.End()
	span.SetAttributes(attribute.String("file", file.Path))

	if len(file.Content) == 0 {
		return nil, nil
	}
	source := string(file.Content)
	if !strings.Contains(source, "mongoengine") {
		return nil, nil
	}

	var out []types.EntityRecord

	for _, idx := range allMatchesIndex(mongoengineDocumentRe, source) {
		className := source[idx[2]:idx[3]]
		bases := source[idx[4]:idx[5]]
		if !isMongoEngineDoc(bases) {
			continue
		}

		classLine := lineOf(source, idx[0])
		body := extractClassBody(source, idx[0])

		// Emit the document entity
		modelEnt := entity(className, "SCOPE.Schema", "", file.Path, classLine,
			map[string]string{"framework": "mongoengine", "pattern_type": "document", "class_name": className})

		// Emit each field entity
		for _, fIdx := range allMatchesIndex(mongoengineFieldRe, body) {
			attrName := body[fIdx[2]:fIdx[3]]
			fieldType := body[fIdx[4]:fIdx[5]]
			fieldLine := classLine + strings.Count(body[:fIdx[0]], "\n")
			out = append(out, entity(className+"."+attrName, "SCOPE.Schema", "column", file.Path, fieldLine,
				map[string]string{
					"framework":    "mongoengine",
					"pattern_type": "field",
					"field_type":   fieldType,
					"parent_class": className,
				}))
			// Issue #4366 — field membership.
			modelEnt.Relationships = append(modelEnt.Relationships,
				containsFieldEdge(className, className+"."+attrName, attrName, "mongoengine"))
		}
		out = append(out, modelEnt)
	}

	span.SetAttributes(attribute.Int("entity_count", len(out)))
	return out, nil
}

// ============================================================================
// Tortoise ORM — column-level extraction
// ============================================================================

// TortoiseSchemaExtractor emits SCOPE.Schema entities for field assignments
// inside Tortoise Model subclasses.
// e.g.  name = fields.CharField(max_length=255)
type TortoiseSchemaExtractor struct{}

func (e *TortoiseSchemaExtractor) Language() string { return "python_tortoise_schema" }

var (
	// tortoiseSchemaModelRe matches a Tortoise model class declaration.
	tortoiseSchemaModelRe = regexp.MustCompile(
		`(?m)^class\s+([A-Z][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*:`)

	// tortoiseFieldRe matches Tortoise field assignments:
	//   name    = fields.CharField(max_length=255)
	//   age     = fields.IntField()
	//   created = fields.DatetimeField(auto_now_add=True)
	// Captures: (attr_name, field_type_path)  e.g. "fields.CharField"
	tortoiseFieldRe = regexp.MustCompile(
		`(?m)^\s+(\w+)\s*=\s*((?:fields|tortoise\.fields)\.[A-Za-z_][A-Za-z0-9_]*(?:Field|field))\s*\(`)
)

// isTortoiseModel is defined in orm_relationships.go (shared with the relationship extractor).

func (e *TortoiseSchemaExtractor) Extract(ctx context.Context, file extractor.FileInput) ([]types.EntityRecord, error) {
	tracer := otel.Tracer("custom.python_tortoise_schema")
	_, span := tracer.Start(ctx, "custom.python_tortoise_schema")
	defer span.End()
	span.SetAttributes(attribute.String("file", file.Path))

	if len(file.Content) == 0 {
		return nil, nil
	}
	source := string(file.Content)
	if !strings.Contains(source, "tortoise") {
		return nil, nil
	}

	var out []types.EntityRecord

	for _, idx := range allMatchesIndex(tortoiseSchemaModelRe, source) {
		className := source[idx[2]:idx[3]]
		bases := source[idx[4]:idx[5]]
		if !isTortoiseModel(bases) {
			continue
		}

		classLine := lineOf(source, idx[0])
		body := extractClassBody(source, idx[0])

		// Emit the model entity
		modelEnt := entity(className, "SCOPE.Schema", "", file.Path, classLine,
			map[string]string{"framework": "tortoise", "pattern_type": "model", "class_name": className})

		// Emit each column entity
		for _, fIdx := range allMatchesIndex(tortoiseFieldRe, body) {
			attrName := body[fIdx[2]:fIdx[3]]
			fieldType := body[fIdx[4]:fIdx[5]]
			fieldLine := classLine + strings.Count(body[:fIdx[0]], "\n")
			out = append(out, entity(className+"."+attrName, "SCOPE.Schema", "column", file.Path, fieldLine,
				map[string]string{
					"framework":    "tortoise",
					"pattern_type": "column",
					"field_type":   fieldType,
					"parent_class": className,
				}))
			// Issue #4366 — column membership.
			modelEnt.Relationships = append(modelEnt.Relationships,
				containsFieldEdge(className, className+"."+attrName, attrName, "tortoise"))
		}
		out = append(out, modelEnt)
	}

	span.SetAttributes(attribute.Int("entity_count", len(out)))
	return out, nil
}
