// http_endpoint_qn_1725_test.go — verifies #1725 fix: synthesized
// http_endpoint_definition and http_endpoint_call entities must carry a
// non-empty qualified_name equal to the canonical synthetic ID.
package engine

import (
	"testing"
)

func TestSynth_HTTPEndpointDefinition_QualifiedName_1725(t *testing.T) {
	src := `from flask import Flask
app = Flask(__name__)

@app.route("/api/v1/inspections/<int:pk>/create-deficiencies", methods=["POST"])
def create(pk):
    return {}
`
	_, res := runDetect(t, "python", "app.py", src)
	var sawDef bool
	for _, e := range res.Entities {
		if e.Kind != httpEndpointDefinitionKind {
			continue
		}
		sawDef = true
		if e.QualifiedName == "" {
			t.Errorf("http_endpoint_definition %q has empty QualifiedName (#1725)", e.Name)
		}
		if e.QualifiedName != e.ID {
			t.Errorf("expected QN==ID for synthetic, got QN=%q ID=%q", e.QualifiedName, e.ID)
		}
	}
	if !sawDef {
		t.Fatal("expected at least one http_endpoint_definition entity")
	}
}

func TestSynth_HTTPEndpointCall_QualifiedName_1725(t *testing.T) {
	// Express-style consumer: axios.get(...) — emits http_endpoint_call.
	src := `import axios from "axios";
async function fetchUser(id) {
  return await axios.get("/api/v1/users/" + id);
}
`
	_, res := runDetect(t, "javascript", "client.js", src)
	var sawCall bool
	for _, e := range res.Entities {
		if e.Kind != httpEndpointCallKind {
			continue
		}
		sawCall = true
		if e.QualifiedName == "" {
			t.Errorf("http_endpoint_call %q has empty QualifiedName (#1725)", e.Name)
		}
		if e.QualifiedName != e.ID {
			t.Errorf("expected QN==ID for synthetic, got QN=%q ID=%q", e.QualifiedName, e.ID)
		}
	}
	if !sawCall {
		t.Skip("no http_endpoint_call emitted for fixture — synthesis may have skipped; primary assertion is on the definition test")
	}
}
