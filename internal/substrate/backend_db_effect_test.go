// Backend-HTTP db_effect recording sweep (#2903).
//
// Proves that the jsts effect sniffer fires db_read / db_write on a small
// hand-written handler fixture for each of the 12 backend-HTTP frameworks
// the Data column tracks. These tests are the proving artefact for the
// honest-greening rule: each test passes BEFORE the corresponding
// registry Data/db_effect cell is flipped to full.
//
// The sniffer is framework-agnostic (it matches ORM call idioms, not the
// routing DSL), so one fixture per framework is sufficient to demonstrate
// the capability is real for that framework's handler style.
package substrate

import (
	"os"
	"path/filepath"
	"testing"
)

// backendDBFixtureDir resolves the substrate_backend_db fixture directory.
// Tests run with cwd = internal/substrate/, so we walk up to the repo root
// then descend into the extractor testdata tree.
func backendDBFixtureDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "extractors", "javascript", "testdata", "substrate_backend_db"))
	if err != nil {
		t.Fatalf("cannot resolve fixture dir: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("fixture dir missing at %s: %v", dir, err)
	}
	return dir
}

func readBackendDBFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(backendDBFixtureDir(t), name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read fixture %s: %v", path, err)
	}
	return string(b)
}

// TestBackendDBEffect asserts the effect sniffer attributes a db_read and a
// db_write to the named handler in each framework fixture.
func TestBackendDBEffect(t *testing.T) {
	cases := []struct {
		framework string
		fixture   string
		readFn    string
		writeFn   string
	}{
		{"express", "express.ts", "expressGetUser", "expressCreateUser"},
		{"nestjs", "nestjs.ts", "nestFindUser", "nestSaveUser"},
		{"fastify", "fastify.ts", "fastifyGetUser", "fastifyCreateUser"},
		{"koa", "koa.ts", "koaGetUser", "koaCreateUser"},
		{"hapi", "hapi.ts", "hapiGetUser", "hapiCreateUser"},
		{"adonisjs", "adonisjs.ts", "adonisShow", "adonisStore"},
		{"feathers", "feathers.ts", "feathersFindUsers", "feathersCreateUser"},
		{"hono", "hono.ts", "honoGetUser", "honoCreateUser"},
		{"marblejs", "marblejs.ts", "marbleGetUser", "marbleCreateUser"},
		{"polka", "polka.ts", "polkaGetUser", "polkaCreateUser"},
		{"restify", "restify.ts", "restifyGetUser", "restifyCreateUser"},
		{"sails", "sails.ts", "sailsFindUsers", "sailsCreateUser"},
	}
	for _, tc := range cases {
		t.Run(tc.framework, func(t *testing.T) {
			by := groupByEffect(sniffEffectsJSTS(readBackendDBFixture(t, tc.fixture)))
			mustHave(t, by, EffectDBRead, tc.readFn)
			mustHave(t, by, EffectDBWrite, tc.writeFn)
		})
	}
}
