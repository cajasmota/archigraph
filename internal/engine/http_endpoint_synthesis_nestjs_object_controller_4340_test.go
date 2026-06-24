package engine

// Regression test for #4340 — NestJS object-form `@Controller({ path, version })`
// base path was not extracted, so the controller prefix was DROPPED and every
// method route normalised identically across controllers, collapsing distinct
// endpoints onto one synthetic ("defined in N controllers").
//
// Real source (acme-backend-v3):
//
//	@Controller({ path: 'companies/inspection',  version: '1' })  inspection-company.controller.ts:17
//	@Controller({ path: 'companies/witnessing',  version: '1' })  witnessing-company.controller.ts:17
//	@Controller({ path: 'companies/contracting', version: '1' })  contracting-company.controller.ts:17
//
// Each declares a root `@Post()` (POST /) and a `@Post(':companyId/hide')`.
// With the prefix dropped, all three collapse onto `http:POST:/` and
// `http:POST:/{companyId}/hide`.

import "testing"

// The three real company controllers, object-form @Controller, byte-faithful
// to the decorator + the two POST routes that collapse (root + hide).
const nestObjFormInspection = `import { Body, Controller, Get, HttpCode, HttpStatus, Param, ParseIntPipe, Post, Req } from '@nestjs/common';

@Controller({ path: 'companies/inspection', version: '1' })
export class InspectionCompanyController {
  constructor(private readonly service: InspectionCompanyService) {}

  @Post()
  @HttpCode(HttpStatus.CREATED)
  create(@Body() body: CreateInspectionCompanyBody): Promise<InspectionCompanyResponse> {
    return this.service.create(body);
  }

  @Post(':companyId/hide')
  @HttpCode(HttpStatus.OK)
  hide(@Param('companyId', ParseIntPipe) companyId: number, @Body() body: HideCompanyBody): Promise<{ hidden: boolean }> {
    return this.service.hide(companyId, body);
  }
}
`

const nestObjFormWitnessing = `import { Body, Controller, HttpCode, HttpStatus, Param, ParseIntPipe, Post } from '@nestjs/common';

@Controller({ path: 'companies/witnessing', version: '1' })
export class WitnessingCompanyController {
  constructor(private readonly service: WitnessingCompanyService) {}

  @Post()
  @HttpCode(HttpStatus.CREATED)
  create(@Body() body: CreateWitnessingCompanyBody): Promise<WitnessingCompanyResponse> {
    return this.service.create(body);
  }

  @Post(':companyId/hide')
  @HttpCode(HttpStatus.OK)
  hide(@Param('companyId', ParseIntPipe) companyId: number, @Body() body: HideCompanyBody): Promise<{ hidden: boolean }> {
    return this.service.hide(companyId, body);
  }
}
`

const nestObjFormContracting = `import { Body, Controller, HttpCode, HttpStatus, Param, ParseIntPipe, Post } from '@nestjs/common';

@Controller({ path: 'companies/contracting', version: '1' })
export class ContractingCompanyController {
  constructor(private readonly service: ContractingCompanyService) {}

  @Post()
  @HttpCode(HttpStatus.CREATED)
  create(@Body() body: CreateContractingCompanyBody): Promise<ContractingCompanyResponse> {
    return this.service.create(body);
  }

  @Post(':companyId/hide')
  @HttpCode(HttpStatus.OK)
  hide(@Param('companyId', ParseIntPipe) companyId: number, @Body() body: HideCompanyBody): Promise<{ hidden: boolean }> {
    return this.service.hide(companyId, body);
  }
}
`

// nestObjFormIDs runs detect on each controller file and returns the union of
// endpoint synthetic IDs across the three (mirroring the cross-file merge that
// collapses them in the real graph).
func nestObjFormIDs(t *testing.T) []string {
	t.Helper()
	files := []struct{ path, src string }{
		{"src/modules/inspection-companies/api/inspection-company.controller.ts", nestObjFormInspection},
		{"src/modules/witnessing-companies/api/witnessing-company.controller.ts", nestObjFormWitnessing},
		{"src/modules/contracting-companies/api/contracting-company.controller.ts", nestObjFormContracting},
	}
	seen := map[string]int{}
	for _, f := range files {
		ids, _ := runDetect(t, "typescript", f.path, f.src)
		for _, id := range ids {
			seen[id]++
		}
	}
	out := make([]string, 0, len(seen))
	for id, n := range seen {
		t.Logf("endpoint %s emitted by %d controller(s)", id, n)
		out = append(out, id)
	}
	return out
}

// TestNestJS_ObjectFormController_4340_DistinctEndpoints proves the object-form
// @Controller({path,version}) prefix is honoured: the three company controllers
// produce three DISTINCT root POSTs and three DISTINCT hide endpoints, instead
// of collapsing onto http:POST:/ and http:POST:/{companyId}/hide.
//
// Before the fix this test FAILS: the prefix is dropped and all three collapse.
func TestNestJS_ObjectFormController_4340_DistinctEndpoints(t *testing.T) {
	ids := nestObjFormIDs(t)
	has := func(want string) bool {
		for _, id := range ids {
			if id == want {
				return true
			}
		}
		return false
	}

	// The collapsed (buggy) IDs MUST NOT be present after the fix.
	collapsed := []string{
		"http:POST:/",
		"http:POST:/{companyId}/hide",
	}
	for _, c := range collapsed {
		if has(c) {
			t.Errorf("#4340 COLLAPSE: prefix dropped, endpoints collapsed onto %q (got: %v)", c, ids)
		}
	}

	// The three distinct controller-rooted endpoints MUST be present.
	wantDistinct := []string{
		"http:POST:/v1/companies/inspection",
		"http:POST:/v1/companies/witnessing",
		"http:POST:/v1/companies/contracting",
		"http:POST:/v1/companies/inspection/{companyId}/hide",
		"http:POST:/v1/companies/witnessing/{companyId}/hide",
		"http:POST:/v1/companies/contracting/{companyId}/hide",
	}
	for _, w := range wantDistinct {
		if !has(w) {
			t.Errorf("#4340: missing distinct endpoint %q (got: %v)", w, ids)
		}
	}
}

// TestNestJS_AllControllerForms_4340 covers the full matrix of @Controller
// argument forms so the generalization (not just the object form) is locked in.
func TestNestJS_AllControllerForms_4340(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // expected synthetic ID for the single @Get('x') route
	}{
		{
			name: "no-arg root",
			src: `@Controller()
export class RootController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/x",
		},
		{
			name: "bare string",
			src: `@Controller('users')
export class UsersController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/users/x",
		},
		{
			name: "array first-host",
			src: `@Controller(['users', 'people'])
export class UsersController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/users/x",
		},
		{
			name: "object path only",
			src: `@Controller({ path: 'users' })
export class UsersController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/users/x",
		},
		{
			name: "object path + version",
			src: `@Controller({ path: 'users', version: '2' })
export class UsersController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/v2/users/x",
		},
		{
			name: "object version-only root",
			src: `@Controller({ version: '1' })
export class RootController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/v1/x",
		},
		{
			name: "object path-array + version",
			src: `@Controller({ path: ['users', 'people'], version: '1' })
export class UsersController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/v1/users/x",
		},
		{
			name: "object numeric version",
			src: `@Controller({ path: 'users', version: 1 })
export class UsersController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/v1/users/x",
		},
		{
			name: "object multiline",
			src: `@Controller({
  path: 'users',
  version: '3',
})
export class UsersController {
  @Get('x')
  x() {}
}`,
			want: "http:GET:/v3/users/x",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ids, _ := runDetect(t, "typescript", "src/users/users.controller.ts", tc.src)
			requireContains(t, ids, []string{tc.want}, tc.name)
		})
	}
}
