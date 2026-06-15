package php_test

import (
	"context"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/php"
)

// ---------------------------------------------------------------------------
// PHP custom-extractor deprecation contract (epic #3628).
//
// Symfony #[Route] and API Platform #[ApiResource] endpoints are SCOPE.Operation
// entities the language-agnostic engine deprecation pass cannot reach, so the
// custom extractors stamp the IDENTICAL flagship property contract (deprecated /
// deprecated_since / deprecated_replacement / deprecation_source) from their own
// framework idioms.
// ---------------------------------------------------------------------------

// epEntity runs a named PHP custom extractor and returns the endpoint entity
// named `name` (e.g. "GET /api/v1/users"), with full Properties.
func epEntity(t *testing.T, extractorName, path, src, name string) types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get(extractorName)
	if !ok {
		t.Fatalf("extractor %q not registered", extractorName)
	}
	ents, err := e.Extract(context.Background(), extreg.FileInput{Path: path, Language: "php", Content: []byte(src)})
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	for _, ent := range ents {
		if ent.Kind == "SCOPE.Operation" && ent.Name == name {
			return ent
		}
	}
	names := make([]string, 0, len(ents))
	for _, ent := range ents {
		if ent.Kind == "SCOPE.Operation" {
			names = append(names, ent.Name)
		}
	}
	t.Fatalf("endpoint %q not found (got: %v)", name, names)
	return types.EntityRecord{}
}

// Symfony #[Route(..., deprecated: true)] flag → deprecated=true + source.
func TestSymfonyDeprecation_RouteFlag(t *testing.T) {
	src := `<?php
namespace App\Controller;
use Symfony\Component\Routing\Annotation\Route;
class UserController {
    #[Route('/api/v1/users', methods: ['GET'], deprecated: true)]
    public function index() {}

    #[Route('/api/v1/health', methods: ['GET'])]
    public function health() {}
}
`
	dep := epEntity(t, "custom_php_symfony", "src/Controller/UserController.php", src, "GET /api/v1/users")
	if dep.Properties["deprecated"] != "true" {
		t.Fatalf("GET /api/v1/users deprecated=%q, want true (props: %v)", dep.Properties["deprecated"], dep.Properties)
	}
	if got := dep.Properties["deprecation_source"]; got != "deprecated: true" {
		t.Errorf("deprecation_source=%q, want 'deprecated: true'", got)
	}

	// Negative: the non-deprecated sibling action carries no deprecation (no leak).
	live := epEntity(t, "custom_php_symfony", "src/Controller/UserController.php", src, "GET /api/v1/health")
	if _, ok := live.Properties["deprecated"]; ok {
		t.Fatalf("GET /api/v1/health deprecation fabricated, want absent (props: %v)", live.Properties)
	}
}

// Symfony `@deprecated since 2.0 use /api/v2/users` PHPDoc above a #[Route]
// action → deprecated=true + since + replacement + source.
func TestSymfonyDeprecation_PHPDoc(t *testing.T) {
	src := `<?php
namespace App\Controller;
use Symfony\Component\Routing\Annotation\Route;
class UserController {
    /**
     * @deprecated since 2.0 use /api/v2/users instead
     */
    #[Route('/api/v1/users', methods: ['GET'])]
    public function index() {}
}
`
	dep := epEntity(t, "custom_php_symfony", "src/Controller/UserController.php", src, "GET /api/v1/users")
	if dep.Properties["deprecated"] != "true" {
		t.Fatalf("deprecated=%q, want true (props: %v)", dep.Properties["deprecated"], dep.Properties)
	}
	if got := dep.Properties["deprecation_source"]; got != "@deprecated" {
		t.Errorf("deprecation_source=%q, want '@deprecated'", got)
	}
	if got := dep.Properties["deprecated_since"]; got != "2.0" {
		t.Errorf("deprecated_since=%q, want 2.0", got)
	}
	if got := dep.Properties["deprecated_replacement"]; got != "/api/v2/users" {
		t.Errorf("deprecated_replacement=%q, want /api/v2/users", got)
	}
}

// Honest-partial: a non-deprecated Symfony route carries no deprecation property.
func TestSymfonyDeprecation_NonDeprecated(t *testing.T) {
	src := `<?php
namespace App\Controller;
use Symfony\Component\Routing\Annotation\Route;
class UserController {
    #[Route('/api/v1/users', methods: ['GET'])]
    public function index() {}
}
`
	e := epEntity(t, "custom_php_symfony", "src/Controller/UserController.php", src, "GET /api/v1/users")
	if got, ok := e.Properties["deprecated"]; ok {
		t.Fatalf("deprecated=%q fabricated on plain route, want absent", got)
	}
}

// API Platform per-operation deprecationReason → deprecated=true + replacement +
// source on that operation only.
func TestAPIPlatformDeprecation_PerOperation(t *testing.T) {
	src := `<?php
namespace App\Entity;
use ApiPlatform\Metadata\ApiResource;
use ApiPlatform\Metadata\Get;
use ApiPlatform\Metadata\GetCollection;
#[ApiResource(operations: [
    new Get(deprecationReason: 'use /books/v2 instead'),
    new GetCollection(),
])]
class Book {}
`
	dep := epEntity(t, "custom_php_api_platform", "src/Entity/Book.php", src, "GET /books/{id}")
	if dep.Properties["deprecated"] != "true" {
		t.Fatalf("GET /books/{id} deprecated=%q, want true (props: %v)", dep.Properties["deprecated"], dep.Properties)
	}
	if got := dep.Properties["deprecation_source"]; got != "deprecationReason" {
		t.Errorf("deprecation_source=%q, want 'deprecationReason'", got)
	}
	if got := dep.Properties["deprecated_replacement"]; got != "/books/v2" {
		t.Errorf("deprecated_replacement=%q, want /books/v2", got)
	}

	// Negative: the non-deprecated GetCollection op carries no deprecation.
	live := epEntity(t, "custom_php_api_platform", "src/Entity/Book.php", src, "GET /books")
	if _, ok := live.Properties["deprecated"]; ok {
		t.Fatalf("GET /books deprecation fabricated, want absent (props: %v)", live.Properties)
	}
}

// API Platform resource-wide deprecationReason on #[ApiResource] deprecates every
// generated CRUD operation.
func TestAPIPlatformDeprecation_ResourceWide(t *testing.T) {
	src := `<?php
namespace App\Entity;
use ApiPlatform\Metadata\ApiResource;
#[ApiResource(deprecationReason: 'use /books/v2 instead')]
class Book {}
`
	get := epEntity(t, "custom_php_api_platform", "src/Entity/Book.php", src, "GET /books/{id}")
	if get.Properties["deprecated"] != "true" {
		t.Fatalf("GET /books/{id} deprecated=%q, want true (resource-wide)", get.Properties["deprecated"])
	}
	if got := get.Properties["deprecated_replacement"]; got != "/books/v2" {
		t.Errorf("deprecated_replacement=%q, want /books/v2", got)
	}
	coll := epEntity(t, "custom_php_api_platform", "src/Entity/Book.php", src, "GET /books")
	if coll.Properties["deprecated"] != "true" {
		t.Fatalf("GET /books deprecated=%q, want true (resource-wide deprecation covers all ops)", coll.Properties["deprecated"])
	}
}

// Honest-partial: a non-deprecated API Platform resource carries no deprecation.
func TestAPIPlatformDeprecation_NonDeprecated(t *testing.T) {
	src := `<?php
namespace App\Entity;
use ApiPlatform\Metadata\ApiResource;
#[ApiResource]
class Book {}
`
	e := epEntity(t, "custom_php_api_platform", "src/Entity/Book.php", src, "GET /books/{id}")
	if got, ok := e.Properties["deprecated"]; ok {
		t.Fatalf("deprecated=%q fabricated on plain resource, want absent", got)
	}
}
