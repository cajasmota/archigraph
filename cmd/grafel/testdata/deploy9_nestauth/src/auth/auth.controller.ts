// Deploy-9 fixture — mirrors the REAL core-backend-v2 auth.controller.ts shape:
// @Public() sits ABOVE the @Post verb decorator on each login/register route,
// plus one @InternalKeyOrAuth() route and a class-level @Authenticated() default
// with a per-method @Public() override (exercising getAllAndOverride precedence).
import { Controller, Get, Post } from '@nestjs/common';
import { Authenticated, Public, InternalKeyOrAuth } from '../shared/auth.decorators';

// Class-level default: every method is @Authenticated() unless it overrides.
@Authenticated()
@Controller('api/v1/auth')
export class AuthController {
  // Explicit public login route — overrides the class-level @Authenticated().
  @Public()
  @Post('login')
  login() {}

  // Explicit public registration.
  @Public()
  @Post('register')
  register() {}

  // Internal-key-or-auth gated webhook (protected).
  @InternalKeyOrAuth()
  @Post('webhook')
  webhook() {}

  // No method-level decorator → inherits the class-level @Authenticated()
  // (protected). Exercises class-level inheritance.
  @Get('me')
  me() {}
}
