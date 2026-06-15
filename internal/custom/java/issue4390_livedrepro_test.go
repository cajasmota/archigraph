package java_test

import (
	"testing"

	_ "github.com/cajasmota/grafel/internal/custom/java"
	"github.com/cajasmota/grafel/internal/types"
)

// Issue #4390 LIVE-REPRO — SUT disambiguation through the REAL registered Java
// custom extractor.
//
// A multi-collaborator Spring/Mockito test injects the SUT plus several
// @Mock/@MockBean collaborators. The TESTS edge must point at the ONE SUT and at
// NO collaborator. Extends the #4359/#4615 coverage linkage.

// testsTargets returns the set of `Class:<X>` targets of every TESTS edge.
func testsTargets(ents []types.EntityRecord) map[string]bool {
	out := map[string]bool{}
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == string(types.RelationshipKindTests) {
				out[r.ToID] = true
			}
		}
	}
	return out
}

func TestIssue4390_Live_InjectMocks_OnlySUTLinked_NotCollaborators(t *testing.T) {
	const src = `
package com.example.order;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import static org.junit.jupiter.api.Assertions.*;

@ExtendWith(MockitoExtension.class)
class OrderServiceTest {

    @InjectMocks OrderService sut;
    @Mock PaymentClient pay;
    @Mock InventoryRepo inv;

    @Test
    void places() {
        assertNotNull(sut);
    }
}
`
	ents := javaCustomExtract(t, "src/test/java/com/example/order/OrderServiceTest.java", src)
	targets := testsTargets(ents)

	if !targets["Class:OrderService"] {
		t.Fatalf("expected TESTS→Class:OrderService; got targets=%v", targets)
	}
	for _, collab := range []string{"Class:PaymentClient", "Class:InventoryRepo"} {
		if targets[collab] {
			t.Errorf("mock collaborator wrongly linked: %s (#4390)", collab)
		}
	}
}

func TestIssue4390_Live_StemMatch_AmongAutowired(t *testing.T) {
	const src = `
package com.example.order;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;

@SpringBootTest
class OrderServiceTest {

    @Autowired OrderService svc;
    @Autowired java.time.Clock clock;

    @Test
    void runs() {}
}
`
	ents := javaCustomExtract(t, "src/test/java/com/example/order/OrderServiceTest.java", src)
	targets := testsTargets(ents)
	if !targets["Class:OrderService"] {
		t.Fatalf("expected stem-match TESTS→Class:OrderService; got %v", targets)
	}
	if targets["Class:Clock"] {
		t.Errorf("collaborator Clock wrongly linked (#4390)")
	}
}

func TestIssue4390_Live_Ambiguous_NoEdge(t *testing.T) {
	const src = `
package com.example.order;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;

@SpringBootTest
class CheckoutFlowTest {

    @Autowired OrderService svc;
    @Autowired PaymentClient pay;

    @Test
    void runs() {}
}
`
	ents := javaCustomExtract(t, "src/test/java/com/example/order/CheckoutFlowTest.java", src)
	targets := testsTargets(ents)
	for tgt := range targets {
		// Only the SUT-class TESTS edges are in scope here (no route calls).
		if tgt == "Class:OrderService" || tgt == "Class:PaymentClient" {
			t.Errorf("ambiguous test wrongly linked SUT %s (#4390)", tgt)
		}
	}
}
