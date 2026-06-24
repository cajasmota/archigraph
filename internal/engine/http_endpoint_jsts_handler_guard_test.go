// http_endpoint_jsts_handler_guard_test.go — regression for handler-level
// (method-decorator) NestJS guard resolution (#4535).
//
// The #4509 reconcile resolved CLASS-level controller guards but missed or
// class-overrode HANDLER-level (method-decorator) guards, the #1 blocker the
// auth_posture_diff consumer reported (Wave-3 feedback). Root cause: the
// handler-method locator regex used `\([^)]*\)`, which stops at the first inner
// `)` — the one closing a decorated parameter (`@Query()`, `@Param('id',
// ParseIntPipe)`). So every handler with a parenthesised/decorated parameter
// (almost all real handlers) was never located and silently fell back to the
// class-level guard or `unknown`.
//
// These fixtures mirror the byte-level decorator structure of real
// acme-backend-v3 controllers (bodies elided — only the routing/auth decorators
// and the decorated parameter shapes are load-bearing).
package engine

import "testing"

// buildingControllerV3 — per-handler `@RequirePage`/`@RequireAction` with NO
// class-level guard. Failure mode (A): every endpoint read NO-AUTH because the
// decorated `@Query()` / `@Param(...)` params hid the method from the locator.
const buildingControllerV3 = `
import { RequireAction, RequirePage } from '../../../common/auth/decorators/auth.decorators';
import { PermissionPage } from '../../../common/auth/page/permission-page';
import { PermissionAction } from '../../../common/auth/action/permission-action';

@Controller({ path: 'buildings', version: '1' })
export class BuildingController {
  constructor(private readonly service: BuildingService) {}

  @Get('lite')
  @RequireAction(PermissionAction.Lite)
  listLite(): Promise<any> { return null; }

  @Get('active')
  @RequirePage(PermissionPage.Buildings)
  listActive(@Query() query: BuildingActiveQuery): Promise<any> { return null; }

  @Post()
  @RequirePage(PermissionPage.Buildings)
  create(@Body() body: BuildingCreateBody): Promise<any> { return null; }

  @Get(':buildingId')
  @RequirePage(PermissionPage.Buildings)
  retrieve(@Param('buildingId', ParseIntPipe) buildingId: number): Promise<any> { return null; }

  @Delete(':buildingId')
  @RequirePage(PermissionPage.Buildings)
  @HttpCode(HttpStatus.OK)
  destroy(@Param('buildingId', ParseIntPipe) buildingId: number): Promise<any> { return null; }
}
`

// clientControllerV3 — class-level `@RequirePage(Clients)` with a handler that
// OVERRIDES it via `@RequirePage(ContractProposals)`. Failure mode (B): the
// resolver reported the class guard because the override handler was never
// located.
const clientControllerV3 = `
import { RequirePage } from '../../../common/auth/decorators/auth.decorators';
import { PermissionPage } from '../../../common/auth/page/permission-page';

@Controller({ path: 'clients', version: '1' })
@RequirePage(PermissionPage.Clients)
export class ClientController {
  constructor(private readonly service: ClientService) {}

  @Get()
  list(@Query() query: ClientListQuery): Promise<any> { return null; }

  @Get(':clientId')
  retrieve(@Param('clientId', ParseIntPipe) clientId: number): Promise<any> { return null; }

  @Get(':clientId/contracts')
  @RequirePage(PermissionPage.ContractProposals)
  listContracts(@Param('clientId', ParseIntPipe) clientId: number, @Query() query: ClientContractsQuery): Promise<any> { return null; }
}
`

// v3IdiomsControllerV3 — the additional v3 metadata idioms the consumer listed:
// @RequireAnyPage(A, B), @AuthenticatedOrInternalKey, @RequireSuperuser, plus a
// handler @Public() override of a class guard.
const v3IdiomsControllerV3 = `
import { RequireAnyPage, AuthenticatedOrInternalKey, RequireSuperuser, Public, RequirePage } from '../../../common/auth/decorators/auth.decorators';
import { PermissionPage } from '../../../common/auth/page/permission-page';

@Controller({ path: 'inspections', version: '1' })
@RequirePage(PermissionPage.Inspections)
export class InspectionController {
  constructor(private readonly service: InspectionService) {}

  @Get('to-reschedule')
  @RequireAnyPage(PermissionPage.Inspections, PermissionPage.Scheduling)
  toReschedule(@Query() query: RescheduleQuery): Promise<any> { return null; }

  @Get('me-content')
  @AuthenticatedOrInternalKey()
  meContent(@Query() query: MeContentQuery): Promise<any> { return null; }

  @Delete(':inspectionId')
  @RequireSuperuser()
  @HttpCode(HttpStatus.OK)
  destroy(@Param('inspectionId', ParseIntPipe) inspectionId: number): Promise<any> { return null; }

  @Get(':inspectionId/public-status')
  @Public()
  publicStatus(@Param('inspectionId', ParseIntPipe) inspectionId: number): Promise<any> { return null; }
}
`

// TestHandlerGuard_FalseNoAuth — failure mode (A): per-handler guards with no
// class-level guard MUST resolve to authenticated, not NO-AUTH/unknown.
func TestHandlerGuard_FalseNoAuth(t *testing.T) {
	eps := authProps(t, "typescript", "src/modules/buildings/api/building.controller.ts", buildingControllerV3)
	for _, k := range []string{
		"GET /v1/buildings/active",
		"POST /v1/buildings",
		"GET /v1/buildings/{buildingId}",
		"DELETE /v1/buildings/{buildingId}",
		"GET /v1/buildings/lite",
	} {
		e, ok := eps[k]
		if !ok {
			t.Fatalf("%s not synthesised (got: %v)", k, keysOf(eps))
		}
		if e.Properties["auth_required"] != "true" {
			t.Errorf("%s: auth_required=%q, want true — per-handler guard not resolved (props: %v)",
				k, e.Properties["auth_required"], e.Properties)
		}
		if e.Properties["auth_method"] != "guard" {
			t.Errorf("%s: auth_method=%q, want guard", k, e.Properties["auth_method"])
		}
		if e.Properties["auth_guard"] == "" {
			t.Errorf("%s: auth_guard not stamped — coverage would mislabel NO-AUTH (props: %v)", k, e.Properties)
		}
	}
	// The page-guarded routes carry the page literal; the action route carries it
	// (the @RequireAction guard).
	if g := eps["GET /v1/buildings/active"].Properties["auth_guard"]; g != "@RequirePage(PermissionPage.Buildings)" {
		t.Errorf("active: auth_guard=%q, want @RequirePage(PermissionPage.Buildings)", g)
	}
	if g := eps["GET /v1/buildings/lite"].Properties["auth_guard"]; g != "@RequireAction(PermissionAction.Lite)" {
		t.Errorf("lite: auth_guard=%q, want @RequireAction(PermissionAction.Lite)", g)
	}
}

// TestHandlerGuard_HandlerOverridesClass — failure mode (B): a handler-level
// guard MUST win over the class-level guard (most-specific wins, mirroring
// NestJS getAllAndOverride([handler, class])).
func TestHandlerGuard_HandlerOverridesClass(t *testing.T) {
	eps := authProps(t, "typescript", "src/modules/clients/api/client.controller.ts", clientControllerV3)

	// The override handler: must report the HANDLER guard (ContractProposals),
	// not the class guard (Clients).
	over, ok := eps["GET /v1/clients/{clientId}/contracts"]
	if !ok {
		t.Fatalf("override endpoint not synthesised (got: %v)", keysOf(eps))
	}
	if g := over.Properties["auth_guard"]; g != "@RequirePage(PermissionPage.ContractProposals)" {
		t.Errorf("contracts: auth_guard=%q, want the HANDLER guard @RequirePage(PermissionPage.ContractProposals) (props: %v)",
			g, over.Properties)
	}
	if over.Properties["auth_confidence"] != "high" {
		t.Errorf("contracts: auth_confidence=%q, want high (handler-direct)", over.Properties["auth_confidence"])
	}

	// A handler WITHOUT its own decorator still inherits the class guard (Clients)
	// at medium confidence — the class-level fallback must remain intact.
	inh, ok := eps["GET /v1/clients/{clientId}"]
	if !ok {
		t.Fatalf("inherited endpoint not synthesised")
	}
	if inh.Properties["auth_required"] != "true" {
		t.Errorf("retrieve: auth_required=%q, want true (inherited class guard)", inh.Properties["auth_required"])
	}
	if g := inh.Properties["auth_guard"]; g != "@RequirePage(PermissionPage.Clients)" {
		t.Errorf("retrieve: auth_guard=%q, want the inherited class guard @RequirePage(PermissionPage.Clients)", g)
	}
}

// TestHandlerGuard_V3Idioms — the additional v3 metadata idioms resolve to the
// right posture: @RequireAnyPage / @AuthenticatedOrInternalKey / @RequireSuperuser
// are protective; a handler @Public() overrides the class guard to public.
func TestHandlerGuard_V3Idioms(t *testing.T) {
	eps := authProps(t, "typescript", "src/modules/inspections/api/inspection.controller.ts", v3IdiomsControllerV3)

	for _, tc := range []struct {
		key, wantGuard string
	}{
		{"GET /v1/inspections/to-reschedule", "@RequireAnyPage(PermissionPage.Inspections, PermissionPage.Scheduling)"},
		{"GET /v1/inspections/me-content", "@AuthenticatedOrInternalKey()"},
		{"DELETE /v1/inspections/{inspectionId}", "@RequireSuperuser()"},
	} {
		e, ok := eps[tc.key]
		if !ok {
			t.Fatalf("%s not synthesised (got: %v)", tc.key, keysOf(eps))
		}
		if e.Properties["auth_required"] != "true" {
			t.Errorf("%s: auth_required=%q, want true (props: %v)", tc.key, e.Properties["auth_required"], e.Properties)
		}
		if g := e.Properties["auth_guard"]; g != tc.wantGuard {
			t.Errorf("%s: auth_guard=%q, want %q", tc.key, g, tc.wantGuard)
		}
	}

	// Handler @Public() overrides the class @RequirePage(Inspections) → public.
	pub, ok := eps["GET /v1/inspections/{inspectionId}/public-status"]
	if !ok {
		t.Fatalf("public-status endpoint not synthesised")
	}
	if pub.Properties["auth_required"] == "true" {
		t.Errorf("public-status: auth_required=true, want public — handler @Public() must override class guard (props: %v)",
			pub.Properties)
	}
}
