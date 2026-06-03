// Value-asserting fixtures for the 4 PHP GraphQL framework records (#4056,
// epic #3872, from PHP audit #3881): lang.php.framework.api-platform,
// api-platform-graphql, graphql-php, lighthouse.
//
// The PHP substrate sniffers (def_use_php.go, effect_sinks_php.go,
// taint_sites_php.go, payload_shapes_php.go, entry_points_php.go,
// template_pattern_php.go, php.go) all register on the "php" slug with NO
// framework gate — they are framework-AGNOSTIC and fire on any .php source
// dispatched via LanguageForPath. The generic PHP framework cluster
// (cakephp/laravel/symfony/…) already carries the language-level Substrate
// lane; the 4 GraphQL records were created in a later credit-wave and never
// received it. These tests prove the sniffers produce the SPECIFIC artifact
// (a named def→use, a named effect sink, a categorised taint site/sanitizer,
// an entry-rooted library_export, a literal/env-fallback/import binding, a
// template literal) on each record's real GraphQL resolver idiom — which is
// what justifies crediting the same partial/full sibling status.
//
// SCOPE NOTE (honest): request_sink_dataflow (DATA_FLOWS_TO) stays missing on
// every record — its source/sink pair is not proven to compose through a
// GraphQL resolver, matching the cakephp sibling (#3740). The graph-level
// confidence_overlay and the Tarjan module_cycle pass have no per-record
// sniffer artifact; they are credited from the language-level sibling cite
// (they consume the namespace_use IMPORTS edges proven by the import binding
// below). request/response_shape are already credited full on the three
// GraphQL records via their custom extractors; the language-level payload
// path is guarded here only where it independently fires.
package substrate

import (
	"reflect"
	"testing"
)

// hasBindingProv asserts sniffPHP produced a binding for ident with the given
// provenance.
func hasBindingProv(t *testing.T, bs []Binding, ident string, prov Provenance) {
	t.Helper()
	for _, b := range bs {
		if b.Ident == ident && b.Provenance == prov {
			return
		}
	}
	t.Errorf("expected %s binding for %q; got %+v", prov, ident, bs)
}

// hasEntryPHP asserts an entry point with the given ident+kind was produced.
func hasEntryPHP(t *testing.T, eps []EntryPoint, ident string, kind EntryKind) {
	t.Helper()
	for _, e := range eps {
		if e.Ident == ident && e.Kind == kind {
			return
		}
	}
	t.Errorf("expected entry point %s/%s; got %+v", ident, kind, eps)
}

// hasTaint asserts a taint match of the given kind+category exists.
func hasTaintKindCat(t *testing.T, ms []TaintMatch, kind TaintKind, cat TaintCategory) {
	t.Helper()
	for _, m := range ms {
		if m.Kind == kind && m.Category == cat {
			return
		}
	}
	t.Errorf("expected taint %s/%s; got %+v", kind, cat, ms)
}

// --- lighthouse: a Lighthouse (Laravel) GraphQL field resolver -------------
//
// Idiom: a `@field(resolver: …)` resolver class with a public resolve()
// method that reads $args, queries Eloquent, mutates state, calls a sanitiser,
// logs, and reads config via env fallback.

const lighthouseResolverSrc = `<?php

namespace App\GraphQL\Queries;

use App\Models\User;
use Illuminate\Support\Facades\Log;

const CACHE_TTL = "3600";
$apiKey = getenv("API_KEY") ?: "dev-key";

class UsersResolver
{
    private $limit;

    public function resolve($root, array $args)
    {
        $term = $args['search'];
        $rows = User::where('name', $term)->get();
        $this->limit = count($rows);
        Log::info("resolved users query");
        $safe = htmlspecialchars($term);
        echo $safe;
        return $rows;
    }
}
`

func TestPHPGraphQL_Lighthouse_DefUse(t *testing.T) {
	defs, uses := sniffDefUsePHP(lighthouseResolverSrc)
	hasDefUse(t, defs, uses, "resolve", "term")
}

func TestPHPGraphQL_Lighthouse_Entry(t *testing.T) {
	hasEntryPHP(t, sniffPHPEntryPoints(lighthouseResolverSrc), "resolve", EntryKindLibraryExport)
}

func TestPHPGraphQL_Lighthouse_Effects(t *testing.T) {
	by := groupByEffect(sniffEffectsPHP(lighthouseResolverSrc))
	mustHave(t, by, EffectDBRead, "resolve")   // User::where(...)->get()
	mustHave(t, by, EffectMutation, "resolve") // $this->limit = ...
}

func TestPHPGraphQL_Lighthouse_Taint(t *testing.T) {
	ms := sniffTaintPHP(lighthouseResolverSrc)
	hasTaintKindCat(t, ms, TaintKindSanitizer, TaintCategoryXSS) // htmlspecialchars
	hasTaintKindCat(t, ms, TaintKindSink, TaintCategoryXSS)      // echo $safe
}

func TestPHPGraphQL_Lighthouse_Template(t *testing.T) {
	ps := sniffTemplatePatternsPHP(lighthouseResolverSrc)
	if !hasTemplateKind(ps, TemplateKindLog) { // Log::info("...")
		t.Errorf("expected log template pattern; got %+v", ps)
	}
}

func TestPHPGraphQL_Lighthouse_Bindings(t *testing.T) {
	bs := sniffPHP(lighthouseResolverSrc)
	hasBindingProv(t, bs, "CACHE_TTL", ProvenanceLiteral)  // constant_propagation
	hasBindingProv(t, bs, "apiKey", ProvenanceEnvFallback) // env_fallback_recognition
	hasBindingProv(t, bs, "User", ProvenanceCrossFile)     // import_resolution_quality
}

// --- graphql-php: a webonyx graphql-php resolver with raw PDO --------------
//
// Idiom: a type-config resolver method using a concatenated SQL sink + a
// prepared-statement sanitiser + an outbound HTTP fetch.

const graphqlPhpResolverSrc = `<?php

namespace App\GraphQL\Type;

use GraphQL\Type\Definition\ObjectType;

class QueryType
{
    public function user($root, $args, $context)
    {
        $id = $args['id'];
        $sql = "SELECT * FROM users WHERE id = " . $id;
        $stmt = $context->pdo->query($sql);
        $check = $context->pdo->prepare("SELECT 1 FROM users WHERE id = ?");
        $resp = file_get_contents("https://api.example.com/enrich");
        return $stmt;
    }
}
`

func TestPHPGraphQL_GraphqlPhp_DefUse(t *testing.T) {
	defs, uses := sniffDefUsePHP(graphqlPhpResolverSrc)
	hasDefUse(t, defs, uses, "user", "sql")
}

func TestPHPGraphQL_GraphqlPhp_Entry(t *testing.T) {
	hasEntryPHP(t, sniffPHPEntryPoints(graphqlPhpResolverSrc), "user", EntryKindLibraryExport)
}

func TestPHPGraphQL_GraphqlPhp_Effects(t *testing.T) {
	by := groupByEffect(sniffEffectsPHP(graphqlPhpResolverSrc))
	mustHave(t, by, EffectHTTPOut, "user") // file_get_contents("https://...")
}

func TestPHPGraphQL_GraphqlPhp_Taint(t *testing.T) {
	ms := sniffTaintPHP(graphqlPhpResolverSrc)
	hasTaintKindCat(t, ms, TaintKindSink, TaintCategorySQL)      // ->query("..." . $id)
	hasTaintKindCat(t, ms, TaintKindSanitizer, TaintCategorySQL) // ->prepare("... ?")
}

func TestPHPGraphQL_GraphqlPhp_Template(t *testing.T) {
	ps := sniffTemplatePatternsPHP(graphqlPhpResolverSrc)
	if !hasTemplateKind(ps, TemplateKindSQL) { // "SELECT * FROM users ..."
		t.Errorf("expected sql template pattern; got %+v", ps)
	}
}

func TestPHPGraphQL_GraphqlPhp_Bindings(t *testing.T) {
	bs := sniffPHP(graphqlPhpResolverSrc)
	hasBindingProv(t, bs, "ObjectType", ProvenanceCrossFile) // import_resolution_quality
}

// --- api-platform-graphql: a Symfony API Platform GraphQL resolver ----------
//
// Idiom: a QueryItemResolverInterface __invoke() that reads request input,
// persists via Doctrine, and reads $_GET as a taint source.

const apiPlatformGraphqlResolverSrc = `<?php

namespace App\Resolver;

use ApiPlatform\GraphQl\Resolver\QueryItemResolverInterface;
use Doctrine\ORM\EntityManagerInterface;

class BookResolver implements QueryItemResolverInterface
{
    private $em;

    public function __invoke($item, array $context)
    {
        $filter = $_GET['filter'];
        $book = $this->em->find('Book', $context['id']);
        $this->em->persist($book);
        return $book;
    }
}
`

func TestPHPGraphQL_ApiPlatformGraphql_DefUse(t *testing.T) {
	defs, uses := sniffDefUsePHP(apiPlatformGraphqlResolverSrc)
	hasDefUse(t, defs, uses, "__invoke", "book")
}

func TestPHPGraphQL_ApiPlatformGraphql_Entry(t *testing.T) {
	hasEntryPHP(t, sniffPHPEntryPoints(apiPlatformGraphqlResolverSrc), "__invoke", EntryKindLibraryExport)
}

func TestPHPGraphQL_ApiPlatformGraphql_Effects(t *testing.T) {
	by := groupByEffect(sniffEffectsPHP(apiPlatformGraphqlResolverSrc))
	mustHave(t, by, EffectDBRead, "__invoke")  // $this->em->find(...)
	mustHave(t, by, EffectDBWrite, "__invoke") // $this->em->persist(...)
}

func TestPHPGraphQL_ApiPlatformGraphql_Taint(t *testing.T) {
	ms := sniffTaintPHP(apiPlatformGraphqlResolverSrc)
	hasTaintKindCat(t, ms, TaintKindSource, TaintCategoryGeneric) // $_GET['filter']
}

func TestPHPGraphQL_ApiPlatformGraphql_Bindings(t *testing.T) {
	bs := sniffPHP(apiPlatformGraphqlResolverSrc)
	hasBindingProv(t, bs, "QueryItemResolverInterface", ProvenanceCrossFile)
}

// --- api-platform: a Symfony API Platform state provider / controller -------
//
// Idiom: a state provider reading request input via $request->get(), querying
// Doctrine, and returning a JsonResponse — the language-level payload path
// fires here, which is what api-platform needs for request/response_shape.

const apiPlatformProviderSrc = `<?php

namespace App\State;

use Doctrine\ORM\EntityManagerInterface;
use Symfony\Component\HttpFoundation\JsonResponse;

class BookProvider
{
    private $em;

    public function provide($request)
    {
        $title = $request->get('title');
        $author = $request->get('author');
        $books = $this->em->getRepository('Book')->findBy(['title' => $title]);
        return new JsonResponse(['id' => 1, 'title' => $title, 'count' => 2]);
    }
}
`

func TestPHPGraphQL_ApiPlatform_DefUse(t *testing.T) {
	defs, uses := sniffDefUsePHP(apiPlatformProviderSrc)
	hasDefUse(t, defs, uses, "provide", "title")
}

func TestPHPGraphQL_ApiPlatform_Entry(t *testing.T) {
	hasEntryPHP(t, sniffPHPEntryPoints(apiPlatformProviderSrc), "provide", EntryKindLibraryExport)
}

func TestPHPGraphQL_ApiPlatform_Effects(t *testing.T) {
	by := groupByEffect(sniffEffectsPHP(apiPlatformProviderSrc))
	mustHave(t, by, EffectDBRead, "provide") // ->getRepository(...)/->findBy(...)
}

func TestPHPGraphQL_ApiPlatform_Taint(t *testing.T) {
	ms := sniffTaintPHP(apiPlatformProviderSrc)
	hasTaintKindCat(t, ms, TaintKindSource, TaintCategoryGeneric) // $request->get(...)
}

func TestPHPGraphQL_ApiPlatform_RequestShape(t *testing.T) {
	shapes := sniffPayloadShapesPHP(apiPlatformProviderSrc)
	req := findShape(shapes, "provide", PayloadDirectionRequest, PayloadSideProducer)
	if req == nil {
		t.Fatalf("expected api-platform request shape; got %+v", shapes)
	}
	if got := sortedNames(req.Fields); !reflect.DeepEqual(got, []string{"author", "title"}) {
		t.Errorf("api-platform request fields: want [author title] got %v", got)
	}
}

func TestPHPGraphQL_ApiPlatform_ResponseShape(t *testing.T) {
	shapes := sniffPayloadShapesPHP(apiPlatformProviderSrc)
	resp := findShape(shapes, "provide", PayloadDirectionResponse, PayloadSideProducer)
	if resp == nil {
		t.Fatalf("expected api-platform JsonResponse response shape; got %+v", shapes)
	}
	if got := sortedNames(resp.Fields); !reflect.DeepEqual(got, []string{"count", "id", "title"}) {
		t.Errorf("api-platform response fields: want [count id title] got %v", got)
	}
}

func TestPHPGraphQL_ApiPlatform_Bindings(t *testing.T) {
	bs := sniffPHP(apiPlatformProviderSrc)
	hasBindingProv(t, bs, "JsonResponse", ProvenanceCrossFile)
}
