package substrate

import "testing"

// TestTaintSniffer_JSTS_DirectSinks confirms the JS/TS sniffer
// recognises the canonical req.body source, the eval / new Function
// sink, and a parameterised-query sanitizer in the same file.
func TestTaintSniffer_JSTS_DirectSinks(t *testing.T) {
	src := `
function handler(req, res) {
  const q = req.body.q;
  db.query("SELECT * FROM t WHERE x = ?", [q]);  // sanitizer
  db.query("SELECT * FROM t WHERE x = " + q);     // sink (concat)
  eval(q);                                         // sink (command)
}
`
	got := sniffTaintJSTS(src)
	if len(got) == 0 {
		t.Fatal("expected matches; got 0")
	}
	have := map[TaintKind]int{}
	for _, m := range got {
		have[m.Kind]++
		if m.Function != "handler" {
			t.Errorf("match %+v not attributed to handler", m)
		}
	}
	if have[TaintKindSource] == 0 {
		t.Error("expected at least one source match")
	}
	if have[TaintKindSink] == 0 {
		t.Error("expected at least one sink match")
	}
	if have[TaintKindSanitizer] == 0 {
		t.Error("expected at least one sanitizer match")
	}
}

// TestTaintSniffer_Python_LiteralOpenIsNotASink documents that
// open("/etc/passwd") with a literal path is NOT flagged as a path-
// traversal sink — only the non-literal first-arg shape is.
func TestTaintSniffer_Python_LiteralOpenIsNotASink(t *testing.T) {
	src := `
def read_config():
    open("/etc/myapp/config.yml")  # benign: literal path
`
	for _, m := range sniffTaintPython(src) {
		if m.Kind == TaintKindSink && m.Category == TaintCategoryPath {
			t.Errorf("literal open() was flagged as path sink: %+v", m)
		}
	}
}

// TestTaintSniffer_Java_RecognisesSpringAnnotations confirms the
// @RequestParam / @RequestBody parameter annotations are surfaced as
// sources. Spring-style controllers are the dominant Java HTTP shape.
func TestTaintSniffer_Java_RecognisesSpringAnnotations(t *testing.T) {
	src := `
@RestController
public class UserController {
  @GetMapping("/users")
  public String list(@RequestParam String q) {
    return q;
  }
}
`
	var found bool
	for _, m := range sniffTaintJava(src) {
		if m.Kind == TaintKindSource && m.Primitive == "@RequestParam/@PathVariable/@RequestBody" {
			found = true
		}
	}
	if !found {
		t.Error("expected @RequestParam to be flagged as a source")
	}
}

// TestTaintSniffer_Go_ParameterisedQueryIsSanitizer asserts that a
// placeholder-based db.Query call counts as a sanitizer and not as a
// sink.
func TestTaintSniffer_Go_ParameterisedQueryIsSanitizer(t *testing.T) {
	src := `
package x

func get(id string) {
	db.Query("SELECT * FROM u WHERE id = ?", id)
}
`
	var (
		hasSan  bool
		hasSink bool
	)
	for _, m := range sniffTaintGo(src) {
		if m.Kind == TaintKindSanitizer && m.Category == TaintCategorySQL {
			hasSan = true
		}
		if m.Kind == TaintKindSink && m.Category == TaintCategorySQL {
			hasSink = true
		}
	}
	if !hasSan {
		t.Error("expected parameterised db.Query to be tagged as SQL sanitizer")
	}
	if hasSink {
		t.Error("parameterised db.Query must not be a SQL sink")
	}
}
