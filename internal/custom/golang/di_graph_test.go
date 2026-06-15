package golang

import (
	"context"
	"testing"

	"github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"
)

func goDIEdges(t *testing.T, src string) []types.RelationshipRecord {
	t.Helper()
	ext := &goDIExtractor{}
	ents, err := ext.Extract(context.Background(), extractor.FileInput{
		Path: "wire.go", Content: []byte(src), Language: "go",
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

func goHasEdge(rels []types.RelationshipRecord, from, to, kind string) bool {
	for _, r := range rels {
		if r.FromID == from && r.ToID == to && r.Kind == kind {
			return true
		}
	}
	return false
}

// google/wire: wire.Build(NewService, NewRepo) with resolvable providers →
// BINDS(NewService → Service) and INJECTED_INTO(Repo → Service).
func TestGoDI_WireBuild(t *testing.T) {
	src := `package main

func NewRepo() *Repo { return &Repo{} }

func NewService(repo *Repo) *Service { return &Service{repo} }

func InitApp() *Service {
    wire.Build(NewService, NewRepo)
    return nil
}`
	rels := goDIEdges(t, src)
	if !goHasEdge(rels, "NewService", "Service", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(NewService -> Service); got %+v", rels)
	}
	if !goHasEdge(rels, "NewRepo", "Repo", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(NewRepo -> Repo); got %+v", rels)
	}
	if !goHasEdge(rels, "Repo", "Service", string(types.RelationshipKindInjectedInto)) {
		t.Fatalf("expected INJECTED_INTO(Repo -> Service); got %+v", rels)
	}
}

// google/wire: wire.NewSet provider set → BINDS.
func TestGoDI_WireNewSet(t *testing.T) {
	src := `package main

func NewMailer() (*Mailer, error) { return &Mailer{}, nil }

var ProviderSet = wire.NewSet(NewMailer)`
	rels := goDIEdges(t, src)
	if !goHasEdge(rels, "NewMailer", "Mailer", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(NewMailer -> Mailer); got %+v", rels)
	}
}

// uber/fx: fx.Provide(NewService) + a consumer constructor param →
// INJECTED_INTO and BINDS.
func TestGoDI_FxProvide(t *testing.T) {
	src := `package main

func NewConfig() *Config { return &Config{} }

func NewService(cfg *Config) *Service { return &Service{cfg} }

func main() {
    fx.New(
        fx.Provide(NewConfig, NewService),
    ).Run()
}`
	rels := goDIEdges(t, src)
	if !goHasEdge(rels, "NewService", "Service", string(types.RelationshipKindBinds)) {
		t.Fatalf("expected BINDS(NewService -> Service); got %+v", rels)
	}
	if !goHasEdge(rels, "Config", "Service", string(types.RelationshipKindInjectedInto)) {
		t.Fatalf("expected INJECTED_INTO(Config -> Service); got %+v", rels)
	}
}

// Negative: a provider defined in another file (not in this source) yields no
// type edge — honest-partial cross-file resolution.
func TestGoDI_UnresolvedProviderNoEdge(t *testing.T) {
	src := `package main

func InitApp() {
    wire.Build(external.NewThing)
}`
	rels := goDIEdges(t, src)
	if len(rels) != 0 {
		t.Fatalf("expected no edges for unresolved provider; got %+v", rels)
	}
}

// Negative: a bare NewX func not registered in wire/fx is not treated as DI.
func TestGoDI_UnregisteredFuncNoEdge(t *testing.T) {
	src := `package main

func NewService(repo *Repo) *Service { return &Service{repo} }`
	rels := goDIEdges(t, src)
	if len(rels) != 0 {
		t.Fatalf("expected no edges for unregistered provider; got %+v", rels)
	}
}

// Provider returning only error contributes no BINDS (no produced type).
func TestGoDI_ErrorOnlyReturnNoBinds(t *testing.T) {
	src := `package main

func Validate(c *Config) error { return nil }

func InitApp() { wire.Build(Validate) }`
	rels := goDIEdges(t, src)
	for _, r := range rels {
		if r.Kind == string(types.RelationshipKindBinds) {
			t.Fatalf("expected no BINDS for error-only provider; got %+v", r)
		}
	}
}
