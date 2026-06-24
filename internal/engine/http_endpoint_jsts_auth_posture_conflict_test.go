// http_endpoint_jsts_auth_posture_conflict_test.go — regression for the
// contradictory auth-posture dual badge (#auth-posture-conflict).
//
// On acme-backend-v3, `DELETE /v1/checklists/{checklistId}` rendered BOTH a red
// "NO-AUTH · sensitive" badge AND a green "Auth · Bearer" badge. The method
// carries no own auth decorator — it inherits the CONTROLLER-level
// `@RequirePage(PermissionPage.Checklists)` guard. The engine resolver already
// reconciles that into a single authoritative posture (auth_required=true,
// auth_method=guard, auth_guard=@RequirePage(...)); the bug was downstream
// coverage code reading only the raw per-method signal keys and missing it.
//
// This test runs the REAL detector pipeline on a byte-copy of the controller and
// asserts the resolved posture is internally coherent: the inherited-guard route
// is AUTHENTICATED (so no consumer can call it NO-AUTH), and the explicit
// @Public() route is decisively public — never both.
package engine

import "testing"

// checklistControllerV3 is a byte-faithful copy of the decorator structure of
// acme-backend-v3 src/modules/checklists/api/checklist.controller.ts (bodies
// elided — only the routing/auth decorators are load-bearing for this pass).
const checklistControllerV3 = `
import { Public, RequirePage } from '../../../common/auth/decorators/auth.decorators';
import { PermissionPage } from '../../../common/auth/permission-page.enum';

@Controller({ path: 'checklists', version: '1' })
@RequirePage(PermissionPage.Checklists)
export class ChecklistController {
  constructor(private readonly service: ChecklistService) {}

  @Get()
  list(): Promise<any> { return null; }

  @Get(':checklistId')
  retrieve(): Promise<any> { return null; }

  @Post()
  @HttpCode(HttpStatus.CREATED)
  create(): Promise<any> { return null; }

  @Put(':checklistId')
  update(): Promise<any> { return null; }

  @Delete(':checklistId')
  @HttpCode(HttpStatus.OK)
  destroy(): Promise<any> { return null; }

  @Get(':checklistId/items')
  @Public()
  getItems(): Promise<any> { return null; }
}
`

func TestAuthPostureConflict_ChecklistInheritedGuard(t *testing.T) {
	eps := authProps(t, "typescript", "src/modules/checklists/api/checklist.controller.ts", checklistControllerV3)

	// The reported endpoint: gated only by the inherited controller-level
	// @RequirePage guard (no own decorator). It MUST resolve to authenticated
	// with a guard evidence stamp — never NO-AUTH.
	del, ok := eps["DELETE /v1/checklists/{checklistId}"]
	if !ok {
		t.Fatalf("DELETE endpoint not synthesised (got: %v)", keysOf(eps))
	}
	if del.Properties["auth_required"] != "true" {
		t.Errorf("DELETE: auth_required=%q, want true — inherited @RequirePage guard (props: %v)",
			del.Properties["auth_required"], del.Properties)
	}
	if del.Properties["auth_method"] != "guard" {
		t.Errorf("DELETE: auth_method=%q, want guard", del.Properties["auth_method"])
	}
	// A guard evidence symbol must be stamped so a coverage consumer that keys on
	// the raw signal-1 property (not just auth_required) also resolves to authed —
	// reconciliation belt-and-braces.
	if del.Properties["auth_guard"] == "" {
		t.Errorf("DELETE: auth_guard not stamped — coverage would mislabel NO-AUTH (props: %v)",
			del.Properties)
	}

	// Mutual exclusivity: an authenticated route must NOT also carry an
	// explicit-public verdict (auth_required=false). The two are reconciled into
	// one posture.
	if del.Properties["auth_required"] == "false" {
		t.Errorf("DELETE: carries BOTH authed and public verdicts — the contradiction this fixes")
	}

	// The explicit @Public() route is decisively public (genuine no-auth by
	// design) — the opposite, equally-coherent verdict.
	items, ok := eps["GET /v1/checklists/{checklistId}/items"]
	if !ok {
		t.Fatalf("GET items endpoint not synthesised (got: %v)", keysOf(eps))
	}
	if items.Properties["auth_required"] != "false" {
		t.Errorf("GET items: auth_required=%q, want false — explicit @Public() overrides inherited guard (props: %v)",
			items.Properties["auth_required"], items.Properties)
	}
}
