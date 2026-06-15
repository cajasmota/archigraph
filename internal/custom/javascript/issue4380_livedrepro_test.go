package javascript_test

import (
	"testing"

	"github.com/cajasmota/grafel/internal/resolve"
	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/javascript"
)

// Issue #4380 LIVE-REPRO (Express/Koa side).
//
// Generalizes the NestJS global-DI fix (#4329) to Express/Koa global middleware.
// A faithful Express app registers cross-cutting middleware app-wide via
// app.use(...):
//
//	app.use(helmet())              — factory call → binds factory `helmet`
//	app.use(cookieParser())        — factory call → binds factory `cookieParser`
//	app.use(express.json())        — factory call → binds leaf `json`
//	app.use(authMiddleware)        — bare symbol  → binds the middleware fn
//	app.use('/api', apiRouter)     — mount        → binds router `apiRouter`
//	app.use(errorHandler)          — bare symbol  → binds the error middleware
//
// PRE-FIX: each app.use(...) produced only a standalone "middleware"/"mount"
// entity with NO edge from the app to the referenced middleware/router symbol,
// so authMiddleware / errorHandler / apiRouter looked orphan and the app-wide
// pipeline was invisible.
//
// POST-FIX: a synthetic `app` entity owns an app → middleware USES edge per
// registration (global=true, di_role=middleware|router, 0-based order); the
// targets resolve to the real symbol through resolve.BuildIndex.

const issue4380ExpressApp = `
import express from 'express';
import helmet from 'helmet';
import cookieParser from 'cookie-parser';
import { authMiddleware } from './auth';
import { errorHandler } from './errors';

const app = express();
const apiRouter = express.Router();

app.use(helmet());
app.use(cookieParser());
app.use(express.json());
app.use(authMiddleware);
app.use('/api', apiRouter);
app.use(errorHandler);
`

func TestIssue4380_ExpressAppUse_GlobalMiddleware(t *testing.T) {
	ents := extractFull(t, "custom_js_express", fi("app.ts", "typescript", issue4380ExpressApp))

	type want struct {
		sym   string
		role  string
		order string
	}
	wants := []want{
		{"helmet", "middleware", "0"},
		{"cookieParser", "middleware", "1"},
		{"json", "middleware", "2"},
		{"authMiddleware", "middleware", "3"},
		{"apiRouter", "router", "4"},
		{"errorHandler", "middleware", "5"},
	}
	for _, w := range wants {
		if !hasEdge(ents, "app", "USES", "app", w.sym) {
			t.Errorf("expected app USES %s (app.use)", w.sym)
			continue
		}
		if v := edgeProp(ents, "USES", "app", w.sym, "global"); v != "true" {
			t.Errorf("%s: expected global=true, got %q", w.sym, v)
		}
		if v := edgeProp(ents, "USES", "app", w.sym, "di_role"); v != w.role {
			t.Errorf("%s: expected di_role=%s, got %q", w.sym, w.role, v)
		}
		if v := edgeProp(ents, "USES", "app", w.sym, "order"); v != w.order {
			t.Errorf("%s: expected order=%s, got %q", w.sym, w.order, v)
		}
	}

	// The previously-orphan middleware function must RESOLVE against a real
	// production entity through the real resolver.
	prod := types.EntityRecord{
		Name: "authMiddleware", Kind: "SCOPE.Operation", Subtype: "function",
		SourceFile: "src/auth.ts", Language: "typescript",
		Properties: map[string]string{"kind": "SCOPE.Operation", "subtype": "function"},
	}
	prod.ID = prod.ComputeID()
	idx := resolve.BuildIndex(append(ents, prod))
	if id, ok := idx.Lookup("authMiddleware"); !ok || id != prod.ID {
		t.Fatalf("global middleware authMiddleware failed to resolve (ok=%v id=%s)", ok, id)
	}

	// The path-mounted router must resolve to the router entity the express
	// extractor itself emits for `express.Router()`.
	if id, ok := idx.Lookup("apiRouter"); !ok || id == "" {
		t.Fatalf("mounted router apiRouter failed to resolve (ok=%v id=%s)", ok, id)
	}
}

// TestIssue4380_ExpressInlineMiddleware_NoFalseEdge ensures inline anonymous
// middleware (no named symbol) does not fabricate a USES edge.
func TestIssue4380_ExpressInlineMiddleware_NoFalseEdge(t *testing.T) {
	src := `
const app = express();
app.use((req, res, next) => { next(); });
app.use(function (req, res, next) { next(); });
`
	ents := extractFull(t, "custom_js_express", fi("app.ts", "typescript", src))
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == string(types.RelationshipKindUses) && r.FromID == "app" {
				t.Errorf("inline middleware should not produce an app USES edge, got ToID=%s", r.ToID)
			}
		}
	}
}
