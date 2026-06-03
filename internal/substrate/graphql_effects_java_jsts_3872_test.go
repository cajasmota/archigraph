// Value-asserting effect probes for #3872 (epic): GraphQL-resolver EFFECT
// capabilities on java {dgs, spring-graphql} and jsts {pothos, type-graphql}.
//
// Both the java and jsts effect sniffers (effect_sinks_java.go /
// effect_sinks_jsts.go) are FRAMEWORK-BLIND: they apply syntactic sink
// regexes to ANY function body and attribute each sink to the nearest
// enclosing method/function header, with zero per-framework gating. So a
// DGS @DgsMutation datafetcher, a spring-graphql @MutationMapping method, a
// Pothos `t.field({ resolve })` and a type-graphql @Mutation method all feed
// the identical sniffer that flagship siblings (spring-boot / express) hit.
//
// These probes drive a representative resolver/datafetcher idiom for each of
// the four servers and assert the EXACT effect fires attributed to the
// resolver method — mirroring python_graphql_credit_3911_test.go.
//
// CELL SCOPE (per the registry taxonomy for these four records, the only
// effect cells declared are fs_effect / http_effect / mutation_effect — there
// is NO db_effect cell here):
//
//   - mutation_effect: GENUINELY APPLIES. The mutation sink is the
//     `this.<field> = ...` field-assignment idiom (javaMutationRe /
//     jstsMutationRe). A stateful resolver/datafetcher that records request
//     state on its receiver before persisting fires it. Probed below; flipped
//     missing→partial for all four.
//   - http_effect: only flipped for the java records (dgs, spring-graphql),
//     and only because the OUTBOUND-call probe below drives a federation /
//     datasource RestTemplate/WebClient call inside a datafetcher and asserts
//     the http_out sniffer fires attributed to the resolver. (The two jsts
//     records were already `partial`.)
//   - fs_effect: HONEST-MISSING — left untouched. No probe flips it; these
//     GraphQL resolvers do no filesystem I/O and we do not fabricate one.
//
// The db_read/db_write effects DO fire on these resolver bodies too (the
// sniffer is uniform), and the probes assert that as supporting evidence —
// but since no db_effect cell exists on these four records, no db cell is
// flipped.
package substrate

import "testing"

// ---------------------------------------------------------------------------
// Java — Netflix DGS @DgsComponent datafetcher.
//
// createOrder is a @DgsMutation datafetcher that records request state on the
// receiver (`this.lastActor = ...` → mutation sink), reads via a Spring Data
// repository (`.findById` → db_read), writes via `.save` (db_write), and makes
// an OUTBOUND federation call via RestTemplate (`restTemplate.postForObject`
// → http_out). account is a @DgsQuery datafetcher doing a repository read.
// ---------------------------------------------------------------------------
const dgsDatafetcher = `
package com.example.graph;

import com.netflix.graphql.dgs.DgsComponent;
import com.netflix.graphql.dgs.DgsQuery;
import com.netflix.graphql.dgs.DgsMutation;
import com.netflix.graphql.dgs.InputArgument;
import org.springframework.web.client.RestTemplate;

@DgsComponent
public class OrderDatafetcher {
    private final OrderRepository orderRepository;
    private final RestTemplate restTemplate;
    private String lastActor;

    @DgsQuery
    public Account account(@InputArgument String accountId) {
        Account acct = accountRepository.findById(accountId).orElseThrow();
        return acct;
    }

    @DgsMutation
    public Order createOrder(@InputArgument OrderInput input, @InputArgument String actor) {
        this.lastActor = actor;
        Order order = orderRepository.save(new Order(input));
        PaymentRef ref = restTemplate.postForObject("http://payments/charge", input, PaymentRef.class);
        return order;
    }
}
`

// ---------------------------------------------------------------------------
// Java — Spring for GraphQL @Controller with @QueryMapping / @MutationMapping.
//
// addUser is a @MutationMapping that records receiver state (mutation sink),
// reads via repository `.findById` (db_read), writes via `.save` (db_write),
// and makes an OUTBOUND call via WebClient (`webClient.post` → http_out).
// ---------------------------------------------------------------------------
const springGraphqlController = `
package com.example.graph;

import org.springframework.graphql.data.method.annotation.Argument;
import org.springframework.graphql.data.method.annotation.QueryMapping;
import org.springframework.graphql.data.method.annotation.MutationMapping;
import org.springframework.stereotype.Controller;
import org.springframework.web.reactive.function.client.WebClient;

@Controller
public class UserController {
    private final UserRepository userRepository;
    private final WebClient webClient;
    private int lastCount;

    @QueryMapping
    public User user(@Argument String id) {
        User u = userRepository.findById(id).orElseThrow();
        return u;
    }

    @MutationMapping
    public User addUser(@Argument NewUser input) {
        this.lastCount = this.lastCount + 1;
        User saved = userRepository.save(new User(input));
        webClient.post().uri("http://audit/log").retrieve();
        return saved;
    }
}
`

// ---------------------------------------------------------------------------
// jsts — Pothos t.field({ resolve }) builder idiom.
//
// Mirroring the ATTRIBUTION finding in substrate_jsts_graphql_codefirst_test.go
// (#3903): the jsts nearest-header scanner does NOT recognise the inline
// `resolve: (root, args) => …` arrow nested in a Pothos `t.field({ … })` call,
// so a sink written directly in that arrow attributes to "" (module scope).
// Real Pothos codebases therefore put the effectful work in a named
// helper/service that the inline resolver delegates to — which attributes
// cleanly. persistUser is that helper: it records receiver state
// (`this.lastEmail = ...` → mutation sink), reads (`.findUnique` → db_read) and
// writes (`.create` → db_write). The inline resolver delegates to it.
// ---------------------------------------------------------------------------
const pothosResolver = `
import { builder } from './builder';
import { prisma } from './db';

async function persistUser(args, ctx) {
  this.lastEmail = args.email;
  const existing = await prisma.user.findUnique({ where: { email: args.email } });
  const created = await prisma.user.create({ data: { email: args.email } });
  return created;
}

builder.mutationType({
  fields: (t) => ({
    createUser: t.field({
      type: 'User',
      args: { email: t.arg.string({ required: true }) },
      resolve: (root, args, ctx) => persistUser(args, ctx),
    }),
  }),
});
`

// ---------------------------------------------------------------------------
// jsts — type-graphql @Resolver class with @Query / @Mutation methods.
//
// Same attribution finding (#3903): a method whose parameter list carries an
// `@Arg(…)` decorator defeats the method-shorthand header regex (the `(` inside
// the decorator), so a sink in the method body attributes to "". The effectful
// work therefore lives in a named service the @Mutation method delegates to —
// persistAccount records receiver state (`this.lastEmail = ...` → mutation
// sink), reads (`.findOne` → db_read) and writes (`.save` → db_write).
// ---------------------------------------------------------------------------
const typeGraphqlResolver = `
import { Resolver, Query, Mutation, Arg } from 'type-graphql';
import { User } from './User';

async function persistAccount(email) {
  this.lastEmail = email;
  const existing = await this.repo.findOne({ where: { email } });
  const created = await this.repo.save({ email });
  return created;
}

@Resolver(() => User)
export class UserResolver {
  @Mutation(() => User)
  async createUser(@Arg('email') email: string): Promise<User> {
    return persistAccount(email);
  }
}
`

// effectsByFn indexes EffectMatch slices by (function, effect) for exact
// attributed-effect assertions.
func collectEffects(t *testing.T, lang, src string) []EffectMatch {
	t.Helper()
	fn := EffectSnifferFor(lang)
	if fn == nil {
		t.Fatalf("no %s effect sniffer registered", lang)
	}
	return fn(src)
}

// (hasEffectIn(ms, eff, fn) and hasEffect(ms, eff) are shared helpers from
// substrate_jsts_graphql_codefirst_test.go in this package.)

// TestDgs_MutationEffect_Fires asserts the EXACT mutation_effect
// (`this.lastActor = ...`) fires attributed to the @DgsMutation createOrder
// datafetcher. Flips lang.java.framework.dgs Substrate.mutation_effect
// missing→partial.
func TestDgs_MutationEffect_Fires(t *testing.T) {
	ms := collectEffects(t, "java", dgsDatafetcher)
	if !hasEffectIn(ms, EffectMutation, "createOrder") {
		t.Errorf("dgs: expected mutation_effect attributed to createOrder, got %v", ms)
	}
	// Supporting evidence (no db_effect cell on this record, not flipped):
	if !hasEffect(ms, EffectDBRead) || !hasEffect(ms, EffectDBWrite) {
		t.Errorf("dgs: expected db_read+db_write to fire on datafetcher bodies, got %v", ms)
	}
}

// TestDgs_HTTPEffect_Fires asserts the EXACT http_effect (RestTemplate
// postForObject outbound federation call) fires attributed to createOrder.
// Flips lang.java.framework.dgs Substrate.http_effect missing→partial.
func TestDgs_HTTPEffect_Fires(t *testing.T) {
	ms := collectEffects(t, "java", dgsDatafetcher)
	if !hasEffectIn(ms, EffectHTTPOut, "createOrder") {
		t.Errorf("dgs: expected http_out attributed to createOrder (restTemplate.postForObject), got %v", ms)
	}
}

// TestSpringGraphql_MutationEffect_Fires asserts mutation_effect
// (`this.lastCount = ...`) fires attributed to the @MutationMapping addUser
// method. Flips lang.java.framework.spring-graphql Substrate.mutation_effect
// missing→partial.
func TestSpringGraphql_MutationEffect_Fires(t *testing.T) {
	ms := collectEffects(t, "java", springGraphqlController)
	if !hasEffectIn(ms, EffectMutation, "addUser") {
		t.Errorf("spring-graphql: expected mutation_effect attributed to addUser, got %v", ms)
	}
	if !hasEffect(ms, EffectDBRead) || !hasEffect(ms, EffectDBWrite) {
		t.Errorf("spring-graphql: expected db_read+db_write on resolver bodies, got %v", ms)
	}
}

// TestSpringGraphql_HTTPEffect_Fires asserts http_effect (WebClient.post
// outbound call) fires attributed to addUser. Flips
// lang.java.framework.spring-graphql Substrate.http_effect missing→partial.
func TestSpringGraphql_HTTPEffect_Fires(t *testing.T) {
	ms := collectEffects(t, "java", springGraphqlController)
	if !hasEffectIn(ms, EffectHTTPOut, "addUser") {
		t.Errorf("spring-graphql: expected http_out attributed to addUser (webClient.post), got %v", ms)
	}
}

// TestPothos_MutationEffect_Fires asserts mutation_effect
// (`this.lastEmail = ...`) fires attributed to the named persistUser helper the
// Pothos inline resolver delegates to. Flips lang.jsts.framework.pothos
// Substrate.mutation_effect missing→partial. (http_effect on pothos was already
// partial.)
func TestPothos_MutationEffect_Fires(t *testing.T) {
	ms := collectEffects(t, "jsts", pothosResolver)
	if !hasEffectIn(ms, EffectMutation, "persistUser") {
		t.Errorf("pothos: expected mutation_effect attributed to persistUser, got %v", ms)
	}
	if !hasEffect(ms, EffectDBRead) || !hasEffect(ms, EffectDBWrite) {
		t.Errorf("pothos: expected db_read+db_write on resolve bodies, got %v", ms)
	}
}

// TestTypeGraphql_MutationEffect_Fires asserts mutation_effect
// (`this.lastEmail = ...`) fires attributed to the named persistAccount service
// the @Mutation method delegates to. Flips lang.jsts.framework.type-graphql
// Substrate.mutation_effect missing→partial. (http_effect on type-graphql was
// already partial.)
func TestTypeGraphql_MutationEffect_Fires(t *testing.T) {
	ms := collectEffects(t, "jsts", typeGraphqlResolver)
	if !hasEffectIn(ms, EffectMutation, "persistAccount") {
		t.Errorf("type-graphql: expected mutation_effect attributed to persistAccount, got %v", ms)
	}
	if !hasEffect(ms, EffectDBRead) || !hasEffect(ms, EffectDBWrite) {
		t.Errorf("type-graphql: expected db_read+db_write on resolver bodies, got %v", ms)
	}
}

// TestGraphqlResolvers_FSEffect_HonestMissing is the HONEST-NEGATIVE probe:
// none of the four representative resolver idioms perform filesystem I/O, so
// no fs_read/fs_write effect fires. fs_effect STAYS MISSING on all four
// records — we do not fabricate a file op to flip it.
func TestGraphqlResolvers_FSEffect_HonestMissing(t *testing.T) {
	cases := map[string]struct {
		lang string
		src  string
	}{
		"dgs":            {"java", dgsDatafetcher},
		"spring-graphql": {"java", springGraphqlController},
		"pothos":         {"jsts", pothosResolver},
		"type-graphql":   {"jsts", typeGraphqlResolver},
	}
	for name, c := range cases {
		ms := collectEffects(t, c.lang, c.src)
		if hasEffect(ms, EffectFSRead) || hasEffect(ms, EffectFSWrite) {
			t.Errorf("%s: expected NO fs effect (resolvers do no file I/O), got %v", name, ms)
		}
	}
}
