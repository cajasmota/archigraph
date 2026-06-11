package java

import "testing"

// issue4390_sut_disambiguation_test.go — proving tests for issue #4390.
//
// When a Java test class injects MULTIPLE candidate fields (the SUT plus
// @Mock/@MockBean collaborators, or several @Autowired beans), the TESTS→subject
// resolver must pick the ONE system-under-test and exclude the collaborators.
//
// Disambiguation priority (resolveJavaTestSubjectDetail):
//
//	@InjectMocks ▸ stem-match ▸ single non-mock field ▸ none
//
// Cite: internal/custom/java/junit5.go (resolveJavaTestSubject /
// resolveJavaTestSubjectDetail).

func TestIssue4390_InjectMocks_PicksSUT_ExcludesMockCollaborators(t *testing.T) {
	const src = `
@ExtendWith(MockitoExtension.class)
class OrderServiceTest {
    @InjectMocks OrderService sut;
    @Mock PaymentClient pay;
    @Mock InventoryRepo inv;

    @Test void places() {}
}
`
	res := resolveJavaTestSubjectDetail(src, "OrderServiceTest")
	if res.subject != "OrderService" {
		t.Fatalf("@InjectMocks SUT: got %q, want OrderService", res.subject)
	}
	if res.tier != "injectmocks" {
		t.Errorf("tier: got %q, want injectmocks", res.tier)
	}
	// Collaborators must NOT be linkable subjects.
	for _, collab := range []string{"PaymentClient", "InventoryRepo"} {
		if res.subject == collab {
			t.Errorf("collaborator %q wrongly selected as SUT", collab)
		}
	}
}

// @InjectMocks overrides stem affinity: even when the test-class stem matches a
// MOCK collaborator's type, the @InjectMocks field is the SUT.
func TestIssue4390_InjectMocks_OverridesStemMatchingCollaborator(t *testing.T) {
	const src = `
class PaymentClientTest {
    @InjectMocks OrderService sut;
    @Mock PaymentClient pay;

    @Test void t() {}
}
`
	// Stem "PaymentClient" matches the @Mock collaborator's type, but it is a
	// mock → excluded. @InjectMocks OrderService is the SUT.
	res := resolveJavaTestSubjectDetail(src, "PaymentClientTest")
	if res.subject != "OrderService" {
		t.Fatalf("got %q, want OrderService (@InjectMocks overrides mock-stem)", res.subject)
	}
}

// No @InjectMocks: stem-affinity disambiguates among multiple @Autowired fields.
func TestIssue4390_StemMatch_AmongMultipleAutowired(t *testing.T) {
	const src = `
@SpringBootTest
class OrderServiceTest {
    @Autowired OrderService svc;
    @Autowired Clock clock;

    @Test void t() {}
}
`
	res := resolveJavaTestSubjectDetail(src, "OrderServiceTest")
	if res.subject != "OrderService" {
		t.Fatalf("stem-match: got %q, want OrderService", res.subject)
	}
	if res.tier != "stem_match" {
		t.Errorf("tier: got %q, want stem_match", res.tier)
	}
}

// Ambiguous: two equally-plausible @Autowired candidates, no @InjectMocks, and
// the stem matches NEITHER → emit no spurious SUT edge.
func TestIssue4390_Ambiguous_NoSpuriousEdge(t *testing.T) {
	const src = `
@SpringBootTest
class CheckoutFlowTest {
    @Autowired OrderService svc;
    @Autowired PaymentClient pay;

    @Test void t() {}
}
`
	res := resolveJavaTestSubjectDetail(src, "CheckoutFlowTest")
	if res.subject != "" {
		t.Fatalf("ambiguous case wrongly produced SUT %q (tier=%s)", res.subject, res.tier)
	}
}

// Single non-mock injected field, no stem affinity → that lone field is the SUT.
func TestIssue4390_SingleField_NoStem(t *testing.T) {
	const src = `
class CheckoutFlowSpec {
    @Autowired OrderService svc;
    @Mock PaymentClient pay;

    @Test void t() {}
}
`
	// "CheckoutFlowSpec" yields no Test/Tests/IT stem; OrderService is the only
	// non-mock candidate → single_field.
	res := resolveJavaTestSubjectDetail(src, "CheckoutFlowSpec")
	if res.subject != "OrderService" {
		t.Fatalf("single-field: got %q, want OrderService", res.subject)
	}
	if res.tier != "single_field" {
		t.Errorf("tier: got %q, want single_field", res.tier)
	}
}

// A @Mock field whose type matches the stem must NOT be linked (collaborator
// exclusion), even when it is the only injected field.
func TestIssue4390_MockMatchingStem_NotLinked(t *testing.T) {
	const src = `
class OrderServiceTest {
    @Mock OrderService svc;

    @Test void t() {}
}
`
	res := resolveJavaTestSubjectDetail(src, "OrderServiceTest")
	if res.subject != "" {
		t.Fatalf("mock matching stem wrongly linked as SUT: %q", res.subject)
	}
}

// Backwards-compatible: the classic single @InjectMocks + stem case still links.
func TestIssue4390_ClassicInjectMocks_StillLinks(t *testing.T) {
	const src = `
class OrderServiceTest {
    @Mock OrderRepository repo;
    @InjectMocks OrderService subject;

    @Test void t() {}
}
`
	if got := resolveJavaTestSubject(src, "OrderServiceTest"); got != "OrderService" {
		t.Fatalf("classic InjectMocks: got %q, want OrderService", got)
	}
}
