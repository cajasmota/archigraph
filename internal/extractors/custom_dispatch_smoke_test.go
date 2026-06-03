package extractors

import (
	"context"
	"testing"
)

// hasEntity reports whether any record in recs has the given Kind and Name.
func hasEntity(recs []entityNameKind, kind, name string) bool {
	for _, e := range recs {
		if e.kind == kind && e.name == name {
			return true
		}
	}
	return false
}

type entityNameKind struct{ kind, name string }

// runSmoke dispatches the full custom-extractor pass for a language+file
// through the REAL registry (populated by custom_registry.go's blank imports)
// and returns the flattened (kind,name) pairs of every emitted entity.
//
// Crucially this exercises CustomExtractorsFor → prefix selection, exactly the
// path that silently dropped the bare-stem Ruby/PHP extractors before #3587/
// #3592. If an extractor is not wired to a dispatch prefix it contributes
// nothing here, so a specific-entity assertion fails — proving live wiring.
func runSmoke(t *testing.T, language, path, content string) []entityNameKind {
	t.Helper()
	ents, errs := RunCustomExtractors(context.Background(), FileInput{
		Path:     path,
		Language: language,
		Content:  []byte(content),
	})
	for _, err := range errs {
		t.Fatalf("custom dispatch returned error for %s/%s: %v", language, path, err)
	}
	out := make([]entityNameKind, 0, len(ents))
	for _, e := range ents {
		out = append(out, entityNameKind{kind: e.Kind, name: e.Name})
	}
	return out
}

// TestSmokeHttp4kHandlerAttributionFiresViaDispatch proves (#4018) that the
// http4k Kotlin route extractor fires through the REAL kotlin dispatch path on a
// live .kt file AND that the deepened handler_attribution (the `handler`
// property) survives the full RunCustomExtractors pass — not just the unit test.
func TestSmokeHttp4kHandlerAttributionFiresViaDispatch(t *testing.T) {
	src := `
val app = routes(
    "/api" bind routes(
        "/users" bind GET to ::listUsers,
        "/x" bind POST to { req -> Response(OK) },
    ),
)
`
	ents, errs := RunCustomExtractors(context.Background(), FileInput{
		Path:     "App.kt",
		Language: "kotlin",
		Content:  []byte(src),
	})
	for _, err := range errs {
		t.Fatalf("custom dispatch returned error: %v", err)
	}
	handlerFor := func(name string) (string, bool) {
		for _, e := range ents {
			if e.Kind == "SCOPE.Operation" && e.Name == name {
				return e.Properties["handler"], true
			}
		}
		return "", false
	}
	if h, ok := handlerFor("GET /api/users"); !ok || h != "::listUsers" {
		t.Errorf("http4k via dispatch: GET /api/users handler = %q (ok=%v); want ::listUsers", h, ok)
	}
	if h, ok := handlerFor("POST /api/x"); !ok || h != "lambda" {
		t.Errorf("http4k via dispatch: POST /api/x handler = %q (ok=%v); want lambda", h, ok)
	}
}

// TestSmokeRubyOrphansFireViaDispatch proves that the previously-orphaned Ruby
// framework extractors are now selected by language dispatch AND actually emit
// their expected entities when run through RunCustomExtractors("ruby", …).
func TestSmokeRubyOrphansFireViaDispatch(t *testing.T) {
	// ruby_auth (custom_ruby_auth): devise_for route auth pattern.
	authSrc := `
Rails.application.routes.draw do
  devise_for :users
end
`
	got := runSmoke(t, "ruby", "config/routes.rb", authSrc)
	if !hasEntity(got, "SCOPE.Pattern", "devise_for:users") {
		t.Errorf("custom_ruby_auth did not fire via dispatch: expected SCOPE.Pattern devise_for:users, got %v", got)
	}

	// ruby_routes (custom_ruby_routes): Grape resource component.
	routesSrc := `
class UsersAPI < Grape::API
  resource :users do
    get do
      User.all
    end
  end
end
`
	got = runSmoke(t, "ruby", "app/api/users_api.rb", routesSrc)
	if !hasEntity(got, "SCOPE.Component", "grape_resource:users") {
		t.Errorf("custom_ruby_routes did not fire via dispatch: expected SCOPE.Component grape_resource:users, got %v", got)
	}

	// ruby_validation (custom_ruby_validation): strong-params request validation.
	valSrc := `
class ArticlesController < ApplicationController
  def article_params
    params.require(:article).permit(:title, :body, :published)
  end
end
`
	got = runSmoke(t, "ruby", "app/controllers/articles_controller.rb", valSrc)
	if !hasEntity(got, "SCOPE.Pattern", "strong_params:params.require(:article)") {
		t.Errorf("custom_ruby_validation did not fire via dispatch: expected strong_params request_validation pattern, got %v", got)
	}
}

// TestSmokePhpOrphansFireViaDispatch proves the previously-orphaned PHP
// framework extractors are now selected by dispatch AND emit expected entities
// via RunCustomExtractors("php", …).
func TestSmokePhpOrphansFireViaDispatch(t *testing.T) {
	// php_doctrine_orm_data (custom_php_doctrine_orm_data): ORM column schema.
	doctrineSrc := `<?php
namespace App\Entity;
use Doctrine\ORM\Mapping as ORM;

#[ORM\Entity(repositoryClass: UserRepository::class)]
class User
{
    #[ORM\Column(type: 'string', length: 255)]
    private string $username;

    #[ORM\Column(type: 'string', unique: true)]
    private string $email;
}
`
	got := runSmoke(t, "php", "User.php", doctrineSrc)
	if !hasEntity(got, "SCOPE.Schema", "username") {
		t.Errorf("custom_php_doctrine_orm_data did not fire via dispatch: expected SCOPE.Schema username column, got %v", got)
	}
	if !hasEntity(got, "SCOPE.Schema", "email") {
		t.Errorf("custom_php_doctrine_orm_data did not fire via dispatch: expected SCOPE.Schema email column, got %v", got)
	}

	// php_pest_test (custom_php_pest_test): Pest test-case operations.
	pestSrc := `<?php
uses(Tests\TestCase::class);

it('can create a user', function () {
    $user = User::factory()->create();
    expect($user->id)->toBeInt();
});

test('user email is unique', function () {
    // ...
});
`
	got = runSmoke(t, "php", "UserTest.php", pestSrc)
	if !hasEntity(got, "SCOPE.Operation", "can create a user") {
		t.Errorf("custom_php_pest_test did not fire via dispatch: expected SCOPE.Operation 'can create a user', got %v", got)
	}
}
