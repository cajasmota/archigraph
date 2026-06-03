// Wave-1 structural-coverage proof tests (epic #3872).
//
// These drive the REAL language-level substrate sniffers on the actual
// framework idioms backing the coverage records flipped in this wave:
//
//   - lang.csharp.framework.hotchocolate  → sniffDefUseCSharp / sniffCSharpEntryPoints
//   - lang.ruby.framework.graphql-ruby    → sniffDefUseRuby   / sniffRubyEntryPoints
//
// Per precedent (kotlin/scala/elixir waves) the structural caps
// (def_use_chain_extraction, dead_code_detection, reachability_analysis)
// are language-level sniffers registered on the slug; the GraphQL
// framework idiom is plain C#/Ruby source so the sniffer fires on it.
// Each subtest asserts EXACT symbols/kinds, never len>0.
package substrate

import "testing"

// hotChocolateResolverSrcW1crp is a representative HotChocolate query-type
// resolver: a `[QueryType]`/`ObjectType` class whose resolver method
// declares locals and reads them — the def-use shape we attribute.
const hotChocolateResolverSrcW1crp = `
using HotChocolate;
using HotChocolate.Types;

public class Query
{
    public Book GetBook(BookRepository repo, int id)
    {
        var book = repo.FindById(id);
        var title = book.Title;
        return new Book { Title = title };
    }
}
`

// graphqlRubyResolverSrcW1crp is a representative graphql-ruby resolver:
// a Types::QueryType field method that binds locals and reuses them.
const graphqlRubyResolverSrcW1crp = `
require "graphql"

module Types
  class QueryType < GraphQL::Schema::Object
    field :book, Types::BookType, null: true do
      argument :id, ID, required: true
    end

    def book(id:)
      record = BookRepository.find(id)
      title = record.title
      title
    end
  end
end
`

func TestDefUse_HotChocolate_resolver_w1crp(t *testing.T) {
	// Drives the registered csharp sniffer exactly as the def_use_pass
	// would, proving def_use_chain_extraction fires on the HotChocolate
	// resolver idiom.
	if DefUseSnifferFor("csharp") == nil {
		t.Fatal("csharp def-use sniffer not registered")
	}
	defs, uses := sniffDefUseCSharp(hotChocolateResolverSrcW1crp)

	// EXACT defs: `var book =` and `var title =` inside GetBook.
	if !containsVarDef(defs, "GetBook", "book") {
		t.Errorf("expected def book in GetBook, got %+v", defs)
	}
	if !containsVarDef(defs, "GetBook", "title") {
		t.Errorf("expected def title in GetBook, got %+v", defs)
	}
	// EXACT use: `title` is read in `Title = title`.
	if !containsVarUse(uses, "GetBook", "title") {
		t.Errorf("expected use title in GetBook, got %+v", uses)
	}
	// EXACT use: `book` is read in `book.Title`.
	if !containsVarUse(uses, "GetBook", "book") {
		t.Errorf("expected use book in GetBook, got %+v", uses)
	}
}

func TestDefUse_GraphQLRuby_resolver_w1crp(t *testing.T) {
	if DefUseSnifferFor("ruby") == nil {
		t.Fatal("ruby def-use sniffer not registered")
	}
	defs, uses := sniffDefUseRuby(graphqlRubyResolverSrcW1crp)

	// EXACT defs: `record =` and `title =` inside the `book` field method.
	if !containsVarDef(defs, "book", "record") {
		t.Errorf("expected def record in book, got %+v", defs)
	}
	if !containsVarDef(defs, "book", "title") {
		t.Errorf("expected def title in book, got %+v", defs)
	}
	// EXACT use: `record` is read in `record.title`.
	if !containsVarUse(uses, "book", "record") {
		t.Errorf("expected use record in book, got %+v", uses)
	}
	// EXACT use: `title` is read as the trailing return expression.
	if !containsVarUse(uses, "book", "title") {
		t.Errorf("expected use title in book, got %+v", uses)
	}
}

func TestEntryPoints_HotChocolate_w1crp(t *testing.T) {
	// dead_code_detection + reachability_analysis seed the BFS from
	// per-language entry-points. A HotChocolate host has a Program.cs
	// Main and public resolver methods (library_export). Prove BOTH
	// kinds surface so the reachability pass can seed them.
	src := `
using HotChocolate.Execution;

public class Program
{
    public static async Task Main(string[] args)
    {
        await BuildSchema().ExecuteAsync();
    }

    public ISchema BuildSchema()
    {
        return SchemaBuilder.New().AddQueryType<Query>().Create();
    }
}
`
	eps := sniffCSharpEntryPoints(src)

	var sawMain, sawExport bool
	for _, ep := range eps {
		if ep.Ident == "Main" && ep.Kind == EntryKindCLIMain {
			sawMain = true
		}
		if ep.Ident == "BuildSchema" && ep.Kind == EntryKindLibraryExport {
			sawExport = true
		}
	}
	if !sawMain {
		t.Errorf("expected cli_main entry Main, got %+v", eps)
	}
	if !sawExport {
		t.Errorf("expected library_export entry BuildSchema, got %+v", eps)
	}
}

func TestEntryPoints_GraphQLRuby_w1crp(t *testing.T) {
	// graphql-ruby schemas expose module-level methods (library_export)
	// and ship RSpec request specs (test_entry). Prove both kinds so
	// the language-agnostic reachability/dead-code passes have seeds.
	src := `
require "graphql"

def execute_query(query_string)
  MySchema.execute(query_string)
end

describe Types::QueryType do
  it "resolves book" do
    execute_query("{ book(id: 1) { title } }")
  end
end
`
	eps := sniffRubyEntryPoints(src)

	var sawExport, sawTest bool
	for _, ep := range eps {
		if ep.Ident == "execute_query" && ep.Kind == EntryKindLibraryExport {
			sawExport = true
		}
		if ep.Kind == EntryKindTestEntry {
			sawTest = true
		}
	}
	if !sawExport {
		t.Errorf("expected library_export entry execute_query, got %+v", eps)
	}
	if !sawTest {
		t.Errorf("expected a test_entry from the RSpec block, got %+v", eps)
	}
}
