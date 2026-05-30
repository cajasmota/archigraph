package java

import (
	"testing"
)

// ============================================================================
// Issue #3256: Play Framework Java response_shape_extraction
// ============================================================================
//
// Play Java controllers return play.mvc.Result values via the Results factory
// methods inherited from Controller (or imported via static import):
//
//   ok(body)          → 200 OK
//   created(location) → 201 Created
//   accepted()        → 202 Accepted
//   noContent()       → 204 No Content
//   badRequest(body)  → 400 Bad Request
//   notFound(body)    → 404 Not Found
//   forbidden(body)   → 403 Forbidden
//   redirect(url)     → 3xx Redirect
//   status(code, ...) → custom HTTP status
//
// Registry target: lang.java.framework.play Substrate/response_shape_extraction → partial
// Cite: internal/custom/java/play_routes.go

// playResponseShapeFixture is a realistic Play controller with multiple
// Result factory call sites: 200, 201, 400, 404, and 302.
const playResponseShapeFixture = `package controllers;

import play.mvc.Controller;
import play.mvc.Result;
import play.mvc.Results;
import play.data.Form;
import play.data.FormFactory;
import javax.inject.Inject;
import models.ItemForm;

public class ItemController extends Controller {

    @Inject
    private FormFactory formFactory;

    public Result index() {
        return ok("item list");
    }

    public Result show(Long id) {
        if (id == null) {
            return notFound("item not found");
        }
        return ok("item " + id);
    }

    public Result create() {
        Form<ItemForm> form = formFactory.form(ItemForm.class).bindFromRequest();
        if (form.hasErrors()) {
            return badRequest(form.errorsAsJson());
        }
        return created("/items/" + form.get().getId());
    }

    public Result delete(Long id) {
        return noContent();
    }

    public Result login() {
        return redirect(routes.HomeController.index());
    }

    public Result custom() {
        return status(418, "I'm a teapot");
    }
}
`

// TestPlay_ResponseShapeExtraction_FactoryMethods_Issue3256 proves that
// Play Result factory call sites (ok, created, badRequest, notFound, noContent,
// redirect, status) are extracted as SCOPE.Reference response_shape entities.
//
// Registry target: lang.java.framework.play Substrate/response_shape_extraction → partial
func TestPlay_ResponseShapeExtraction_FactoryMethods_Issue3256(t *testing.T) {
	r := ExtractPlay(PatternContext{
		Source:    playResponseShapeFixture,
		Language:  "java",
		Framework: "play",
		FilePath:  "app/controllers/ItemController.java",
	})

	factories := make(map[string]bool)
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_PLAY_RESULT_FACTORY" {
			if e.Kind != "SCOPE.Reference" {
				t.Errorf("[#3256 response_shape] expected SCOPE.Reference, got %s", e.Kind)
			}
			if e.Subtype != "response_shape" {
				t.Errorf("[#3256 response_shape] expected subtype=response_shape, got %s", e.Subtype)
			}
			if e.Properties["framework"] != "play" {
				t.Errorf("[#3256 response_shape] expected framework=play, got %v", e.Properties["framework"])
			}
			if m, ok := e.Properties["result_factory"].(string); ok {
				factories[m] = true
			}
		}
	}

	for _, want := range []string{"ok", "notFound", "badRequest", "created", "noContent", "redirect", "status"} {
		if !factories[want] {
			t.Errorf("[#3256 response_shape] expected result factory %q, got %v", want, factories)
		}
	}
}

// TestPlay_ResponseShapeExtraction_ControllerClass_Issue3256 proves that the
// controller_class property is set on the response_shape entity.
func TestPlay_ResponseShapeExtraction_ControllerClass_Issue3256(t *testing.T) {
	src := `package controllers;

import play.mvc.Controller;
import play.mvc.Result;

public class ProductController extends Controller {
    public Result list() {
        return ok("products");
    }
}
`
	r := ExtractPlay(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "play",
		FilePath:  "app/controllers/ProductController.java",
	})

	found := false
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_PLAY_RESULT_FACTORY" {
			cls, _ := e.Properties["controller_class"].(string)
			if cls == "ProductController" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("[#3256 response_shape] expected controller_class=ProductController on response_shape entity")
	}
}

// TestPlay_ResponseShapeExtraction_AsyncController_Issue3256 proves that async
// controllers (returning CompletionStage<Result>) also have response-shape
// detection for their internal factory calls.
func TestPlay_ResponseShapeExtraction_AsyncController_Issue3256(t *testing.T) {
	src := `package controllers;

import play.mvc.Controller;
import play.mvc.Result;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.CompletableFuture;

public class AsyncController extends Controller {
    public CompletionStage<Result> fetchData() {
        return CompletableFuture.supplyAsync(() -> ok("data"));
    }

    public CompletionStage<Result> failFast() {
        return CompletableFuture.completedFuture(internalServerError("boom"));
    }
}
`
	r := ExtractPlay(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "play",
		FilePath:  "app/controllers/AsyncController.java",
	})

	factories := make(map[string]bool)
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_PLAY_RESULT_FACTORY" {
			if m, ok := e.Properties["result_factory"].(string); ok {
				factories[m] = true
			}
		}
	}
	if !factories["ok"] {
		t.Errorf("[#3256 response_shape] expected ok() in async controller, got %v", factories)
	}
	if !factories["internalServerError"] {
		t.Errorf("[#3256 response_shape] expected internalServerError() in async controller, got %v", factories)
	}
}

// TestPlay_ResponseShapeExtraction_StaticImport_Issue3256 proves that response
// shapes are detected when Results methods are used directly (from Controller
// inheritance or static import) without a class qualifier.
func TestPlay_ResponseShapeExtraction_StaticImport_Issue3256(t *testing.T) {
	src := `package controllers;

import play.mvc.Controller;
import play.mvc.Result;
import static play.mvc.Results.ok;
import static play.mvc.Results.forbidden;

public class AuthedController extends Controller {
    public Result secret(String userId) {
        if (userId == null) {
            return forbidden("access denied");
        }
        return ok("secret content");
    }
}
`
	r := ExtractPlay(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "play",
		FilePath:  "app/controllers/AuthedController.java",
	})

	factories := make(map[string]bool)
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_PLAY_RESULT_FACTORY" {
			if m, ok := e.Properties["result_factory"].(string); ok {
				factories[m] = true
			}
		}
	}
	if !factories["ok"] {
		t.Errorf("[#3256 response_shape] expected ok() with static import, got %v", factories)
	}
	if !factories["forbidden"] {
		t.Errorf("[#3256 response_shape] expected forbidden() with static import, got %v", factories)
	}
}

// TestPlay_ResponseShapeExtraction_WrongFramework_Issue3256 confirms the
// extractor does not emit response_shape entities for non-play frameworks.
func TestPlay_ResponseShapeExtraction_WrongFramework_Issue3256(t *testing.T) {
	r := ExtractPlay(PatternContext{
		Source:    playResponseShapeFixture,
		Language:  "java",
		Framework: "spring_boot",
		FilePath:  "app/controllers/ItemController.java",
	})
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_PLAY_RESULT_FACTORY" {
			t.Errorf("[#3256 response_shape] expected no response_shape for framework=spring_boot, got %v", e)
		}
	}
}

// TestPlay_ResponseShapeExtraction_RoutesFileSkipped_Issue3256 confirms that
// response-shape extraction does not fire for conf/routes files.
func TestPlay_ResponseShapeExtraction_RoutesFileSkipped_Issue3256(t *testing.T) {
	src := `GET  /ok  controllers.HomeController.index()
`
	r := ExtractPlay(PatternContext{
		Source:    src,
		Language:  "java",
		Framework: "play",
		FilePath:  "conf/routes",
	})
	for _, e := range r.Entities {
		if e.Provenance == "INFERRED_FROM_PLAY_RESULT_FACTORY" {
			t.Errorf("[#3256 response_shape] expected no response_shape for routes file, got %v", e)
		}
	}
}
