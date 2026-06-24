package dart_test

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/dart"
)

// pents runs the persistence extractor and returns the entity records.
func pents(t *testing.T, path, src string) []types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get("custom_dart_persistence")
	if !ok {
		t.Fatal("custom_dart_persistence not registered")
	}
	ents, err := e.Extract(context.Background(), fi(path, "dart", src))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return ents
}

func findEnt(ents []types.EntityRecord, kind, subtype, name string) *types.EntityRecord {
	for i := range ents {
		if ents[i].Kind == kind && ents[i].Subtype == subtype && ents[i].Name == name {
			return &ents[i]
		}
	}
	return nil
}

// TestDartDrift proves a `class Foos extends Table { IntColumn get id => ... }`
// synthesises a drift table + typed columns with SQL types, the autoIncrement
// column is flagged primary_key, and @DriftDatabase emits a database node with
// REFERENCES edges to its tables.
func TestDartDrift(t *testing.T) {
	src := `
import 'package:drift/drift.dart';

class TodoItems extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get title => text().withLength(min: 1, max: 50)();
  BoolColumn get done => boolean().withDefault(const Constant(false))();
}

@DriftDatabase(tables: [TodoItems])
class AppDatabase extends _$AppDatabase {}
`
	ents := pents(t, "lib/db.dart", src)

	tbl := findEnt(ents, "SCOPE.Schema", "table", "TodoItems")
	if tbl == nil {
		t.Fatal("expected drift table TodoItems")
	}
	if tbl.Properties["framework"] != "drift" {
		t.Errorf("table framework = %q", tbl.Properties["framework"])
	}
	id := findEnt(ents, "SCOPE.Schema", "column", "id")
	if id == nil {
		t.Fatal("expected column id")
	}
	if id.Properties["sql_type"] != "INTEGER" {
		t.Errorf("id sql_type = %q, want INTEGER", id.Properties["sql_type"])
	}
	if id.Properties["primary_key"] != "true" {
		t.Errorf("id should be primary_key (autoIncrement)")
	}
	title := findEnt(ents, "SCOPE.Schema", "column", "title")
	if title == nil || title.Properties["sql_type"] != "TEXT" {
		t.Errorf("expected title TEXT column, got %+v", title)
	}
	db := findEnt(ents, "SCOPE.Schema", "database", "AppDatabase")
	if db == nil {
		t.Fatal("expected drift database AppDatabase")
	}
	foundRef := false
	for _, r := range db.Relationships {
		if r.Kind == "REFERENCES" && r.ToID == "TodoItems" {
			foundRef = true
		}
	}
	if !foundRef {
		t.Error("expected AppDatabase REFERENCES TodoItems")
	}
}

// TestDartFloor proves @entity/@dao/@Database produce floor model/dao/database
// schema entities and that the model's `final` fields become columns.
func TestDartFloor(t *testing.T) {
	src := `
import 'package:floor/floor.dart';

@entity
class Person {
  @primaryKey
  final int id;
  final String name;
}

@dao
abstract class PersonDao {
  @Query('SELECT * FROM Person')
  Future<List<Person>> findAll();
}

@Database(version: 1, entities: [Person])
abstract class AppDatabase extends FloorDatabase {}
`
	ents := pents(t, "lib/floor.dart", src)
	if findEnt(ents, "SCOPE.Schema", "model", "Person") == nil {
		t.Error("expected floor model Person")
	}
	if findEnt(ents, "SCOPE.Schema", "column", "name") == nil {
		t.Error("expected floor column name")
	}
	if findEnt(ents, "SCOPE.Schema", "dao", "PersonDao") == nil {
		t.Error("expected floor dao PersonDao")
	}
	if findEnt(ents, "SCOPE.Schema", "database", "AppDatabase") == nil {
		t.Error("expected floor database AppDatabase")
	}
}

// TestDartIsar proves an @collection class becomes an isar collection model,
// its fields become columns, and the Id field is flagged primary_key.
func TestDartIsar(t *testing.T) {
	src := `
import 'package:isar/isar.dart';

@collection
class User {
  Id id = Isar.autoIncrement;
  late String name;
  String? email;
}
`
	ents := pents(t, "lib/user.dart", src)
	coll := findEnt(ents, "SCOPE.Schema", "collection", "User")
	if coll == nil {
		t.Fatal("expected isar collection User")
	}
	if coll.Properties["store_kind"] != "nosql_embedded" {
		t.Errorf("User store_kind = %q", coll.Properties["store_kind"])
	}
	id := findEnt(ents, "SCOPE.Schema", "column", "id")
	if id == nil || id.Properties["primary_key"] != "true" {
		t.Errorf("expected isar id column flagged primary_key, got %+v", id)
	}
	if findEnt(ents, "SCOPE.Schema", "column", "name") == nil {
		t.Error("expected isar column name")
	}
}

// TestDartHive proves @HiveType class + @HiveField fields produce a hive model
// with the type_id and ordered fields carrying their hive_field ordinal.
func TestDartHive(t *testing.T) {
	src := `
import 'package:hive/hive.dart';

@HiveType(typeId: 3)
class Contact {
  @HiveField(0)
  final String name;
  @HiveField(1)
  final int age;
}
`
	ents := pents(t, "lib/contact.dart", src)
	m := findEnt(ents, "SCOPE.Schema", "model", "Contact")
	if m == nil {
		t.Fatal("expected hive model Contact")
	}
	if m.Properties["type_id"] != "3" {
		t.Errorf("Contact type_id = %q, want 3", m.Properties["type_id"])
	}
	name := findEnt(ents, "SCOPE.Schema", "field", "name")
	if name == nil {
		t.Fatal("expected hive field name")
	}
	if name.Properties["hive_field"] != "0" {
		t.Errorf("name hive_field = %q, want 0", name.Properties["hive_field"])
	}
	age := findEnt(ents, "SCOPE.Schema", "field", "age")
	if age == nil || age.Properties["hive_field"] != "1" {
		t.Errorf("expected hive field age ordinal 1, got %+v", age)
	}
}

// TestDartSqfliteRawSQL proves a CREATE TABLE in a sqflite db.execute(...) call
// is parsed into a table + columns with SQL types and primary-key flag
// (honest-partial raw-SQL path).
func TestDartSqfliteRawSQL(t *testing.T) {
	src := `
import 'package:sqflite/sqflite.dart';

Future<void> onCreate(Database db, int version) async {
  await db.execute('''
    CREATE TABLE products (
      id INTEGER PRIMARY KEY,
      name TEXT NOT NULL,
      price REAL
    );
  ''');
}
`
	ents := pents(t, "lib/store.dart", src)
	tbl := findEnt(ents, "SCOPE.Schema", "table", "products")
	if tbl == nil {
		t.Fatal("expected sqflite table products")
	}
	if tbl.Properties["raw_sql"] != "true" {
		t.Error("products should be raw_sql=true")
	}
	id := findEnt(ents, "SCOPE.Schema", "column", "id")
	if id == nil || id.Properties["primary_key"] != "true" || id.Properties["sql_type"] != "INTEGER" {
		t.Errorf("expected products.id INTEGER PRIMARY KEY, got %+v", id)
	}
	if findEnt(ents, "SCOPE.Schema", "column", "name") == nil {
		t.Error("expected products.name column")
	}
}

// TestDartObjectbox proves an @Entity() class (parenthesised form, distinct
// from floor's bare @entity) becomes an objectbox model with id primary key.
func TestDartObjectbox(t *testing.T) {
	src := `
import 'package:objectbox/objectbox.dart';

@Entity()
class Note {
  @Id()
  int id = 0;
  final String text;
  Note(this.text);
}
`
	ents := pents(t, "lib/note.dart", src)
	m := findEnt(ents, "SCOPE.Schema", "model", "Note")
	if m == nil {
		t.Fatal("expected objectbox model Note")
	}
	if m.Properties["store_kind"] != "nosql_embedded" {
		t.Errorf("Note store_kind = %q", m.Properties["store_kind"])
	}
	id := findEnt(ents, "SCOPE.Schema", "column", "id")
	if id == nil || id.Properties["primary_key"] != "true" {
		t.Errorf("expected objectbox id column primary_key, got %+v", id)
	}
	if findEnt(ents, "SCOPE.Schema", "column", "text") == nil {
		t.Error("expected objectbox column text")
	}
}

// TestDartCodegenAwareness proves freezed / json_serializable / part 'x.g.dart'
// markers are stamped onto a model entity.
func TestDartCodegenAwareness(t *testing.T) {
	src := `
import 'package:isar/isar.dart';
import 'package:json_annotation/json_annotation.dart';

part 'user.g.dart';

@collection
@JsonSerializable()
class User {
  Id id = Isar.autoIncrement;
  late String name;
}
`
	ents := pents(t, "lib/user.dart", src)
	m := findEnt(ents, "SCOPE.Schema", "collection", "User")
	if m == nil {
		t.Fatal("expected isar collection User")
	}
	if m.Properties["codegen"] != "json_serializable" {
		t.Errorf("User codegen = %q, want json_serializable", m.Properties["codegen"])
	}
	if m.Properties["build_runner"] != "true" {
		t.Error("User should be build_runner=true")
	}
	if m.Properties["has_part_g"] != "true" {
		t.Error("User should be has_part_g=true")
	}
}

// TestDartPersistenceNoSignal proves a plain Dart file with no persistence
// signal yields nothing (no misfire).
func TestDartPersistenceNoSignal(t *testing.T) {
	src := `
class Foo {
  final int x;
  Foo(this.x);
}
`
	if ents := pents(t, "lib/foo.dart", src); len(ents) != 0 {
		t.Errorf("expected no entities for plain class, got %d", len(ents))
	}
}
