package java

import (
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// detectPanache — detection logic tests
// -----------------------------------------------------------------------------

func TestDetectPanache_SQLEntity(t *testing.T) {
	decl := `@Entity
public class Book extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`

	info := detectPanache(decl, imports)
	if info.variant != panacheSQLEntity {
		t.Fatalf("expected panacheSQLEntity, got %v", info.variant)
	}
	if !info.isEntity {
		t.Error("expected isEntity=true")
	}
	if info.reactive {
		t.Error("expected reactive=false")
	}
}

func TestDetectPanache_SQLEntityBase(t *testing.T) {
	decl := `public class Book extends PanacheEntityBase {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntityBase;`

	info := detectPanache(decl, imports)
	if info.variant != panacheSQLEntity {
		t.Fatalf("expected panacheSQLEntity for PanacheEntityBase, got %v", info.variant)
	}
	if !info.isEntity {
		t.Error("expected isEntity=true")
	}
}

func TestDetectPanache_SQLRepository(t *testing.T) {
	decl := `@ApplicationScoped
public class BookRepository implements PanacheRepository<Book> {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheRepository;`

	info := detectPanache(decl, imports)
	if info.variant != panacheSQLRepository {
		t.Fatalf("expected panacheSQLRepository, got %v", info.variant)
	}
	if info.isEntity {
		t.Error("expected isEntity=false for repository")
	}
}

func TestDetectPanache_SQLRepositoryBase(t *testing.T) {
	decl := `public class BookRepository implements PanacheRepositoryBase<Book, Long> {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheRepositoryBase;`

	info := detectPanache(decl, imports)
	if info.variant != panacheSQLRepository {
		t.Fatalf("expected panacheSQLRepository for PanacheRepositoryBase, got %v", info.variant)
	}
}

func TestDetectPanache_ReactiveEntity(t *testing.T) {
	decl := `@Entity public class Product extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.reactive.panache.PanacheEntity;`

	info := detectPanache(decl, imports)
	if info.variant != panacheReactiveEntity {
		t.Fatalf("expected panacheReactiveEntity, got %v", info.variant)
	}
	if !info.reactive {
		t.Error("expected reactive=true for reactive Panache import")
	}
}

func TestDetectPanache_MongoEntity(t *testing.T) {
	decl := `@MongoEntity public class Person extends PanacheMongoEntity {`
	imports := `import io.quarkus.mongodb.panache.PanacheMongoEntity;`

	info := detectPanache(decl, imports)
	if info.variant != panacheMongoEntity {
		t.Fatalf("expected panacheMongoEntity, got %v", info.variant)
	}
	if !info.isEntity {
		t.Error("expected isEntity=true")
	}
}

func TestDetectPanache_MongoRepository(t *testing.T) {
	decl := `@ApplicationScoped public class PersonRepository implements PanacheMongoRepository<Person> {`
	imports := `import io.quarkus.mongodb.panache.PanacheMongoRepository;`

	info := detectPanache(decl, imports)
	if info.variant != panacheMongoRepository {
		t.Fatalf("expected panacheMongoRepository, got %v", info.variant)
	}
	if info.isEntity {
		t.Error("expected isEntity=false for repository")
	}
}

func TestDetectPanache_ReactiveMongoEntity(t *testing.T) {
	decl := `public class Article extends ReactivePanacheMongoEntity {`
	imports := `import io.quarkus.mongodb.panache.reactive.ReactivePanacheMongoEntity;`

	info := detectPanache(decl, imports)
	if info.variant != panacheReactiveMongoEntity {
		t.Fatalf("expected panacheReactiveMongoEntity, got %v", info.variant)
	}
	if !info.reactive {
		t.Error("expected reactive=true")
	}
}

func TestDetectPanache_None(t *testing.T) {
	decl := `public class BookService {`
	imports := `import java.util.List;`

	info := detectPanache(decl, imports)
	if info.variant != panacheNone {
		t.Fatalf("expected panacheNone for non-Panache class, got %v", info.variant)
	}
}

func TestDetectPanache_WildcardImport(t *testing.T) {
	decl := `@Entity public class Order extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.*;`

	info := detectPanache(decl, imports)
	if info.variant != panacheSQLEntity {
		t.Fatalf("expected panacheSQLEntity with wildcard import, got %v", info.variant)
	}
}

// -----------------------------------------------------------------------------
// extractExtends / extractImplements
// -----------------------------------------------------------------------------

func TestExtractExtends_Simple(t *testing.T) {
	decl := `public class Book extends PanacheEntity {`
	if got := extractExtends(decl); got != "PanacheEntity" {
		t.Errorf("expected PanacheEntity, got %q", got)
	}
}

func TestExtractExtends_Generic(t *testing.T) {
	decl := `public class Book extends PanacheEntityBase<Long> {`
	if got := extractExtends(decl); got != "PanacheEntityBase" {
		t.Errorf("expected PanacheEntityBase, got %q", got)
	}
}

func TestExtractExtends_FullyQualified(t *testing.T) {
	decl := `public class Book extends io.quarkus.hibernate.orm.panache.PanacheEntity {`
	if got := extractExtends(decl); got != "PanacheEntity" {
		t.Errorf("expected PanacheEntity, got %q", got)
	}
}

func TestExtractExtends_None(t *testing.T) {
	decl := `public class Book {`
	if got := extractExtends(decl); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractImplements_Single(t *testing.T) {
	decl := `public class BookRepo implements PanacheRepository<Book> {`
	ifaces := extractImplements(decl)
	if len(ifaces) != 1 || ifaces[0] != "PanacheRepository" {
		t.Errorf("expected [PanacheRepository], got %v", ifaces)
	}
}

func TestExtractImplements_Multiple(t *testing.T) {
	decl := `public class BookRepo implements PanacheRepository<Book>, Serializable {`
	ifaces := extractImplements(decl)
	found := false
	for _, i := range ifaces {
		if i == "PanacheRepository" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected PanacheRepository in %v", ifaces)
	}
}

// -----------------------------------------------------------------------------
// synthesizePanacheEntities — SQL entity synthesis
// -----------------------------------------------------------------------------

func TestSynthesizePanache_SQLEntity_HasFindById(t *testing.T) {
	decl := `@Entity public class Book extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Book", decl, "", "/src/Book.java", imports)

	if len(entities) == 0 {
		t.Fatal("expected synthesized entities, got none")
	}

	found := false
	for _, e := range entities {
		if e.Name == "Book.findById" {
			found = true
			if e.Properties["is_static"] != "true" {
				t.Error("findById should be static")
			}
			if e.Properties["synthesized_from"] != "quarkus_panache" {
				t.Errorf("expected synthesized_from=quarkus_panache, got %q", e.Properties["synthesized_from"])
			}
			if e.Properties["pattern_type"] != "panache_inherited_method" {
				t.Errorf("expected pattern_type=panache_inherited_method, got %q", e.Properties["pattern_type"])
			}
			if e.Properties["owner"] != "Book" {
				t.Errorf("expected owner=Book, got %q", e.Properties["owner"])
			}
		}
	}
	if !found {
		t.Error("expected Book.findById entity")
	}
}

func TestSynthesizePanache_SQLEntity_HasInstancePersist(t *testing.T) {
	decl := `@Entity public class Book extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Book", decl, "", "/src/Book.java", imports)

	// Instance persist is the one without is_static
	found := false
	for _, e := range entities {
		if e.Name == "Book.persist" && e.Properties["is_static"] == "" {
			found = true
		}
	}
	if !found {
		t.Error("expected instance Book.persist entity (no is_static)")
	}
}

func TestSynthesizePanache_SQLEntity_HasDeleteAll(t *testing.T) {
	decl := `@Entity public class Order extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Order", decl, "", "/src/Order.java", imports)

	found := false
	for _, e := range entities {
		if e.Name == "Order.deleteAll" {
			found = true
		}
	}
	if !found {
		t.Error("expected Order.deleteAll entity")
	}
}

func TestSynthesizePanache_SQLEntity_HasCount(t *testing.T) {
	decl := `@Entity public class Invoice extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Invoice", decl, "", "/src/Invoice.java", imports)

	found := false
	for _, e := range entities {
		if e.Name == "Invoice.count" {
			found = true
		}
	}
	if !found {
		t.Error("expected Invoice.count entity")
	}
}

func TestSynthesizePanache_SQLEntity_HasListAll(t *testing.T) {
	decl := `@Entity public class Product extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Product", decl, "", "/src/Product.java", imports)

	found := false
	for _, e := range entities {
		if e.Name == "Product.listAll" {
			found = true
		}
	}
	if !found {
		t.Error("expected Product.listAll entity")
	}
}

func TestSynthesizePanache_SQLEntity_QualityScore(t *testing.T) {
	decl := `@Entity public class Book extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Book", decl, "", "/src/Book.java", imports)

	for _, e := range entities {
		if e.QualityScore != panacheSynthQuality {
			t.Errorf("entity %s: expected QualityScore=%.1f, got %.1f", e.Name, panacheSynthQuality, e.QualityScore)
		}
	}
}

func TestSynthesizePanache_SQLEntity_SourceFile(t *testing.T) {
	decl := `@Entity public class Book extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Book", decl, "", "/some/path/Book.java", imports)

	for _, e := range entities {
		if e.SourceFile != "/some/path/Book.java" {
			t.Errorf("entity %s: expected SourceFile=/some/path/Book.java, got %q", e.Name, e.SourceFile)
		}
	}
}

// -----------------------------------------------------------------------------
// Repository synthesis
// -----------------------------------------------------------------------------

func TestSynthesizePanache_Repository_HasFindById(t *testing.T) {
	decl := `@ApplicationScoped public class BookRepository implements PanacheRepository<Book> {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheRepository;`
	entities := synthesizePanacheEntities("BookRepository", decl, "", "/src/BookRepository.java", imports)

	if len(entities) == 0 {
		t.Fatal("expected synthesized entities for repository")
	}
	found := false
	for _, e := range entities {
		if e.Name == "BookRepository.findById" {
			found = true
			// Repository methods are instance methods — no is_static
			if e.Properties["is_static"] == "true" {
				t.Error("repository findById should NOT be static")
			}
		}
	}
	if !found {
		t.Error("expected BookRepository.findById entity")
	}
}

func TestSynthesizePanache_Repository_HasPersist(t *testing.T) {
	decl := `public class OrderRepo implements PanacheRepositoryBase<Order, Long> {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheRepositoryBase;`
	entities := synthesizePanacheEntities("OrderRepo", decl, "", "/src/OrderRepo.java", imports)

	found := false
	for _, e := range entities {
		if e.Name == "OrderRepo.persist" {
			found = true
		}
	}
	if !found {
		t.Error("expected OrderRepo.persist entity")
	}
}

// -----------------------------------------------------------------------------
// Reactive Panache
// -----------------------------------------------------------------------------

func TestSynthesizePanache_ReactiveEntity_ReturnsUni(t *testing.T) {
	decl := `@Entity public class Task extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.reactive.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Task", decl, "", "/src/Task.java", imports)

	foundUni := false
	for _, e := range entities {
		if e.Name == "Task.findById" && strings.Contains(e.Signature, "Uni<") {
			foundUni = true
			if e.Properties["reactive"] != "true" {
				t.Error("reactive entity should have reactive=true property")
			}
		}
	}
	if !foundUni {
		t.Error("expected reactive Task.findById returning Uni<Task>")
	}
}

func TestSynthesizePanache_ReactiveEntity_NoStaticReturnsDirectType(t *testing.T) {
	// Non-reactive entity should return Task directly, not Uni<Task>
	decl := `@Entity public class Task extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Task", decl, "", "/src/Task.java", imports)

	for _, e := range entities {
		if e.Name == "Task.findById" && e.Properties["is_static"] == "true" {
			if strings.Contains(e.Signature, "Uni<") {
				t.Error("SQL (non-reactive) findById should not return Uni<>")
			}
		}
	}
}

// -----------------------------------------------------------------------------
// MongoDB Panache
// -----------------------------------------------------------------------------

func TestSynthesizePanache_MongoEntity_HasFindById(t *testing.T) {
	decl := `@MongoEntity public class Article extends PanacheMongoEntity {`
	imports := `import io.quarkus.mongodb.panache.PanacheMongoEntity;`
	entities := synthesizePanacheEntities("Article", decl, "", "/src/Article.java", imports)

	if len(entities) == 0 {
		t.Fatal("expected synthesized entities for Mongo entity")
	}
	found := false
	for _, e := range entities {
		if e.Name == "Article.findById" {
			found = true
		}
	}
	if !found {
		t.Error("expected Article.findById entity")
	}
}

func TestSynthesizePanache_MongoEntity_HasPersistOrUpdate(t *testing.T) {
	decl := `@MongoEntity public class Article extends PanacheMongoEntity {`
	imports := `import io.quarkus.mongodb.panache.PanacheMongoEntity;`
	entities := synthesizePanacheEntities("Article", decl, "", "/src/Article.java", imports)

	found := false
	for _, e := range entities {
		if e.Name == "Article.persistOrUpdate" {
			found = true
		}
	}
	if !found {
		t.Error("expected Article.persistOrUpdate entity (Mongo-specific)")
	}
}

// -----------------------------------------------------------------------------
// @NamedQuery synthesis
// -----------------------------------------------------------------------------

func TestSynthesizeNamedQuery_Basic(t *testing.T) {
	decl := `@Entity
@NamedQuery(name="Book.byTitle", query="FROM Book WHERE title=:title")
public class Book extends PanacheEntity {`
	entities := synthesizeNamedQueryEntities("Book", decl, "/src/Book.java")

	if len(entities) != 1 {
		t.Fatalf("expected 1 named query entity, got %d", len(entities))
	}
	if entities[0].Name != "Book.byTitle" {
		t.Errorf("expected name=Book.byTitle, got %q", entities[0].Name)
	}
	if entities[0].Properties["pattern_type"] != "panache_named_query" {
		t.Errorf("expected pattern_type=panache_named_query")
	}
}

func TestSynthesizeNamedQuery_Multiple(t *testing.T) {
	decl := `@Entity
@NamedQueries({
  @NamedQuery(name="Book.byTitle", query="FROM Book WHERE title=:title"),
  @NamedQuery(name="Book.byAuthor", query="FROM Book WHERE author=:author")
})
public class Book extends PanacheEntity {`
	entities := synthesizeNamedQueryEntities("Book", decl, "/src/Book.java")

	if len(entities) != 2 {
		t.Fatalf("expected 2 named query entities, got %d", len(entities))
	}
}

func TestSynthesizeNamedQuery_None(t *testing.T) {
	decl := `@Entity public class Book extends PanacheEntity {`
	entities := synthesizeNamedQueryEntities("Book", decl, "/src/Book.java")

	if len(entities) != 0 {
		t.Fatalf("expected 0 named query entities, got %d", len(entities))
	}
}

// -----------------------------------------------------------------------------
// Guard: non-Panache classes produce no output
// -----------------------------------------------------------------------------

func TestSynthesizePanache_NonPanache_NilOutput(t *testing.T) {
	decl := `@Service public class BookService {`
	imports := `import java.util.List;`
	entities := synthesizePanacheEntities("BookService", decl, "", "/src/BookService.java", imports)

	if len(entities) != 0 {
		t.Errorf("expected nil for non-Panache class, got %d entities", len(entities))
	}
}

// -----------------------------------------------------------------------------
// Minimum count checks
// -----------------------------------------------------------------------------

func TestSynthesizePanache_SQLEntity_MinimumMethodCount(t *testing.T) {
	decl := `@Entity public class Book extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Book", decl, "", "/src/Book.java", imports)

	// We expect at least 30 entities (24 static + 5 instance + any named queries)
	if len(entities) < 30 {
		t.Errorf("expected at least 30 synthesized entities, got %d", len(entities))
	}
}

func TestSynthesizePanache_Repository_MinimumMethodCount(t *testing.T) {
	decl := `public class BookRepo implements PanacheRepository<Book> {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheRepository;`
	entities := synthesizePanacheEntities("BookRepo", decl, "", "/src/BookRepo.java", imports)

	if len(entities) < 15 {
		t.Errorf("expected at least 15 synthesized repository entities, got %d", len(entities))
	}
}

// -----------------------------------------------------------------------------
// Projection synthesis
// -----------------------------------------------------------------------------

func TestSynthesizePanache_SQLEntity_HasProject(t *testing.T) {
	decl := `@Entity public class Book extends PanacheEntity {`
	imports := `import io.quarkus.hibernate.orm.panache.PanacheEntity;`
	entities := synthesizePanacheEntities("Book", decl, "", "/src/Book.java", imports)

	found := false
	for _, e := range entities {
		if e.Name == "Book.project" {
			found = true
		}
	}
	if !found {
		t.Error("expected Book.project entity for projections support")
	}
}
