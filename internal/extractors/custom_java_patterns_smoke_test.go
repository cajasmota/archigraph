package extractors

import (
	"context"
	"testing"

	"github.com/cajasmota/archigraph/internal/types"
)

// custom_java_patterns_smoke_test.go — end-to-end proof for #3586.
//
// These tests drive the FULL live custom-extractor dispatch
// (RunCustomExtractors → CustomExtractorsFor("java") → prefix selection →
// custom_java_patterns.Extract) on representative Java sources and assert that
// SPECIFIC entities and relationships now reach the graph through the live path.
//
// Before this PR the 35 Extract*(ctx PatternContext) PatternResult functions had
// zero non-test callers; PatternContext was only built in unit tests, so none of
// these entities/relationships were ever emitted by the indexer. A passing
// assertion here therefore proves the dead layer is genuinely wired, not merely
// unit-callable. Assertions are value-specific (exact Kind/Name and an exact
// relationship Kind+ToID), never len > 0.

// runJavaPatterns dispatches the live java custom-extractor pass and returns the
// emitted records. It asserts the custom_java_patterns extractor is actually
// selected for "java" so a regression in dispatch wiring fails loudly.
func runJavaPatterns(t *testing.T, path, content string) []types.EntityRecord {
	t.Helper()

	selected := false
	for _, e := range CustomExtractorsFor("java") {
		if e.Language() == "custom_java_patterns" {
			selected = true
			break
		}
	}
	if !selected {
		t.Fatalf("custom_java_patterns is NOT selected by CustomExtractorsFor(\"java\") — " +
			"the pattern dispatch extractor would never run live")
	}

	ents, errs := RunCustomExtractors(context.Background(), FileInput{
		Path:     path,
		Language: "java",
		Content:  []byte(content),
	})
	for _, err := range errs {
		t.Fatalf("custom dispatch returned error for %s: %v", path, err)
	}
	return ents
}

// findRecord returns the first record matching kind+name, or nil.
func findRecord(recs []types.EntityRecord, kind, name string) *types.EntityRecord {
	for i := range recs {
		if recs[i].Kind == kind && recs[i].Name == name {
			return &recs[i]
		}
	}
	return nil
}

// hasRel reports whether rec carries an embedded relationship of the given kind
// to the given ToID (structural ref).
func hasRel(rec *types.EntityRecord, kind, toID string) bool {
	if rec == nil {
		return false
	}
	for _, r := range rec.Relationships {
		if r.Kind == kind && r.ToID == toID {
			return true
		}
	}
	return false
}

// TestJavaPatternsSpringControllerLive proves Spring Boot DI + request-mapping
// extraction reaches the graph through the live dispatch path.
func TestJavaPatternsSpringControllerLive(t *testing.T) {
	src := `
package com.example.api;

import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.stereotype.Service;
import org.springframework.beans.factory.annotation.Autowired;

@RestController
@RequestMapping("/api/users")
public class UserController {

    @GetMapping("/{id}")
    public User getUser(Long id) {
        return null;
    }
}

@Service
public class UserService {

    @Autowired
    private UserRepository userRepository;
}
`
	recs := runJavaPatterns(t, "src/main/java/com/example/api/UserController.java", src)

	// 1. The HTTP endpoint operation must emit with method + resolved path.
	ep := findRecord(recs, "SCOPE.Operation", "UserController.getUser")
	if ep == nil {
		t.Fatalf("expected SCOPE.Operation UserController.getUser endpoint to emit live; got %v", names(recs))
	}
	if got := ep.Properties["http_method"]; got != "GET" {
		t.Errorf("endpoint http_method = %q, want GET", got)
	}
	if got := ep.Properties["path"]; got != "/api/users/{id}" {
		t.Errorf("endpoint path = %q, want /api/users/{id}", got)
	}

	// 2. The @Service stereotype component must emit.
	svc := findRecord(recs, "SCOPE.Component", "UserService")
	if svc == nil {
		t.Fatalf("expected SCOPE.Component UserService stereotype to emit; got %v", names(recs))
	}
	if got := svc.Properties["stereotype"]; got != "service" {
		t.Errorf("UserService stereotype = %q, want service", got)
	}

	// 3. The @Autowired DI edge UserService -> UserRepository must emit as an
	//    embedded DEPENDS_ON relationship on the service's stereotype entity.
	wantTarget := "scope:dependency:spring_boot:" +
		"src/main/java/com/example/api/UserController.java:UserRepository"
	if !hasRel(svc, "DEPENDS_ON", wantTarget) {
		t.Errorf("expected DEPENDS_ON edge from UserService to UserRepository (%s); got rels %v",
			wantTarget, svc.Relationships)
	}
}

// TestJavaPatternsJpaEntityLive proves JPA/Hibernate entity + association
// extraction reaches the graph through the live dispatch path.
func TestJavaPatternsJpaEntityLive(t *testing.T) {
	src := `
package com.example.model;

import jakarta.persistence.Entity;
import jakarta.persistence.Table;
import jakarta.persistence.OneToMany;

@Entity
@Table(name = "orders")
public class Order {

    @OneToMany
    private List<LineItem> items;
}
`
	recs := runJavaPatterns(t, "src/main/java/com/example/model/Order.java", src)

	order := findRecord(recs, "SCOPE.Schema", "Order")
	if order == nil {
		t.Fatalf("expected SCOPE.Schema Order entity to emit live; got %v", names(recs))
	}
	if got := order.Properties["table_name"]; got != "orders" {
		t.Errorf("Order table_name = %q, want orders", got)
	}

	// The @OneToMany association Order -> LineItem must emit as a DEPENDS_ON edge.
	wantTarget := "scope:schema:hibernate_entity:" +
		"src/main/java/com/example/model/Order.java:LineItem"
	if !hasRel(order, "DEPENDS_ON", wantTarget) {
		t.Errorf("expected DEPENDS_ON association edge Order -> LineItem (%s); got rels %v",
			wantTarget, order.Relationships)
	}
}

// TestJavaPatternsAndroidActivityLive proves Android component extraction reaches
// the graph through the live dispatch path.
func TestJavaPatternsAndroidActivityLive(t *testing.T) {
	src := `
package com.example.app;

import android.app.Activity;
import android.os.Bundle;
import android.content.Intent;

public class MainActivity extends Activity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        Intent intent = new Intent(this, DetailActivity.class);
        startActivity(intent);
    }
}
`
	recs := runJavaPatterns(t, "app/src/main/java/com/example/app/MainActivity.java", src)

	// The Activity must emit as a SCOPE.UIComponent (subtype=component) screen.
	act := findRecord(recs, "SCOPE.UIComponent", "MainActivity")
	if act == nil {
		t.Fatalf("expected SCOPE.UIComponent MainActivity (Android Activity) to emit live; got %v", names(recs))
	}
	if prov := act.Properties["provenance"]; prov != "INFERRED_FROM_ANDROID_ACTIVITY" {
		t.Errorf("MainActivity provenance = %q, want INFERRED_FROM_ANDROID_ACTIVITY", prov)
	}

	// The explicit Intent(this, DetailActivity.class) navigation must emit as a
	// SCOPE.Operation navigation edge MainActivity->DetailActivity.
	nav := findRecord(recs, "SCOPE.Operation", "MainActivity->DetailActivity")
	if nav == nil {
		t.Fatalf("expected SCOPE.Operation MainActivity->DetailActivity intent navigation to emit live; got %v", names(recs))
	}
}

// names is a compact dump of (Kind,Name) pairs for failure messages.
func names(recs []types.EntityRecord) []string {
	out := make([]string, 0, len(recs))
	for _, r := range recs {
		out = append(out, r.Kind+"/"+r.Name)
	}
	return out
}
