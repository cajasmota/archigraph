package python

import (
	"context"
	"testing"

	"github.com/cajasmota/archigraph/internal/extractor"
	"github.com/cajasmota/archigraph/internal/types"
)

// diEdges returns every DI edge (with FromID/ToID/Kind) emitted by the
// python_di_graph extractor for the given source.
func diEdges(t *testing.T, src string) []types.RelationshipRecord {
	t.Helper()
	ext, ok := extractor.Get("python_di_graph")
	if !ok {
		t.Fatal("python_di_graph not registered")
	}
	ents, err := ext.Extract(context.Background(), extractor.FileInput{
		Path: "test.py", Content: []byte(src), Language: "python",
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	var rels []types.RelationshipRecord
	for _, e := range ents {
		rels = append(rels, e.Relationships...)
	}
	return rels
}

func hasEdge(rels []types.RelationshipRecord, from, to, kind string) bool {
	for _, r := range rels {
		if r.FromID == from && r.ToID == to && r.Kind == kind {
			return true
		}
	}
	return false
}

// FastAPI: `svc: Service = Depends(get_service)` in handler → INJECTED_INTO
// with resolved provider=get_service, consumer=handler.
func TestPyDI_FastAPIDependsCallable(t *testing.T) {
	src := `from fastapi import Depends

def get_service():
    return Service()

@app.get("/x")
def handler(svc: Service = Depends(get_service)):
    return svc
`
	rels := diEdges(t, src)
	if !hasEdge(rels, "get_service", "handler", string(types.RelationshipKindInjectedInto)) {
		t.Fatalf("expected INJECTED_INTO(get_service -> handler); got %+v", rels)
	}
}

// FastAPI: `Depends(SvcClass)` → provider is the class.
func TestPyDI_FastAPIDependsClass(t *testing.T) {
	src := `def handler(svc=Depends(SvcClass)):
    return svc
`
	rels := diEdges(t, src)
	if !hasEdge(rels, "SvcClass", "handler", string(types.RelationshipKindInjectedInto)) {
		t.Fatalf("expected INJECTED_INTO(SvcClass -> handler); got %+v", rels)
	}
}

// FastAPI bare Depends() resolves from the type annotation.
func TestPyDI_FastAPIDependsBareType(t *testing.T) {
	src := `def handler(svc: AuthService = Depends()):
    return svc
`
	rels := diEdges(t, src)
	if !hasEdge(rels, "AuthService", "handler", string(types.RelationshipKindInjectedInto)) {
		t.Fatalf("expected INJECTED_INTO(AuthService -> handler); got %+v", rels)
	}
}

// Negative: dynamic Depends(getattr(...)) yields no edge.
func TestPyDI_FastAPIDynamicNoEdge(t *testing.T) {
	src := `def handler(svc=Depends(getattr(mod, name))):
    return svc
`
	rels := diEdges(t, src)
	for _, r := range rels {
		if r.Kind == string(types.RelationshipKindInjectedInto) {
			t.Fatalf("expected no INJECTED_INTO for dynamic Depends; got %+v", r)
		}
	}
}

// dependency-injector: container provider Factory(Service) → BINDS token→impl.
func TestPyDI_ContainerProviderBinds(t *testing.T) {
	src := `from dependency_injector import containers, providers

class Container(containers.DeclarativeContainer):
    service = providers.Factory(Service)
    repo = providers.Singleton(Repository, db=service)
`
	rels := diEdges(t, src)
	if !hasEdge(rels, "service", "Service", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(service -> Service); got %+v", rels)
	}
	if !hasEdge(rels, "repo", "Repository", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(repo -> Repository); got %+v", rels)
	}
}

// dependency-injector: @inject + Provide[Container.service] → INJECTED_INTO.
func TestPyDI_InjectProvideInjectedInto(t *testing.T) {
	src := `from dependency_injector.wiring import inject, Provide

@inject
def main(svc: Service = Provide[Container.service]):
    return svc
`
	rels := diEdges(t, src)
	if !hasEdge(rels, "service", "main", string(types.RelationshipKindInjectedInto)) {
		t.Fatalf("expected INJECTED_INTO(service -> main); got %+v", rels)
	}
}

// Negative: a Provide[...] without @inject is not attributed (no edge).
func TestPyDI_ProvideWithoutInjectNoEdge(t *testing.T) {
	src := `def main(svc: Service = Provide[Container.service]):
    return svc
`
	rels := diEdges(t, src)
	for _, r := range rels {
		if r.FromID == "service" && r.ToID == "main" {
			t.Fatalf("expected no edge without @inject; got %+v", r)
		}
	}
}

// Negative: a Configuration() provider with no class arg yields no BINDS.
func TestPyDI_ConfigurationProviderNoBinds(t *testing.T) {
	src := `class Container(containers.DeclarativeContainer):
    config = providers.Configuration()
`
	rels := diEdges(t, src)
	for _, r := range rels {
		if r.Kind == string(types.RelationshipKindBinds) {
			t.Fatalf("expected no BINDS for Configuration(); got %+v", r)
		}
	}
}
