package javascript_test

import (
	"testing"

	"github.com/cajasmota/grafel/internal/types"

	_ "github.com/cajasmota/grafel/internal/custom/javascript"
)

// hasDTOEdge reports whether some entity carries an edge of (kind -> toID).
// FromID is empty: graph assembly binds it to the emitting endpoint entity.
func hasDTOEdge(ents []types.EntityRecord, kind, toID string) bool {
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == kind && r.ToID == toID {
				return true
			}
		}
	}
	return false
}

// dtoEdgeOwner returns the name of the entity carrying the (kind -> toID) edge.
func dtoEdgeOwner(ents []types.EntityRecord, kind, toID string) string {
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == kind && r.ToID == toID {
				return e.Name
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// NestJS (#3629/#3607)
// ---------------------------------------------------------------------------

// @Body() dto: CreateUserDto → endpoint ACCEPTS_INPUT CreateUserDto.
func TestNestReqResp_AcceptsInputEdge(t *testing.T) {
	src := `@Controller('users')
export class UsersController {
  @Post()
  create(@Body() dto: CreateUserDto) {
    return this.svc.create(dto);
  }
}`
	ents := extractFull(t, "custom_js_nestjs", fi("users.controller.ts", "typescript", src))
	if !hasDTOEdge(ents, "ACCEPTS_INPUT", "Class:CreateUserDto") {
		t.Fatal("expected ACCEPTS_INPUT -> Class:CreateUserDto")
	}
	if owner := dtoEdgeOwner(ents, "ACCEPTS_INPUT", "Class:CreateUserDto"); owner != "POST create" {
		t.Errorf("expected edge owner 'POST create', got %q", owner)
	}
}

// return type Promise<UserDto> → endpoint RETURNS UserDto (generic unwrapped).
func TestNestReqResp_ReturnsEdgePromise(t *testing.T) {
	src := `@Controller('users')
export class UsersController {
  @Get(':id')
  async findOne(@Param('id') id: string): Promise<UserDto> {
    return this.svc.find(id);
  }
}`
	ents := extractFull(t, "custom_js_nestjs", fi("users.controller.ts", "typescript", src))
	if !hasDTOEdge(ents, "RETURNS", "Class:UserDto") {
		t.Fatal("expected RETURNS -> Class:UserDto from Promise<UserDto>")
	}
}

// Bare return type without a wrapper → RETURNS the type directly.
func TestNestReqResp_ReturnsEdgeBare(t *testing.T) {
	src := `@Controller('orders')
export class OrdersController {
  @Post()
  create(@Body() dto: CreateOrderDto): OrderDto {
    return null;
  }
}`
	ents := extractFull(t, "custom_js_nestjs", fi("orders.controller.ts", "typescript", src))
	if !hasDTOEdge(ents, "ACCEPTS_INPUT", "Class:CreateOrderDto") {
		t.Error("expected ACCEPTS_INPUT -> Class:CreateOrderDto")
	}
	if !hasDTOEdge(ents, "RETURNS", "Class:OrderDto") {
		t.Error("expected RETURNS -> Class:OrderDto")
	}
}

// Primitive body param (no DTO type) → no ACCEPTS_INPUT edge. Honest-partial.
func TestNestReqResp_PrimitiveBodyNoEdge(t *testing.T) {
	src := `@Controller('ping')
export class PingController {
  @Post()
  ping(@Body() value: string): boolean {
    return true;
  }
}`
	ents := extractFull(t, "custom_js_nestjs", fi("ping.controller.ts", "typescript", src))
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == "ACCEPTS_INPUT" {
				t.Errorf("expected no ACCEPTS_INPUT edge for primitive body, got -> %s", r.ToID)
			}
			if r.Kind == "RETURNS" {
				t.Errorf("expected no RETURNS edge for boolean return, got -> %s", r.ToID)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Express (#3629/#3607) — TypeScript typed handlers only.
// ---------------------------------------------------------------------------

// Request<P, ResBody, ReqBody> generic: 3rd arg is the request DTO.
func TestExpressReqResp_TypedRequestBody(t *testing.T) {
	src := `import { Request, Response } from 'express';
router.post('/users', (req: Request<{}, UserDto, CreateUserDto>, res: Response) => {
  res.json(svc.create(req.body));
});`
	ents := extractFull(t, "custom_js_express", fi("users.routes.ts", "typescript", src))
	if !hasDTOEdge(ents, "ACCEPTS_INPUT", "Class:CreateUserDto") {
		t.Fatal("expected ACCEPTS_INPUT -> Class:CreateUserDto from Request<,,CreateUserDto>")
	}
	if !hasDTOEdge(ents, "RETURNS", "Class:UserDto") {
		t.Error("expected RETURNS -> Class:UserDto from Request<,UserDto,>")
	}
}

// Response<UserDto> generic: typed response DTO.
func TestExpressReqResp_TypedResponse(t *testing.T) {
	src := `import { Request, Response } from 'express';
router.get('/me', (req: Request, res: Response<ProfileDto>) => {
  res.json(profile);
});`
	ents := extractFull(t, "custom_js_express", fi("me.routes.ts", "typescript", src))
	if !hasDTOEdge(ents, "RETURNS", "Class:ProfileDto") {
		t.Fatal("expected RETURNS -> Class:ProfileDto from Response<ProfileDto>")
	}
}

// Untyped req.body handler → no edge (honest-partial).
func TestExpressReqResp_UntypedNoEdge(t *testing.T) {
	src := `router.post('/users', (req, res) => {
  res.json(svc.create(req.body));
});`
	ents := extractFull(t, "custom_js_express", fi("users.routes.ts", "typescript", src))
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == "ACCEPTS_INPUT" || r.Kind == "RETURNS" {
				t.Errorf("expected no DTO edge for untyped handler, got %s -> %s", r.Kind, r.ToID)
			}
		}
	}
}

// JavaScript (no type annotations) → no DTO edges regardless of shape.
func TestExpressReqResp_JavaScriptNoEdge(t *testing.T) {
	src := `router.post('/users', (req, res) => { res.json({}); });`
	ents := extractFull(t, "custom_js_express", fi("users.routes.js", "javascript", src))
	for _, e := range ents {
		for _, r := range e.Relationships {
			if r.Kind == "ACCEPTS_INPUT" || r.Kind == "RETURNS" {
				t.Errorf("expected no DTO edge in JS, got %s -> %s", r.Kind, r.ToID)
			}
		}
	}
}
