package javascript_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	extreg "github.com/cajasmota/grafel/internal/extractor"
	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/javascript"
)

// Issue #4325 LIVE-REPRO. Byte-copies of the REAL acme-v3 controllers are
// committed alongside this test as _repro_*.ts.txt. We run the actual
// nestjs extractor over them and assert on the endpoint's surfaced params,
// response_type, and edges — first proving the gap, then the fix.

func loadRepro(t *testing.T, base string) extreg.FileInput {
	t.Helper()
	p := filepath.Join("testdata", "issue4325", base)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read repro %s: %v", p, err)
	}
	return extreg.FileInput{Path: "src/" + base, Language: "typescript", Content: b}
}

func nestEndpoint(t *testing.T, file extreg.FileInput, methodName string) types.EntityRecord {
	t.Helper()
	e, ok := extreg.Get("custom_js_nestjs")
	if !ok {
		t.Fatal("custom_js_nestjs not registered")
	}
	ents, err := e.Extract(context.Background(), file)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, ent := range ents {
		if ent.Subtype == "endpoint" && ent.Properties["method_name"] == methodName {
			return ent
		}
	}
	t.Fatalf("endpoint with method_name=%q not found among %d entities", methodName, len(ents))
	return types.EntityRecord{}
}

func dumpEndpoint(t *testing.T, ep types.EntityRecord) {
	t.Logf("ENDPOINT name=%q verb=%s path=%q method=%q",
		ep.Name, ep.Properties["http_method"], ep.Properties["route_path"], ep.Properties["method_name"])
	t.Logf("  parameters prop = %q", ep.Properties["parameters"])
	t.Logf("  response_type prop = %q", ep.Properties["response_type"])
	for _, r := range ep.Relationships {
		t.Logf("  EDGE %s -> %s  props=%v", r.Kind, r.ToID, r.Properties)
	}
}

// device.controller.ts `filters` handler: two @Query params + Promise<DeviceFiltersResponse>.
func TestIssue4325_LiveRepro_DeviceFilters(t *testing.T) {
	ep := nestEndpoint(t, loadRepro(t, "device.controller.ts"), "filters")
	dumpEndpoint(t, ep)

	// --- ASSERT FIX (after) ---
	params := ep.Properties["parameters"]
	if params == "" {
		t.Errorf("GAP: parameters empty — @Query('group_id')/@Query('building_id') not surfaced")
	}
	if !strings.Contains(params, "group_id") || !strings.Contains(params, "building_id") {
		t.Errorf("GAP: query params group_id/building_id missing from parameters=%q", params)
	}
	if rt := ep.Properties["response_type"]; rt != "DeviceFiltersResponse" {
		t.Errorf("GAP: response_type=%q, want DeviceFiltersResponse", rt)
	}
}

// building.controller.ts `createNote` handler: @Body() BuildingNoteCreateBody + Promise<BuildingNoteCreateResponse>.
func TestIssue4325_LiveRepro_BuildingCreateNote(t *testing.T) {
	ep := nestEndpoint(t, loadRepro(t, "building.controller.ts"), "createNote")
	dumpEndpoint(t, ep)

	params := ep.Properties["parameters"]
	if !strings.Contains(params, "BuildingNoteCreateBody") {
		t.Errorf("GAP: @Body() DTO BuildingNoteCreateBody not surfaced as a body param; parameters=%q", params)
	}
	if rt := ep.Properties["response_type"]; rt != "BuildingNoteCreateResponse" {
		t.Errorf("GAP: response_type=%q, want BuildingNoteCreateResponse", rt)
	}
}

// Synthetic unit coverage for the decorator-stacking + param mechanisms that
// the real fixtures exercise, kept compact so the wire shape is asserted
// directly (the fixtures assert end-to-end on real source).
func TestIssue4325_DecoratorStacking_And_Params(t *testing.T) {
	src := `
import { Controller, Get, Post, Body, Query, Param, HttpCode, HttpStatus } from '@nestjs/common';
@Controller('things')
export class ThingController {
  @Get(':id')
  one(@Param('id') id: string, @Query('q') q: string): Promise<ThingResponse> {
    return this.svc.one(id);
  }

  @Post('create')
  @HttpCode(HttpStatus.OK)
  @UseGuards(AuthGuard)
  create(@Body() dto: CreateThingDto): Promise<ThingResponse> {
    return this.svc.create(dto);
  }
}
`
	one := nestEndpoint(t, fi("t.ts", "typescript", src), "one")
	if got := one.Properties["http_method"]; got != "GET" {
		t.Errorf("one verb=%q want GET", got)
	}
	if p := one.Properties["parameters"]; !strings.Contains(p, `"in":"path"`) || !strings.Contains(p, `"in":"query"`) {
		t.Errorf("one parameters missing path/query: %q", p)
	}
	if rt := one.Properties["response_type"]; rt != "ThingResponse" {
		t.Errorf("one response_type=%q want ThingResponse", rt)
	}

	// The POST sits behind @HttpCode + @UseGuards — pre-#4325 it would be
	// dropped entirely by the adjacency-only verb regex.
	create := nestEndpoint(t, fi("t.ts", "typescript", src), "create")
	if got := create.Properties["http_method"]; got != "POST" {
		t.Errorf("create verb=%q want POST (decorator stacking regression)", got)
	}
	if p := create.Properties["parameters"]; !strings.Contains(p, `"in":"body"`) || !strings.Contains(p, "CreateThingDto") {
		t.Errorf("create parameters missing body DTO: %q", p)
	}
	if rt := create.Properties["response_type"]; rt != "ThingResponse" {
		t.Errorf("create response_type=%q want ThingResponse", rt)
	}
}
