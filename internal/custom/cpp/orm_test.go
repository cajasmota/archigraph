package cpp_test

// orm_test.go — tests for ODB, SOCI, and sqlpp11 C++ ORM extractors.

import (
	"testing"
)

// ============================================================================
// ODB
// ============================================================================

func TestODBModelExtraction(t *testing.T) {
	src := `
#pragma db object
class Person {
public:
	std::string name;
	int age;
};
`
	ents := extract(t, "custom_cpp_odb", fi("person.hxx", "cpp", src))
	if !containsEntity(ents, "SCOPE.Schema", "Person") {
		t.Error("expected SCOPE.Schema entity for Person")
	}
}

func TestODBMemberExtraction(t *testing.T) {
	src := `
#pragma db member(Person::name) column("person_name")
#pragma db member(Person::age)
`
	ents := extract(t, "custom_cpp_odb", fi("person.hxx", "cpp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Schema" && e.Subtype == "column" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SCOPE.Schema column entity for member pragma")
	}
}

func TestODBRelationshipExtraction(t *testing.T) {
	src := `
#pragma db member(Employee::employer) one_to_many inverse(employees)
`
	ents := extract(t, "custom_cpp_odb", fi("employee.hxx", "cpp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "relationship" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SCOPE.Pattern relationship for one_to_many member")
	}
}

func TestODBLazyPtrExtraction(t *testing.T) {
	src := `
odb::lazy_ptr<Employer> employer_;
odb::lazy_shared_ptr<Department> dept_;
`
	ents := extract(t, "custom_cpp_odb", fi("employee.hxx", "cpp", src))
	found := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "relationship" {
			found++
		}
	}
	if found < 2 {
		t.Errorf("expected at least 2 lazy_ptr relationship entities, got %d", found)
	}
}

func TestODBQueryExtraction(t *testing.T) {
	src := `
odb::result<Person> r = db.query<Person>(odb::query<Person>::age > 18);
`
	ents := extract(t, "custom_cpp_odb", fi("main.cpp", "cpp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Subtype == "query" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SCOPE.Operation query entity for odb::query<Person>")
	}
}

func TestODBNoMatch(t *testing.T) {
	src := `#include <iostream>
int main() { return 0; }
`
	ents := extract(t, "custom_cpp_odb", fi("main.cpp", "cpp", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

func TestODBWrongLanguage(t *testing.T) {
	src := `#pragma db object
class Foo {};`
	// python language → should not match
	ents := extract(t, "custom_cpp_odb", fi("foo.hxx", "python", src))
	if len(ents) != 0 {
		t.Errorf("wrong language should return no entities, got %d", len(ents))
	}
}

// ============================================================================
// SOCI
// ============================================================================

func TestSOCITypeConversionModel(t *testing.T) {
	src := `
template<> struct type_conversion<Person> {
	typedef values base_type;
	static void from_base(values const& v, indicator ind, Person& p) {
		p.name = v.get<string>("name");
	}
};
`
	ents := extract(t, "custom_cpp_soci", fi("person.cpp", "cpp", src))
	if !containsEntity(ents, "SCOPE.Schema", "Person") {
		t.Error("expected SCOPE.Schema entity for type_conversion<Person>")
	}
}

func TestSOCIIntoBinding(t *testing.T) {
	src := `
soci::session sql("postgresql://dbname=mydb");
int count;
sql << "SELECT COUNT(*) FROM persons", into(count);
`
	ents := extract(t, "custom_cpp_soci", fi("query.cpp", "cpp", src))
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Schema" && e.Subtype == "column" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SCOPE.Schema column binding for into(count)")
	}
}

func TestSOCIQueryExtraction(t *testing.T) {
	src := `
soci::session sql("sqlite3://mydb.db");
sql << "SELECT * FROM users WHERE id = :id", use(id), into(user);
sql << "INSERT INTO logs (msg) VALUES (:msg)", use(msg);
`
	ents := extract(t, "custom_cpp_soci", fi("query.cpp", "cpp", src))
	queryCount := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Subtype == "query" {
			queryCount++
		}
	}
	if queryCount < 2 {
		t.Errorf("expected at least 2 query entities, got %d", queryCount)
	}
}

func TestSOCINoMatch(t *testing.T) {
	src := `#include <vector>
void foo() { std::vector<int> v; }`
	ents := extract(t, "custom_cpp_soci", fi("foo.cpp", "cpp", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities, got %d", len(ents))
	}
}

// ============================================================================
// sqlpp11
// ============================================================================

func TestSQLPP11TableExtraction(t *testing.T) {
	src := `
struct TabPerson : sqlpp::table<TabPerson, TabPerson::Id, TabPerson::Name> {
	struct id_ {
		struct _alias_t { static constexpr const char _literal[] = "id"; };
		using _traits = sqlpp::make_traits<sqlpp::integer, sqlpp::tag::must_not_insert>;
	};
	struct name_ {
		struct _alias_t { static constexpr const char _literal[] = "name"; };
		using _traits = sqlpp::make_traits<sqlpp::varchar>;
	};
	id_   id;
	name_ name;
};
`
	ents := extract(t, "custom_cpp_sqlpp11", fi("tables.h", "cpp", src))
	if !containsEntity(ents, "SCOPE.Schema", "TabPerson") {
		t.Error("expected SCOPE.Schema entity for TabPerson")
	}
}

func TestSQLPP11AliasExtraction(t *testing.T) {
	src := `SQLPP_ALIAS_PROVIDER(left)
SQLPP_ALIAS_PROVIDER(right)
`
	ents := extract(t, "custom_cpp_sqlpp11", fi("tables.h", "cpp", src))
	found := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Schema" && e.Subtype == "alias" {
			found++
		}
	}
	if found < 2 {
		t.Errorf("expected at least 2 alias schema entities, got %d", found)
	}
}

func TestSQLPP11QueryExtraction(t *testing.T) {
	src := `
auto result = db(select(tab.id, tab.name).from(tab).where(tab.id == id));
db(insert_into(tab).set(tab.name = "alice"));
db(update(tab).set(tab.name = "bob").where(tab.id == 1));
db(remove_from(tab).where(tab.id == 2));
`
	ents := extract(t, "custom_cpp_sqlpp11", fi("queries.cpp", "cpp", src))
	queryCount := 0
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Subtype == "query" {
			queryCount++
		}
	}
	if queryCount < 4 {
		t.Errorf("expected at least 4 query entities (select/insert/update/remove), got %d", queryCount)
	}
}

func TestSQLPP11NoMatch(t *testing.T) {
	src := `#include <iostream>
int main() { return 0; }
`
	ents := extract(t, "custom_cpp_sqlpp11", fi("main.cpp", "cpp", src))
	if len(ents) != 0 {
		t.Errorf("expected no entities for non-sqlpp11 file, got %d", len(ents))
	}
}
