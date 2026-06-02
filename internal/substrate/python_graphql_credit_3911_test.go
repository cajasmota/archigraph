// Value-asserting probes for #3911: graphene + ariadne are processed by the
// per-LANGUAGE python substrate sniffers (def_use, effects, taint, dataflow),
// which register by "python" with zero framework gating. These probes prove
// each capability genuinely fires on graphene/ariadne resolver source, so the
// substrate cells can be credited honestly (mirroring strawberry).
//
// The negative probe documents the honest gap: GraphQL resolvers read
// info/args, NOT request.data/json — so the request-input→sink dataflow does
// NOT fire (request_sink_dataflow stays missing), mirroring the jsts Pothos
// finding #3903.
package substrate

import "testing"

// grapheneSchema is a representative code-first graphene schema/resolver file.
// Resolvers read args (NOT request.*), assign locals, hit the Django ORM, and
// build a raw SQL cursor.execute — the same Python primitives strawberry hits.
const grapheneSchema = `
import graphene
from .models import Account, Order

class AccountType(graphene.ObjectType):
    id = graphene.ID()
    name = graphene.String()

class Query(graphene.ObjectType):
    account = graphene.Field(AccountType, account_id=graphene.ID(required=True))
    orders = graphene.List(lambda: OrderType)

    def resolve_account(self, info, account_id):
        acct = Account.objects.get(pk=account_id)
        return acct

    def resolve_orders(self, info):
        cursor = info.context.db.cursor()
        cursor.execute("SELECT * FROM orders WHERE owner = %s" % info.context.user_id)
        return cursor.fetchall()

class CreateOrder(graphene.Mutation):
    class Arguments:
        amount = graphene.Int(required=True)

    def mutate(self, info, amount):
        order = Order.objects.create(amount=amount)
        order.save()
        return order
`

// ariadneResolver is a representative ariadne schema-first resolver module.
// Resolvers are bound via @query.field / @mutation.field and read (obj, info,
// **kwargs) — again NOT request.*.
const ariadneResolver = `
import os
from ariadne import QueryType, MutationType
from .models import Account, Order

query = QueryType()
mutation = MutationType()

@query.field("account")
def resolve_account(obj, info, account_id):
    acct = Account.objects.get(pk=account_id)
    return acct

@mutation.field("createOrder")
def resolve_create_order(obj, info, amount):
    region = os.environ.get("ORDER_REGION")
    cursor = info.context["db"].cursor()
    cursor.execute("INSERT INTO orders (amount) VALUES (%s)" % amount)
    order = Order.objects.create(amount=amount, region=region)
    return order
`

// TestGrapheneAriadne_DefUseFires proves def_use_chain_extraction (partial).
func TestGrapheneAriadne_DefUseFires(t *testing.T) {
	fn := DefUseSnifferFor("python")
	if fn == nil {
		t.Fatal("no python def-use sniffer registered")
	}
	for name, src := range map[string]string{"graphene": grapheneSchema, "ariadne": ariadneResolver} {
		defs, uses := fn(src)
		if len(defs) == 0 {
			t.Errorf("%s: expected def-use defs, got 0", name)
		}
		if len(uses) == 0 {
			t.Errorf("%s: expected def-use uses, got 0", name)
		}
		// Resolver-local binding: `acct`/`order`/`cur` are defined inside a
		// resolver function (function-attributed, not module scope).
		var sawResolverLocal bool
		for _, d := range defs {
			if d.Function != "" && (d.Var == "acct" || d.Var == "order" || d.Var == "cursor") {
				sawResolverLocal = true
			}
		}
		if !sawResolverLocal {
			t.Errorf("%s: expected a resolver-local def (acct/order/cur) with non-empty Function; defs=%v", name, defs)
		}
	}
}

// TestGrapheneAriadne_EffectsFire proves db_effect (db_read + db_write) fires.
func TestGrapheneAriadne_EffectsFire(t *testing.T) {
	fn := EffectSnifferFor("python")
	if fn == nil {
		t.Fatal("no python effect sniffer registered")
	}
	for name, src := range map[string]string{"graphene": grapheneSchema, "ariadne": ariadneResolver} {
		matches := fn(src)
		var sawRead, sawWrite bool
		for _, m := range matches {
			switch m.Effect {
			case EffectDBRead:
				sawRead = true
			case EffectDBWrite:
				sawWrite = true
			}
		}
		if !sawRead {
			t.Errorf("%s: expected a db_read effect (Model.objects.get / cursor.execute), got %v", name, matches)
		}
		if !sawWrite {
			t.Errorf("%s: expected a db_write effect (Model.objects.create / .save), got %v", name, matches)
		}
	}
}

// TestGrapheneAriadne_TaintFires proves taint_sink_detection: the raw
// string-formatted cursor.execute is a non-literal SQL sink.
func TestGrapheneAriadne_TaintSinkFires(t *testing.T) {
	fn := TaintSnifferFor("python")
	if fn == nil {
		t.Fatal("no python taint sniffer registered")
	}
	for name, src := range map[string]string{"graphene": grapheneSchema, "ariadne": ariadneResolver} {
		matches := fn(src)
		var sawSink bool
		for _, m := range matches {
			if m.Kind == TaintKindSink {
				sawSink = true
			}
		}
		if !sawSink {
			t.Errorf("%s: expected a taint SINK (non-literal cursor.execute), got %v", name, matches)
		}
	}
}

// TestGraphene_TaintSourceEnvFires proves taint_source_detection via the
// os.environ.get read in the ariadne resolver (env source).
func TestAriadne_TaintSourceEnvFires(t *testing.T) {
	fn := TaintSnifferFor("python")
	if fn == nil {
		t.Fatal("no python taint sniffer registered")
	}
	matches := fn(ariadneResolver)
	var sawSource bool
	for _, m := range matches {
		if m.Kind == TaintKindSource {
			sawSource = true
		}
	}
	if !sawSource {
		t.Errorf("ariadne: expected a taint SOURCE (os.environ.get), got %v", matches)
	}
}

// TestGrapheneAriadne_RequestSinkDataflow_DoesNotFire is the HONEST NEGATIVE
// probe (#3911, mirroring jsts Pothos #3903). GraphQL resolvers read info/args,
// NOT request.data/json/GET/POST — so the request-input→sink dataflow sniffer
// finds no source and produces no request_sink flow. This cell STAYS MISSING.
func TestGrapheneAriadne_RequestSinkDataflow_DoesNotFire(t *testing.T) {
	ex := DataFlowSnifferExFor("python")
	if ex == nil {
		t.Fatal("no python dataflow sniffer registered")
	}
	for name, src := range map[string]string{"graphene": grapheneSchema, "ariadne": ariadneResolver} {
		res := ex(src)
		// No request.* source means no request-rooted flow. (The cursor.execute
		// sink exists, but with no request source there is no source→sink flow.)
		if len(res.Flows) != 0 {
			t.Errorf("%s: expected NO request-input dataflow (resolvers read info/args, not request.*), got %d flows: %v",
				name, len(res.Flows), res.Flows)
		}
	}
}
