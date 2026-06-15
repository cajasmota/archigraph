package java_test

import (
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func javaExcEdge(recs []types.EntityRecord, fromName, kind, typeName string) bool {
	want := extractor.ExceptionTypeTargetID(typeName)
	for i := range recs {
		if recs[i].Name != fromName {
			continue
		}
		for _, r := range recs[i].Relationships {
			if r.Kind == kind && r.ToID == want {
				return true
			}
		}
	}
	return false
}

func javaExcNodeCount(recs []types.EntityRecord, typeName string) int {
	want := extractor.ExceptionTypeName(typeName)
	c := 0
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) && recs[i].Name == want {
			c++
		}
	}
	return c
}

// TestJavaExceptionFlow_ThrowsAndCatchConverge: a `throws IOException` clause on
// one method and a `catch (IOException e)` in another converge on the SAME node.
func TestJavaExceptionFlow_ThrowsAndCatchConverge(t *testing.T) {
	src := `package demo;

public class Repo {
    public void read() throws IOException {
        throw new IOException("disk");
    }

    public void caller() {
        try {
            read();
        } catch (IOException e) {
            handle(e);
        }
    }
}
`
	recs := extractJavaRaw(t, src)

	// throws-clause THROWS and the explicit throw new both on read().
	if !javaExcEdge(recs, "Repo.read", "THROWS", "IOException") {
		t.Errorf("missing THROWS(Repo.read -> IOException)")
	}
	if !javaExcEdge(recs, "Repo.caller", "CATCHES", "IOException") {
		t.Errorf("missing CATCHES(Repo.caller -> IOException)")
	}
	if n := javaExcNodeCount(recs, "IOException"); n != 1 {
		t.Fatalf("throws + catch of IOException must converge on ONE node, got %d", n)
	}
}

// TestJavaExceptionFlow_MultiThrows: `throws A, B` yields a THROWS edge each.
func TestJavaExceptionFlow_MultiThrows(t *testing.T) {
	src := `package demo;
public class Svc {
    public void run() throws IOException, SQLException {
        doWork();
    }
}
`
	recs := extractJavaRaw(t, src)
	if !javaExcEdge(recs, "Svc.run", "THROWS", "IOException") {
		t.Errorf("missing THROWS(Svc.run -> IOException)")
	}
	if !javaExcEdge(recs, "Svc.run", "THROWS", "SQLException") {
		t.Errorf("missing THROWS(Svc.run -> SQLException)")
	}
}

// TestJavaExceptionFlow_MultiCatch: `catch (A | B e)` yields a CATCHES edge each.
func TestJavaExceptionFlow_MultiCatch(t *testing.T) {
	src := `package demo;
public class Svc {
    public void run() {
        try {
            doWork();
        } catch (IOException | SQLException e) {
            recover();
        }
    }
}
`
	recs := extractJavaRaw(t, src)
	if !javaExcEdge(recs, "Svc.run", "CATCHES", "IOException") {
		t.Errorf("missing CATCHES(Svc.run -> IOException)")
	}
	if !javaExcEdge(recs, "Svc.run", "CATCHES", "SQLException") {
		t.Errorf("missing CATCHES(Svc.run -> SQLException)")
	}
}

// TestJavaExceptionFlow_ThrowNew: `throw new IllegalArgumentException()`.
func TestJavaExceptionFlow_ThrowNew(t *testing.T) {
	src := `package demo;
public class Validator {
    public void check(int x) {
        if (x < 0) {
            throw new IllegalArgumentException("negative");
        }
    }
}
`
	recs := extractJavaRaw(t, src)
	if !javaExcEdge(recs, "Validator.check", "THROWS", "IllegalArgumentException") {
		t.Errorf("missing THROWS(Validator.check -> IllegalArgumentException)")
	}
}

// TestJavaExceptionFlow_RethrowVarDropped: NEGATIVE — `throw e;` re-throwing a
// variable carries no NEW type and must emit no THROWS edge/node.
func TestJavaExceptionFlow_RethrowVarDropped(t *testing.T) {
	src := `package demo;
public class Svc {
    public void run() {
        try {
            doWork();
        } catch (RuntimeException e) {
            throw e;
        }
    }
}
`
	recs := extractJavaRaw(t, src)
	// The CATCHES(RuntimeException) is expected; the bare `throw e` must NOT
	// add any extra exception node (only RuntimeException exists).
	for i := range recs {
		if recs[i].Kind == string(types.EntityKindExceptionType) &&
			recs[i].Name != "exception:RuntimeException" {
			t.Errorf("bare throw e fabricated a node: %q", recs[i].Name)
		}
	}
}
