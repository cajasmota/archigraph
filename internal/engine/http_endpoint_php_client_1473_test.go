package engine

// Tests for issue #1473 — PHP Laravel Http:: variable-interpolated URL resolution.
//
// Context: #1470 added Laravel Http:: client extraction, but when the URL is
// constructed via double-quoted string interpolation or concatenation with a
// variable assigned from config()/env(), the extractor emitted nothing because
// the variable's value was not a literal string in the symbol table.
//
// Patterns fixed:
//   - Interpolation:  Http::get("{$ordersUrl}/orders/{$orderId}")
//     where $ordersUrl = config('services.orders.url')
//   - Concatenation:  Http::post($notifUrl . '/notifications', $data)
//     where $notifUrl  = config('services.notifications.url')
//   - Guzzle variant: $client->get("{$erpUrl}/api/erp/invoices/{$id}")
//   - Chained:        Http::withToken(config('x'))->post("{$erpUrl}/api/erp/invoices")
//
// Fixture mirrors services/billing — InvoiceController.php + GenerateInvoicePdf.php
//
// Before fix (#1473 open): 0 billing→{orders,notifications,legacy-erp} edges for
// variable-interpolated/concat URL forms.
//
// After fix: all three services resolve correctly.

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// InvoiceController — interpolated URLs
// ---------------------------------------------------------------------------

// TestPHP1473_InterpolatedURL verifies that Http:: calls with
// double-quoted interpolated URLs where the base var is assigned via config()
// produce the correct http_endpoint_call entities and FETCHES edges.
func TestPHP1473_InterpolatedURL(t *testing.T) {
	src := `<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Illuminate\Support\Facades\Http;

class InvoiceController extends Controller
{
    public function store(Request $request)
    {
        $ordersUrl = config('services.orders.url');
        $notifUrl  = config('services.notifications.url');
        $erpUrl    = config('services.erp.url');

        // interpolated base + static path
        $orderResponse = Http::post("{$ordersUrl}/api/orders", [
            'customer_id' => $request->customer_id,
        ]);
        $orderId = $orderResponse->json('id');

        // interpolated base + static path (notifications)
        Http::post("{$notifUrl}/api/notifications", [
            'user_id' => $request->customer_id,
            'event'   => 'invoice.created',
        ]);

        // chained withToken + interpolated URL (legacy-erp)
        Http::withToken(config('services.erp.token'))
            ->post("{$erpUrl}/api/erp/invoices", [
                'order_id' => $orderId,
            ]);

        return response()->json(['order_id' => $orderId]);
    }

    public function show(Request $request, string $id)
    {
        $ordersUrl = config('services.orders.url');
        // interpolated base + path + param
        $response = Http::get("{$ordersUrl}/api/orders/{$id}");
        return response()->json($response->json());
    }
}
`
	ids, rels := runDetectWithRels(t, "php", "InvoiceController.php", src)

	// billing → orders (POST via interpolated URL)
	requireContains(t, ids, []string{"http:POST:/api/orders"}, "#1473 billing→orders POST interpolated")
	requireFetches(t, rels, "http:POST:/api/orders", "#1473 billing→orders POST interpolated")

	// billing → notifications (POST via interpolated URL)
	requireContains(t, ids, []string{"http:POST:/api/notifications"}, "#1473 billing→notifications POST interpolated")
	requireFetches(t, rels, "http:POST:/api/notifications", "#1473 billing→notifications POST interpolated")

	// billing → legacy-erp (POST via withToken chained + interpolated URL)
	requireContains(t, ids, []string{"http:POST:/api/erp/invoices"}, "#1473 billing→legacy-erp POST interpolated chained")
	requireFetches(t, rels, "http:POST:/api/erp/invoices", "#1473 billing→legacy-erp POST interpolated chained")

	// billing → orders (GET with path param via interpolated URL — canonicalized with {id})
	requireContains(t, ids, []string{"http:GET:/api/orders/{id}"}, "#1473 billing→orders GET interpolated with param")
}

// ---------------------------------------------------------------------------
// InvoiceController — concatenation URLs
// ---------------------------------------------------------------------------

// TestPHP1473_ConcatenationURL verifies that Http:: calls where the URL is
// assembled via PHP dot-concatenation (`$var . "/path"`) are resolved when the
// leading variable is a config()/env() assignment.
func TestPHP1473_ConcatenationURL(t *testing.T) {
	src := `<?php

namespace App\Http\Controllers;

use Illuminate\Support\Facades\Http;

class InvoiceController extends Controller
{
    public function store(Request $request)
    {
        $ordersUrl = config('services.orders.url');
        $notifUrl  = config('services.notifications.url');
        $erpUrl    = config('services.erp.url');

        // concatenation form — base var . static path
        Http::post($ordersUrl . '/api/orders', ['items' => []]);

        Http::post($notifUrl . '/api/notifications', [
            'event' => 'invoice.created',
        ]);

        Http::withToken(config('services.erp.token'))
            ->post($erpUrl . '/api/erp/invoices', ['sync' => true]);

        return response()->json([]);
    }

    public function show(string $id)
    {
        $ordersUrl = config('services.orders.url');
        // concat with additional param segment
        Http::get($ordersUrl . '/api/orders/' . $id);
    }
}
`
	ids, rels := runDetectWithRels(t, "php", "InvoiceController.php", src)

	requireContains(t, ids, []string{"http:POST:/api/orders"}, "#1473 billing→orders POST concat")
	requireFetches(t, rels, "http:POST:/api/orders", "#1473 billing→orders POST concat")

	requireContains(t, ids, []string{"http:POST:/api/notifications"}, "#1473 billing→notifications POST concat")
	requireFetches(t, rels, "http:POST:/api/notifications", "#1473 billing→notifications POST concat")

	requireContains(t, ids, []string{"http:POST:/api/erp/invoices"}, "#1473 billing→legacy-erp POST concat chained")
	requireFetches(t, rels, "http:POST:/api/erp/invoices", "#1473 billing→legacy-erp POST concat chained")

	// GET with path param — canonicalized as {id}
	requireContains(t, ids, []string{"http:GET:/api/orders/{id}"}, "#1473 billing→orders GET concat with param")
}

// ---------------------------------------------------------------------------
// GenerateInvoicePdf — interpolated + concatenation mix
// ---------------------------------------------------------------------------

// TestPHP1473_GenerateInvoicePdf verifies the GenerateInvoicePdf job
// class fixture which mixes interpolated and concatenation URL forms.
func TestPHP1473_GenerateInvoicePdf(t *testing.T) {
	src := `<?php

namespace App\Jobs;

use Illuminate\Support\Facades\Http;

class GenerateInvoicePdf
{
    public function __construct(
        private readonly int    $invoiceId,
        private readonly string $orderId,
    ) {}

    public function handle(): void
    {
        $ordersUrl = config('services.orders.url');
        $erpUrl    = config('services.erp.url');
        $notifUrl  = config('services.notifications.url');

        // Fetch order details — interpolated URL with path param
        $order = Http::get("{$ordersUrl}/api/orders/{$this->orderId}")->json();

        // Push PDF metadata to legacy ERP — withHeaders chained + interpolated
        Http::withHeaders([
            'X-Api-Key' => config('services.erp.api_key'),
        ])->post("{$erpUrl}/api/erp/pdf-uploads", [
            'invoice_id' => $this->invoiceId,
        ]);

        // Notify user — concatenation form
        Http::withToken(config('services.notifications.token'))
            ->post($notifUrl . '/api/notifications', [
                'event' => 'invoice.pdf_ready',
            ]);
    }
}
`
	ids, rels := runDetectWithRels(t, "php", "GenerateInvoicePdf.php", src)

	// billing → orders GET (interpolated with $this->orderId — canonicalized to {orderId})
	requireContains(t, ids, []string{"http:GET:/api/orders/{orderId}"}, "#1473 pdf billing→orders GET interpolated")
	requireFetches(t, rels, "http:GET:/api/orders/{orderId}", "#1473 pdf billing→orders GET interpolated")

	// billing → legacy-erp POST (withHeaders chained + interpolated)
	requireContains(t, ids, []string{"http:POST:/api/erp/pdf-uploads"}, "#1473 pdf billing→legacy-erp POST interpolated chained")
	requireFetches(t, rels, "http:POST:/api/erp/pdf-uploads", "#1473 pdf billing→legacy-erp POST interpolated chained")

	// billing → notifications POST (withToken chained + concat)
	requireContains(t, ids, []string{"http:POST:/api/notifications"}, "#1473 pdf billing→notifications POST concat chained")
	requireFetches(t, rels, "http:POST:/api/notifications", "#1473 pdf billing→notifications POST concat chained")
}

// ---------------------------------------------------------------------------
// Guzzle variant
// ---------------------------------------------------------------------------

// TestPHP1473_GuzzleInterpolated verifies that Guzzle $client->get() /
// $client->post() calls with interpolated URLs are also resolved.
func TestPHP1473_GuzzleInterpolated(t *testing.T) {
	src := `<?php

use GuzzleHttp\Client;

class BillingService
{
    public function getOrder(string $id): array
    {
        $ordersUrl = config('services.orders.url');
        $client = new Client();
        $response = $client->get("{$ordersUrl}/api/orders/{$id}");
        return json_decode($response->getBody(), true);
    }

    public function createOrder(array $data): array
    {
        $ordersUrl = config('services.orders.url');
        $client = new Client();
        $response = $client->post("{$ordersUrl}/api/orders", ['json' => $data]);
        return json_decode($response->getBody(), true);
    }
}
`
	ids, rels := runDetectWithRels(t, "php", "BillingService.php", src)

	requireContains(t, ids, []string{"http:GET:/api/orders/{id}", "http:POST:/api/orders"}, "#1473 guzzle interpolated")
	requireFetches(t, rels, "http:GET:/api/orders/{id}", "#1473 guzzle interpolated GET")
	requireFetches(t, rels, "http:POST:/api/orders", "#1473 guzzle interpolated POST")
}

// ---------------------------------------------------------------------------
// Before/After measurement — fixture mirrors billing service
// ---------------------------------------------------------------------------

// TestPHP1473_BillingServiceLinkCount is the primary before/after measurement.
//
// BEFORE (#1473 open): 0 edges for variable-interpolated/concat URL forms.
// AFTER (this fix): all three billing→{orders,notifications,legacy-erp} edges
// emit correctly regardless of whether the URL is a literal, interpolated, or
// concatenated from a config() variable.
//
// Expected AFTER counts across two billing files combined:
//
//	billing → orders        : ≥ 2 edges (POST + GET from InvoiceController)
//	billing → notifications : ≥ 2 edges (POST from InvoiceController + GenerateInvoicePdf)
//	billing → legacy-erp   : ≥ 2 edges (POST /erp/invoices + POST /erp/pdf-uploads)
func TestPHP1473_BillingServiceLinkCount(t *testing.T) {
	invoiceControllerSrc := `<?php

namespace App\Http\Controllers;

use Illuminate\Support\Facades\Http;

class InvoiceController extends Controller
{
    public function store()
    {
        $ordersUrl = config('services.orders.url');
        $notifUrl  = config('services.notifications.url');
        $erpUrl    = config('services.erp.url');

        Http::post("{$ordersUrl}/api/orders", ['items' => []]);
        Http::post("{$notifUrl}/api/notifications", ['event' => 'invoice.created']);
        Http::withToken(config('services.erp.token'))
            ->post("{$erpUrl}/api/erp/invoices", ['sync' => true]);
    }

    public function show(string $id)
    {
        $ordersUrl = config('services.orders.url');
        Http::get("{$ordersUrl}/api/orders/{$id}");
    }
}
`

	generateInvoicePdfSrc := `<?php

namespace App\Jobs;

use Illuminate\Support\Facades\Http;

class GenerateInvoicePdf
{
    public function handle(): void
    {
        $ordersUrl = config('services.orders.url');
        $erpUrl    = config('services.erp.url');
        $notifUrl  = config('services.notifications.url');

        Http::get($ordersUrl . '/api/orders/123');
        Http::withHeaders(['X-Api-Key' => 'key'])
            ->post("{$erpUrl}/api/erp/pdf-uploads", []);
        Http::withToken('token')
            ->post($notifUrl . '/api/notifications', ['event' => 'pdf_ready']);
    }
}
`

	idsIC, relsIC := runDetectWithRels(t, "php", "InvoiceController.php", invoiceControllerSrc)
	idsPDF, relsPDF := runDetectWithRels(t, "php", "GenerateInvoicePdf.php", generateInvoicePdfSrc)

	// Deduplicate http: entity IDs across both files.
	allIDs := make(map[string]bool)
	for _, id := range append(idsIC, idsPDF...) {
		if strings.HasPrefix(id, "http:") {
			allIDs[id] = true
		}
	}

	// Count FETCHES edges by target.
	fetchesByTarget := make(map[string]int)
	totalFetches := 0
	for _, r := range append(relsIC, relsPDF...) {
		if r.Kind == fetchesEdgeKind {
			totalFetches++
			fetchesByTarget[r.ToID]++
		}
	}

	// Before/after report.
	t.Logf("=== #1473 before/after link count (variable-interpolated URLs) ===")
	t.Logf("BEFORE: 0 edges for interpolated/concat URL forms (config() base vars unresolved)")
	t.Logf("AFTER:  %d distinct http_endpoint_call entities, %d FETCHES edges across 2 billing files",
		len(allIDs), totalFetches)
	// Count all orders edges regardless of path-param variant
	// (e.g. http:GET:/api/orders/{id} and http:GET:/api/orders both count).
	ordersEdges := 0
	notifEdges := 0
	erpEdges := 0
	for target, count := range fetchesByTarget {
		switch {
		case strings.HasPrefix(target, "http:POST:/api/orders") ||
			strings.HasPrefix(target, "http:GET:/api/orders") ||
			strings.HasPrefix(target, "http:PUT:/api/orders") ||
			strings.HasPrefix(target, "http:PATCH:/api/orders"):
			ordersEdges += count
		case strings.HasPrefix(target, "http:POST:/api/notifications") ||
			strings.HasPrefix(target, "http:GET:/api/notifications"):
			notifEdges += count
		case strings.HasPrefix(target, "http:POST:/api/erp/") ||
			strings.HasPrefix(target, "http:PATCH:/api/erp/") ||
			strings.HasPrefix(target, "http:PUT:/api/erp/"):
			erpEdges += count
		}
	}

	t.Logf("  billing→orders        : %d edge(s)", ordersEdges)
	t.Logf("  billing→notifications : %d edge(s)", notifEdges)
	t.Logf("  billing→legacy-erp   : %d edge(s)", erpEdges)

	// Assert minimum AFTER counts.
	if ordersEdges < 2 {
		t.Errorf("#1473 billing→orders: expected ≥2 FETCHES edges, got %d", ordersEdges)
	}
	if notifEdges < 2 {
		t.Errorf("#1473 billing→notifications: expected ≥2 FETCHES edges, got %d", notifEdges)
	}
	if erpEdges < 2 {
		t.Errorf("#1473 billing→legacy-erp: expected ≥2 FETCHES edges, got %d", erpEdges)
	}
	if len(allIDs) < 4 {
		t.Errorf("#1473 expected ≥4 distinct http_endpoint_call entities, got %d: %v", len(allIDs), allIDs)
	}
	if totalFetches < 6 {
		t.Errorf("#1473 expected ≥6 total FETCHES edges, got %d", totalFetches)
	}
}

// ---------------------------------------------------------------------------
// Negative / no-regression
// ---------------------------------------------------------------------------

// TestPHP1473_NegativeNoFalsePositives confirms that plain variables without
// a known config/env assignment are NOT emitted (prevents false positives).
func TestPHP1473_NegativeNoFalsePositives(t *testing.T) {
	src := `<?php

use Illuminate\Support\Facades\Http;

function callUnknownBase(): void
{
    // $unknownVar has no prior assignment in this file — should NOT emit.
    Http::get("{$unknownVar}/api/things");
    Http::post($anotherUnknownVar . '/api/stuff', []);
}
`
	ids, _ := runDetectWithRels(t, "php", "unknown.php", src)

	for _, id := range ids {
		if strings.HasPrefix(id, "http:") {
			if strings.Contains(id, "/api/things") || strings.Contains(id, "/api/stuff") {
				t.Errorf("#1473 negative: unexpectedly emitted entity %q for unknown base var", id)
			}
		}
	}
}

// TestPHP1473_EnvAssignmentInterpolated verifies that env() assignments (not
// just config()) are also tracked as runtime-dynamic base URLs.
func TestPHP1473_EnvAssignmentInterpolated(t *testing.T) {
	src := `<?php

use Illuminate\Support\Facades\Http;

function callOrders(): void
{
    $baseUrl = env('ORDERS_SERVICE_URL');
    Http::get("{$baseUrl}/api/orders");
    Http::post($baseUrl . '/api/orders', []);
}
`
	ids, rels := runDetectWithRels(t, "php", "caller.php", src)

	requireContains(t, ids, []string{"http:GET:/api/orders", "http:POST:/api/orders"}, "#1473 env() interpolated")
	requireFetches(t, rels, "http:GET:/api/orders", "#1473 env() interpolated GET")
	requireFetches(t, rels, "http:POST:/api/orders", "#1473 env() interpolated POST")
}
