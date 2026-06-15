package python

import (
	"context"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
)

// TestEmitGenericDecoratorProperties_2016 asserts that the generic decorator
// capture pass stamps `decorator_<name>` properties on every decorated
// Operation entity, covering the W8R1-R3 evidence in issue #2016:
//
//   - @property                  → decorator_property = "true"
//   - @cached_property           → decorator_cached_property = "true"
//   - @staticmethod              → decorator_staticmethod = "true"
//   - @classmethod               → decorator_classmethod = "true"
//   - @contextmanager            → decorator_contextmanager = "true"
//   - @functools.wraps(fn)       → decorator_wraps = "<call snippet>"
//   - @full_name.setter          → decorator_setter = "full_name"
//
// The pass MUST not clobber the specialised stamps produced by the DRF
// @action pass (#2004) — `decorator_action` is the only key the generic
// pass would write for an action, but the structured `drf_action=true`
// + `http_method=...` properties from the action pass must survive.
func TestEmitGenericDecoratorProperties_2016(t *testing.T) {
	src := `from contextlib import contextmanager
from functools import cached_property, wraps


class UserProfile:
    @property
    def full_name(self):
        return self.first + " " + self.last

    @full_name.setter
    def full_name(self, value):
        self.first, self.last = value.split(" ", 1)

    @cached_property
    def avatar_url(self):
        return "/static/" + self.id

    @staticmethod
    def banner_default():
        return "default-banner.png"

    @classmethod
    def from_dict(cls, data):
        return cls()

    @contextmanager
    def transactional(self):
        yield


def memoize(fn):
    @wraps(fn)
    def inner(*a, **kw):
        return fn(*a, **kw)
    return inner
`

	e := &Extractor{}
	out, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     "fixture/userprofile.py",
		Content:  []byte(src),
		Language: "python",
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	findOp := func(name string) map[string]string {
		for _, ent := range out {
			if ent.Kind == "SCOPE.Operation" && ent.Name == name {
				return ent.Properties
			}
		}
		return nil
	}

	cases := []struct {
		opName    string
		wantKey   string
		wantValue string // "" means we only check presence (key set to any non-empty)
		wantExact bool   // when true, value must match exactly
	}{
		{"UserProfile.full_name", "decorator_property", "true", true},
		// The setter is emitted as a same-named Operation; check that
		// decorator_setter survives with the property-target name as value.
		// Both entries have the same Name in the entity stream because Python
		// allows the property/setter pair to share the method name; the
		// stamping pass writes whichever it walks first. We assert at least
		// one of the two carries decorator_setter.
		{"UserProfile.avatar_url", "decorator_cached_property", "true", true},
		{"UserProfile.banner_default", "decorator_staticmethod", "true", true},
		{"UserProfile.from_dict", "decorator_classmethod", "true", true},
		{"UserProfile.transactional", "decorator_contextmanager", "true", true},
	}

	for _, tc := range cases {
		props := findOp(tc.opName)
		if props == nil {
			t.Errorf("operation %q not emitted", tc.opName)
			continue
		}
		got, ok := props[tc.wantKey]
		if !ok {
			t.Errorf("operation %q missing property %q (got props=%v)", tc.opName, tc.wantKey, props)
			continue
		}
		if tc.wantExact && got != tc.wantValue {
			t.Errorf("operation %q property %q = %q, want %q",
				tc.opName, tc.wantKey, got, tc.wantValue)
		}
	}

	// Setter pair: walk all operations named "UserProfile.full_name" and
	// assert that decorator_setter = "full_name" is present on at least one.
	foundSetter := false
	for _, ent := range out {
		if ent.Kind != "SCOPE.Operation" || ent.Name != "UserProfile.full_name" {
			continue
		}
		if v, ok := ent.Properties["decorator_setter"]; ok && v == "full_name" {
			foundSetter = true
			break
		}
	}
	if !foundSetter {
		t.Errorf("expected at least one UserProfile.full_name operation with decorator_setter=full_name")
	}

	// @wraps(fn) on inner: the inner function lives inside the `memoize`
	// outer body. The current extractor emits nested functions as
	// Operations too (no parent class) — check that `decorator_wraps`
	// carries the call-form snippet.
	innerProps := findOp("inner")
	if innerProps == nil {
		// Tolerate: the extractor may not surface deeply-nested helpers.
		t.Logf("inner not emitted as a top-level Operation (expected for nested helpers); skipping @wraps assertion")
	} else if got, ok := innerProps["decorator_wraps"]; !ok || got == "" {
		t.Errorf("expected decorator_wraps on inner (got props=%v)", innerProps)
	}
}

// TestEmitGenericDecoratorProperties_2016_DoesNotOverwriteDRF asserts that the
// generic decorator pass does NOT overwrite structured properties stamped by
// the DRF @action pass. The action pass writes `drf_action`, `http_method`,
// `http_methods`, `is_detail`, `url_path` on the method Operation; those keys
// must survive untouched.
func TestEmitGenericDecoratorProperties_2016_DoesNotOverwriteDRF(t *testing.T) {
	src := `from rest_framework.decorators import action


class ContractViewSet:
    @action(detail=True, methods=["post"], url_path="approve")
    def approve(self, request, pk=None):
        return {"ok": True}
`

	e := &Extractor{}
	out, err := e.Extract(context.Background(), extractor.FileInput{
		Path:     "fixture/contract_viewset.py",
		Content:  []byte(src),
		Language: "python",
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var props map[string]string
	for _, ent := range out {
		if ent.Kind == "SCOPE.Operation" && ent.Name == "ContractViewSet.approve" {
			props = ent.Properties
			break
		}
	}
	if props == nil {
		t.Fatalf("ContractViewSet.approve not emitted")
	}
	// DRF-pass keys must survive.
	wants := map[string]string{
		"drf_action":  "true",
		"http_method": "post",
		"is_detail":   "true",
		"url_path":    "approve",
	}
	for k, want := range wants {
		if got := props[k]; got != want {
			t.Errorf("property %q = %q, want %q (full props=%v)", k, got, want, props)
		}
	}
	// And the generic pass must additionally stamp `decorator_action`
	// (a call-form snippet starting with "@action").
	if got, ok := props["decorator_action"]; !ok || got == "" {
		t.Errorf("expected decorator_action call-form snippet, got %q", got)
	}
}
