package dashboard

import "testing"

// p builds an INJECTED_INTO edge property map.
func diProps(framework, provider, consumer, via, qualifier string) map[string]string {
	m := map[string]string{}
	if framework != "" {
		m["framework"] = framework
	}
	if provider != "" {
		m["provider"] = provider
	}
	if consumer != "" {
		m["consumer"] = consumer
	}
	if via != "" {
		m["via"] = via
	}
	if qualifier != "" {
		m["qualifier"] = qualifier
	}
	return m
}

// TestDIFoldGroupsAndDedups verifies the core fold: provider→consumers grouping,
// per-provider consumer de-duplication, framework grouping with DI-hubs-first
// ordering, and the resolved totals.
func TestDIFoldGroupsAndDedups(t *testing.T) {
	a := newDIAccum()
	// One nestjs provider injected into two controllers (a DI hub) + a repeat
	// edge for the same pair (must de-dup).
	a.entByID["UsersService"] = diEntMeta{name: "UsersService", kind: "SCOPE.Service", sourceFile: "users.service.ts", repoSlug: "api", startLine: 4}
	a.entByID["UsersController"] = diEntMeta{name: "UsersController", kind: "SCOPE.Controller", sourceFile: "users.controller.ts", repoSlug: "api", startLine: 9}
	a.entByID["AdminController"] = diEntMeta{name: "AdminController", kind: "SCOPE.Controller", sourceFile: "admin.controller.ts", repoSlug: "api", startLine: 3}

	a.addEdge(diEdge{fromID: "UsersService", toID: "UsersController", props: diProps("nestjs", "UsersService", "UsersController", "constructor", "")}, "")
	a.addEdge(diEdge{fromID: "UsersService", toID: "AdminController", props: diProps("nestjs", "UsersService", "AdminController", "constructor", "")}, "")
	a.addEdge(diEdge{fromID: "UsersService", toID: "UsersController", props: diProps("nestjs", "UsersService", "UsersController", "constructor", "")}, "") // dup
	// A second, smaller nestjs provider.
	a.addEdge(diEdge{fromID: "MailService", toID: "UsersController", props: diProps("nestjs", "MailService", "UsersController", "", "")}, "")
	// A spring provider — distinct framework group.
	a.addEdge(diEdge{fromID: "Repo", toID: "OrderService", props: diProps("spring", "Repo", "OrderService", "field", "primaryRepo")}, "")

	rep := a.assemble("g")

	if rep.TotalInjections != 4 { // dup not counted
		t.Fatalf("TotalInjections = %d, want 4", rep.TotalInjections)
	}
	if rep.TotalProviders != 3 {
		t.Fatalf("TotalProviders = %d, want 3", rep.TotalProviders)
	}
	// Distinct consumers: UsersController, AdminController, OrderService.
	if rep.TotalConsumers != 3 {
		t.Fatalf("TotalConsumers = %d, want 3", rep.TotalConsumers)
	}

	// nestjs (2 providers) must sort before spring (1) — more providers first.
	if len(rep.Groups) != 2 || rep.Groups[0].Framework != "nestjs" || rep.Groups[1].Framework != "spring" {
		t.Fatalf("group order/frameworks unexpected: %+v", rep.Groups)
	}
	if rep.Groups[0].Count != 2 {
		t.Fatalf("nestjs Count = %d, want 2", rep.Groups[0].Count)
	}

	// Within nestjs, the DI hub (UsersService, 2 consumers) is first.
	hub := rep.Groups[0].Providers[0]
	if hub.Name != "UsersService" || len(hub.Consumers) != 2 {
		t.Fatalf("nestjs hub = %s with %d consumers, want UsersService/2", hub.Name, len(hub.Consumers))
	}
	// Resolved provider carries repo-qualified ID + kind + ref.
	if hub.EntityID != "api/UsersService" || hub.Kind != "SCOPE.Service" || hub.SourceFile != "users.service.ts" {
		t.Fatalf("hub resolution wrong: %+v", hub)
	}
	// Consumers sorted by name: AdminController before UsersController.
	if hub.Consumers[0].Name != "AdminController" || hub.Consumers[1].Name != "UsersController" {
		t.Fatalf("consumer order wrong: %+v", hub.Consumers)
	}
	if hub.Consumers[0].Via != "constructor" || hub.Consumers[0].Kind != "SCOPE.Controller" {
		t.Fatalf("consumer facet wrong: %+v", hub.Consumers[0])
	}

	// Frameworks list sorted.
	if len(rep.Frameworks) != 2 || rep.Frameworks[0] != "nestjs" || rep.Frameworks[1] != "spring" {
		t.Fatalf("Frameworks = %v, want [nestjs spring]", rep.Frameworks)
	}

	// Spring consumer carries the qualifier.
	spring := rep.Groups[1].Providers[0]
	if spring.Consumers[0].Qualifier != "primaryRepo" {
		t.Fatalf("spring qualifier = %q, want primaryRepo", spring.Consumers[0].Qualifier)
	}
}

// TestDIFoldUnresolvedFallback verifies that an endpoint with no indexed entity
// falls back to the edge property name, then the raw key tail — never invented.
func TestDIFoldUnresolvedFallback(t *testing.T) {
	a := newDIAccum()
	// No entByID entries. provider prop present, consumer prop absent.
	a.addEdge(diEdge{
		fromID: "api::SCOPE.Token:DATA_SOURCE",
		toID:   "api::SCOPE.Class:OrderService",
		props:  diProps("guice", "DataSource", "", "", ""),
	}, "")
	rep := a.assemble("g")
	if rep.TotalProviders != 1 {
		t.Fatalf("TotalProviders = %d, want 1", rep.TotalProviders)
	}
	prov := rep.Groups[0].Providers[0]
	if prov.Name != "DataSource" { // from provider prop
		t.Fatalf("provider name = %q, want DataSource (from prop)", prov.Name)
	}
	if prov.EntityID != "api::SCOPE.Token:DATA_SOURCE" { // unresolved ⇒ raw id, not repo-qualified
		t.Fatalf("unresolved provider EntityID = %q, want raw id", prov.EntityID)
	}
	if prov.Repo != "" || prov.SourceFile != "" {
		t.Fatalf("unresolved provider must have no repo/ref: %+v", prov)
	}
	// consumer prop absent ⇒ raw key tail.
	if prov.Consumers[0].Name != "OrderService" {
		t.Fatalf("consumer name = %q, want OrderService (key tail)", prov.Consumers[0].Name)
	}
}

// TestDIFoldFrameworkFilter verifies the framework query filter restricts edges.
func TestDIFoldFrameworkFilter(t *testing.T) {
	a := newDIAccum()
	a.addEdge(diEdge{fromID: "A", toID: "B", props: diProps("nestjs", "A", "B", "", "")}, "spring")
	a.addEdge(diEdge{fromID: "C", toID: "D", props: diProps("spring", "C", "D", "", "")}, "spring")
	rep := a.assemble("g")
	if rep.TotalProviders != 1 || rep.Groups[0].Framework != "spring" {
		t.Fatalf("framework filter failed: %+v", rep.Groups)
	}
}

// TestDIFoldEmpty verifies a clean empty report when no DI edges exist.
func TestDIFoldEmpty(t *testing.T) {
	rep := newDIAccum().assemble("g")
	if rep.TotalProviders != 0 || rep.TotalConsumers != 0 || rep.TotalInjections != 0 {
		t.Fatalf("empty totals nonzero: %+v", rep)
	}
	if len(rep.Groups) != 0 || len(rep.Frameworks) != 0 {
		t.Fatalf("empty report has groups/frameworks: %+v", rep)
	}
}
