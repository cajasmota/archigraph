package csharp_test

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Dapper POCO models
// ---------------------------------------------------------------------------

func TestDapperTableAttribute(t *testing.T) {
	src := `
using Dapper;
using System.ComponentModel.DataAnnotations.Schema;

[Table("products")]
public class Product
{
    [Column("product_id")]
    public int Id { get; set; }

    [Column("product_name")]
    public string Name { get; set; }

    [Column("price")]
    public decimal Price { get; set; }
}
`
	ents := extract(t, "custom_csharp_orm_models", fi("Product.cs", "csharp", src))

	if !containsEntity(ents, "SCOPE.Component", "dapper:table:products") {
		t.Error("expected dapper:table:products from [Table(\"products\")]")
	}
	if !containsEntity(ents, "SCOPE.Component", "dapper:poco:Product") {
		t.Error("expected dapper:poco:Product POCO class entity")
	}
	// At least one [Column] entity should appear
	foundColumn := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "schema_extraction" {
			foundColumn = true
			break
		}
	}
	if !foundColumn {
		t.Error("expected schema_extraction entity from [Column] attribute")
	}
}

func TestDapperQueryAttribution(t *testing.T) {
	src := `
using Dapper;

public class OrderRepository
{
    public IEnumerable<Order> GetAll(IDbConnection conn)
    {
        return conn.Query<Order>("SELECT * FROM orders");
    }

    public void Create(IDbConnection conn, Order order)
    {
        conn.Execute("INSERT INTO orders VALUES (@Id, @Total)", order);
    }
}
`
	ents := extract(t, "custom_csharp_orm_models", fi("OrderRepository.cs", "csharp", src))

	foundQuery := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Subtype == "query_attribution" {
			foundQuery = true
			break
		}
	}
	if !foundQuery {
		t.Error("expected query_attribution from Dapper Query<T> / Execute calls")
	}
}

// ---------------------------------------------------------------------------
// LINQ-to-SQL / LinqToDB
// ---------------------------------------------------------------------------

func TestLinqToDBTableAndColumn(t *testing.T) {
	src := `
using LinqToDB;
using LinqToDB.Mapping;

[Table(Name = "customers")]
public class Customer
{
    [Column(Name = "customer_id"), PrimaryKey, Identity]
    public int Id { get; set; }

    [Column(Name = "full_name")]
    public string Name { get; set; }

    [Association(ThisKey = "Id", OtherKey = "CustomerId")]
    public List<Order> Orders { get; set; }
}
`
	ents := extract(t, "custom_csharp_orm_models", fi("Customer.cs", "csharp", src))

	if !containsEntity(ents, "SCOPE.Component", "linqtodb:table:customers") {
		t.Error("expected linqtodb:table:customers from [Table(Name=\"customers\")]")
	}
	foundSchema := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "schema_extraction" {
			foundSchema = true
			break
		}
	}
	if !foundSchema {
		t.Error("expected schema_extraction from [Column] attribute in linqtodb")
	}
	foundAssoc := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "relationship_extraction" {
			foundAssoc = true
			break
		}
	}
	if !foundAssoc {
		t.Error("expected relationship_extraction from [Association] attribute")
	}
}

func TestLinqToSQLContext(t *testing.T) {
	src := `
using System.Data.Linq;
using System.Data.Linq.Mapping;

[Table(Name="orders")]
public class Order
{
    [Column(IsPrimaryKey=true)]
    public int OrderId { get; set; }

    [Column]
    public string Description { get; set; }
}

public class ShopDataContext : DataContext
{
    public Table<Order> Orders;
}
`
	ents := extract(t, "custom_csharp_orm_models", fi("ShopDataContext.cs", "csharp", src))

	if !containsEntity(ents, "SCOPE.Component", "linq-to-sql:context:ShopDataContext") {
		t.Error("expected linq-to-sql:context:ShopDataContext")
	}
	if !containsEntity(ents, "SCOPE.Component", "linq-to-sql:table_prop:Order") {
		t.Error("expected linq-to-sql:table_prop:Order from Table<Order>")
	}
}

// ---------------------------------------------------------------------------
// NHibernate / FluentNHibernate
// ---------------------------------------------------------------------------

func TestNHibernateClassMap(t *testing.T) {
	src := `
using FluentNHibernate.Mapping;

public class ProductMap : ClassMap<Product>
{
    public ProductMap()
    {
        Table("products");
        Id(x => x.Id).Column("id").GeneratedBy.Native();
        Map(x => x.Name).Column("name").Not.Nullable();
        Map(x => x.Price).Column("price");
        References(x => x.Category).Column("category_id");
        HasMany(x => x.Reviews).KeyColumn("product_id");
    }
}
`
	ents := extract(t, "custom_csharp_orm_models", fi("ProductMap.cs", "csharp", src))

	if !containsEntity(ents, "SCOPE.Component", "nhibernate:classmap:ProductMap") {
		t.Error("expected nhibernate:classmap:ProductMap")
	}
	if !containsEntity(ents, "SCOPE.Component", "nhibernate:schema:Product") {
		t.Error("expected nhibernate:schema:Product")
	}
	foundRef := false
	foundHasMany := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Pattern" && e.Subtype == "relationship_extraction" {
			if e.Name == "nhibernate:references:Category" {
				foundRef = true
			}
			if e.Name == "nhibernate:hasmany:Reviews" {
				foundHasMany = true
			}
		}
	}
	if !foundRef {
		t.Error("expected relationship_extraction for References(x => x.Category)")
	}
	if !foundHasMany {
		t.Error("expected relationship_extraction for HasMany(x => x.Reviews)")
	}
}

func TestNHibernateSessionQuery(t *testing.T) {
	src := `
using NHibernate;

public class ProductRepository
{
    private readonly ISession _session;

    public IList<Product> GetAll()
    {
        return _session.Query<Product>().ToList();
    }

    public Product GetById(int id)
    {
        return _session.Get<Product>(id);
    }
}
`
	ents := extract(t, "custom_csharp_orm_models", fi("ProductRepository.cs", "csharp", src))

	foundQuery := false
	for _, e := range ents {
		if e.Kind == "SCOPE.Operation" && e.Subtype == "query_attribution" {
			foundQuery = true
			break
		}
	}
	if !foundQuery {
		t.Error("expected query_attribution from ISession.Query<T> / Get<T>")
	}
}

// ---------------------------------------------------------------------------
// Non-csharp files should produce no entities
// ---------------------------------------------------------------------------

func TestOrmModelsNonCsharpFile(t *testing.T) {
	src := `
using Dapper;
[Table("x")]
public class X {}
`
	ents := extract(t, "custom_csharp_orm_models", fi("model.go", "go", src))
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for non-csharp file, got %d", len(ents))
	}
}
