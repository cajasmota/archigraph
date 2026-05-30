package cpp_test

// driver_schema_test.go — tests for the C++ raw database driver schema extractor.

import (
	"testing"
)

func TestCppDriverLibpqxx_CreateTable(t *testing.T) {
	src := `#include <pqxx/pqxx>
#include <string>

void setup(pqxx::connection& conn) {
	pqxx::work tx(conn);
	tx.exec("CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(255) NOT NULL, age INT)");
	tx.commit();
}
`
	ents := extract(t, "custom_cpp_driver_schema", fi("setup.cpp", "cpp", src))
	if !containsEntity(ents, "SCOPE.Schema", "users") {
		t.Error("expected SCOPE.Schema entity for users table")
	}
	// Should also have column entities
	found := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Schema" && e.Subtype == "column" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected SCOPE.Schema column entity from CREATE TABLE")
	}
}

func TestCppDriverLibpqxx_QueryAttribution(t *testing.T) {
	src := `#include <pqxx/pqxx>

void query(pqxx::connection& conn, int id) {
	pqxx::work tx(conn);
	pqxx::result r = tx.exec("SELECT id, name FROM users WHERE id = " + std::to_string(id));
	tx.exec("INSERT INTO logs (msg) VALUES ('queried')");
	tx.commit();
}
`
	ents := extract(t, "custom_cpp_driver_schema", fi("query.cpp", "cpp", src))
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

func TestCppDriverMongocxx_Collection(t *testing.T) {
	src := `#include <mongocxx/client.hpp>
#include <mongocxx/instance.hpp>

void save(mongocxx::client& client) {
	auto db = client["mydb"];
	auto users = db["users"];
	auto logs = db.collection("audit_logs");
}
`
	ents := extract(t, "custom_cpp_driver_schema", fi("mongo.cpp", "cpp", src))
	if !containsEntity(ents, "SCOPE.Schema", "users") {
		t.Error("expected SCOPE.Schema entity for users collection")
	}
	if !containsEntity(ents, "SCOPE.Schema", "audit_logs") {
		t.Error("expected SCOPE.Schema entity for audit_logs collection")
	}
}

func TestCppDriverMysql_Execute(t *testing.T) {
	src := `#include <mysql/mysql.h>
#include <string>

void setup(MYSQL* conn) {
	mysql_query(conn, "CREATE TABLE orders (id INT AUTO_INCREMENT PRIMARY KEY, total DECIMAL(10,2))");
}

void doQuery(MYSQL* conn) {
	mysql_query(conn, "SELECT * FROM orders WHERE id = 1");
}
`
	// mysql_query doesn't match our .exec pattern, but we still test gate behavior
	// (no match expected since mysql_query is a function not a method call)
	ents := extract(t, "custom_cpp_driver_schema", fi("mysql.cpp", "cpp", src))
	// No match expected (mysql_query is not .exec-style), just verify no panic
	_ = ents
}

func TestCppDriverNoMatch_WrongLanguage(t *testing.T) {
	src := `#include <pqxx/pqxx>
class Foo {};`
	ents := extract(t, "custom_cpp_driver_schema", fi("foo.cpp", "python", src))
	if len(ents) != 0 {
		t.Errorf("wrong language should return no entities, got %d", len(ents))
	}
}

func TestCppDriverNoMatch_NoInclude(t *testing.T) {
	src := `#include <iostream>
int main() {
	return 0;
}
`
	ents := extract(t, "custom_cpp_driver_schema", fi("main.cpp", "cpp", src))
	if len(ents) != 0 {
		t.Errorf("without driver include, expected no entities, got %d", len(ents))
	}
}
