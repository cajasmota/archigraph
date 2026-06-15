// Package python_test — issue #2654 discriminator-pattern extraction tests.
//
// Verifies that comparison_operator nodes of the form `identifier == literal`
// are detected in function/method bodies and stamped as
// Properties["discriminators"] on the enclosing entity.
//
// One case for Python equality:
//   - TestDiscriminator_PythonEquality — `if status == 'paid':` → "status=paid"
package python_test

import (
	"strings"
	"testing"

	"github.com/cajasmota/grafel/internal/types"
)

// pyDiscriminatorProp returns the "discriminators" property of the entity named
// entityName, or "" when the entity is not found or the property is absent.
func pyDiscriminatorProp(ents []types.EntityRecord, entityName string) string {
	for i := range ents {
		if ents[i].Name == entityName {
			return ents[i].Properties["discriminators"]
		}
	}
	return ""
}

// TestDiscriminator_PythonEquality verifies that an equality comparison
// (`if status == 'paid':`) inside a Python function body is captured as
// discriminator "status=paid" on the enclosing function entity.
func TestDiscriminator_PythonEquality(t *testing.T) {
	src := `
def process_payment(status):
    if status == 'paid':
        return True
    return False
`
	ents := extractPy(t, src, "payments/views.py")
	got := pyDiscriminatorProp(ents, "process_payment")
	if got == "" {
		t.Fatalf("process_payment: discriminators property is empty; want 'status=paid'")
	}
	if !strings.Contains(got, "status=paid") {
		t.Errorf("process_payment: discriminators=%q, want it to contain 'status=paid'", got)
	}
}

// TestDiscriminator_PythonNumericEquality verifies that a numeric equality
// comparison (`if code == 404:`) is captured as discriminator "code=404".
func TestDiscriminator_PythonNumericEquality(t *testing.T) {
	src := `
def handle_response(code):
    if code == 404:
        return 'not_found'
    return 'ok'
`
	ents := extractPy(t, src, "api/views.py")
	got := pyDiscriminatorProp(ents, "handle_response")
	if got == "" {
		t.Fatalf("handle_response: discriminators property is empty; want 'code=404'")
	}
	if !strings.Contains(got, "code=404") {
		t.Errorf("handle_response: discriminators=%q, want it to contain 'code=404'", got)
	}
}
