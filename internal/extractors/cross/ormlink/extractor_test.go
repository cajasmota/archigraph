// Package ormlink_test verifies ORM → SQL table MAPS_TO link extraction.
// Issue #1275.
package ormlink_test

import (
	"context"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	_ "github.com/cajasmota/grafel/internal/extractors/cross/ormlink"
)

func ext(t *testing.T) extractor.Extractor {
	t.Helper()
	e, ok := extractor.Get("_cross_ormlink")
	if !ok {
		t.Fatal("_cross_ormlink extractor not registered")
	}
	return e
}

func links(t *testing.T, src, path, lang string) map[string]string {
	t.Helper()
	e := ext(t)
	records, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     path,
		Content:  []byte(src),
		Language: lang,
	})
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	// Collect tableName keyed by modelName from sentinel Properties.
	out := map[string]string{}
	for _, r := range records {
		if r.Subtype != "orm_model_sentinel" {
			continue
		}
		out[r.Properties["orm_model"]] = r.Properties["table_name"]
	}
	return out
}

func assertLink(t *testing.T, got map[string]string, model, wantTable string) {
	t.Helper()
	if got[model] != wantTable {
		t.Errorf("model %q → table %q, want %q (all links: %v)", model, got[model], wantTable, got)
	}
}

// ---------------------------------------------------------------------------
// Django
// ---------------------------------------------------------------------------

func TestOrmLink_Django_ExplicitDbTable(t *testing.T) {
	src := `from django.db import models

class Order(models.Model):
    total = models.DecimalField(max_digits=10, decimal_places=2)

    class Meta:
        db_table = "shop_orders"
`
	got := links(t, src, "shop/models.py", "python")
	assertLink(t, got, "Order", "shop_orders")
}

func TestOrmLink_Django_ConventionTableName(t *testing.T) {
	src := `from django.db import models

class Customer(models.Model):
    name = models.CharField(max_length=200)
`
	// models.py lives in "accounts" app directory
	got := links(t, src, "accounts/models.py", "python")
	// Convention: <app_label>_<snake_plural> = accounts_customers
	assertLink(t, got, "Customer", "accounts_customers")
}

func TestOrmLink_Django_MultipleModels(t *testing.T) {
	src := `from django.db import models

class Product(models.Model):
    name = models.CharField(max_length=200)
    class Meta:
        db_table = "catalog_products"

class Review(models.Model):
    rating = models.IntegerField()
`
	got := links(t, src, "catalog/models.py", "python")
	assertLink(t, got, "Product", "catalog_products")
	assertLink(t, got, "Review", "catalog_reviews")
}

// ---------------------------------------------------------------------------
// SQLAlchemy
// ---------------------------------------------------------------------------

func TestOrmLink_SQLAlchemy_Tablename(t *testing.T) {
	src := `from sqlalchemy import Column, Integer, String
from sqlalchemy.ext.declarative import declarative_base

Base = declarative_base()

class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True)
    name = Column(String)
`
	got := links(t, src, "db/models.py", "python")
	assertLink(t, got, "User", "users")
}

func TestOrmLink_SQLAlchemy_MultipleModels(t *testing.T) {
	src := `from sqlalchemy.ext.declarative import declarative_base
Base = declarative_base()

class Account(Base):
    __tablename__ = "accounts"

class Token(Base):
    __tablename__ = "auth_tokens"
`
	got := links(t, src, "models.py", "python")
	assertLink(t, got, "Account", "accounts")
	assertLink(t, got, "Token", "auth_tokens")
}

// ---------------------------------------------------------------------------
// ActiveRecord
// ---------------------------------------------------------------------------

func TestOrmLink_ActiveRecord_Convention(t *testing.T) {
	src := `class LineItem < ApplicationRecord
  belongs_to :order
end
`
	got := links(t, src, "app/models/line_item.rb", "ruby")
	assertLink(t, got, "LineItem", "line_items")
}

func TestOrmLink_ActiveRecord_ExplicitTableName(t *testing.T) {
	src := `class LegacyOrder < ApplicationRecord
  self.table_name = "old_orders"
end
`
	got := links(t, src, "app/models/legacy_order.rb", "ruby")
	assertLink(t, got, "LegacyOrder", "old_orders")
}

// ---------------------------------------------------------------------------
// Hibernate / JPA
// ---------------------------------------------------------------------------

func TestOrmLink_JPA_WithTableAnnotation(t *testing.T) {
	src := `import javax.persistence.Entity;
import javax.persistence.Table;

@Entity
@Table(name = "products")
public class Product {
    private Long id;
}
`
	got := links(t, src, "src/main/java/Product.java", "java")
	assertLink(t, got, "Product", "products")
}

func TestOrmLink_JPA_WithoutTableAnnotation(t *testing.T) {
	src := `import javax.persistence.Entity;

@Entity
public class Invoice {
    private Long id;
}
`
	got := links(t, src, "src/Invoice.java", "java")
	assertLink(t, got, "Invoice", "invoice")
}

// ---------------------------------------------------------------------------
// Ecto
// ---------------------------------------------------------------------------

func TestOrmLink_Ecto_Schema(t *testing.T) {
	src := `defmodule MyApp.Accounts.User do
  use Ecto.Schema

  schema "users" do
    field :email, :string
    field :name, :string
  end
end
`
	got := links(t, src, "lib/my_app/accounts/user.ex", "elixir")
	assertLink(t, got, "MyApp.Accounts.User", "users")
}

// ---------------------------------------------------------------------------
// TypeORM
// ---------------------------------------------------------------------------

func TestOrmLink_TypeORM_ExplicitName(t *testing.T) {
	src := `import { Entity, Column } from 'typeorm';

@Entity({ name: 'shop_products' })
export class ProductEntity {
  id: number;
}
`
	got := links(t, src, "src/product.entity.ts", "typescript")
	assertLink(t, got, "ProductEntity", "shop_products")
}

func TestOrmLink_TypeORM_ConventionName(t *testing.T) {
	src := `import { Entity } from 'typeorm';

@Entity()
export class OrderItem {
  id: number;
}
`
	got := links(t, src, "src/order-item.entity.ts", "typescript")
	assertLink(t, got, "OrderItem", "order_items")
}

// ---------------------------------------------------------------------------
// Sequelize
// ---------------------------------------------------------------------------

func TestOrmLink_Sequelize_Define(t *testing.T) {
	src := `const { Sequelize } = require('sequelize');
const sequelize = new Sequelize('sqlite::memory:');

const User = sequelize.define('users', {
  name: Sequelize.STRING,
});
`
	got := links(t, src, "models/user.js", "javascript")
	assertLink(t, got, "users", "users")
}

// ---------------------------------------------------------------------------
// Prisma
// ---------------------------------------------------------------------------

func TestOrmLink_Prisma_WithMap(t *testing.T) {
	src := `model Post {
  id    Int    @id
  title String

  @@map("blog_posts")
}
`
	got := links(t, src, "prisma/schema.prisma", "prisma")
	assertLink(t, got, "Post", "blog_posts")
}

func TestOrmLink_Prisma_ConventionNoMap(t *testing.T) {
	src := `model Comment {
  id   Int    @id
  body String
}
`
	got := links(t, src, "prisma/schema.prisma", "prisma")
	assertLink(t, got, "Comment", "comments")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestOrmLink_EmptyFile(t *testing.T) {
	e := ext(t)
	records, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     "models.py",
		Content:  []byte(""),
		Language: "python",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for empty file, got %d", len(records))
	}
}

func TestOrmLink_NoOrmSignals(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	e := ext(t)
	records, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     "main.go",
		Content:  []byte(src),
		Language: "go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for non-ORM file, got %d", len(records))
	}
}

func TestOrmLink_MapsToRelationshipEmitted(t *testing.T) {
	src := `from sqlalchemy.ext.declarative import declarative_base
Base = declarative_base()

class Widget(Base):
    __tablename__ = "widgets"
`
	e := ext(t)
	records, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     "models/widget.py",
		Content:  []byte(src),
		Language: "python",
	})
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	var mapsToCount int
	for _, r := range records {
		for _, rel := range r.Relationships {
			if rel.Kind == "MAPS_TO" && rel.ToID == "widgets" {
				mapsToCount++
			}
		}
	}
	if mapsToCount == 0 {
		t.Error("expected at least one MAPS_TO relationship pointing to 'widgets'")
	}
}
