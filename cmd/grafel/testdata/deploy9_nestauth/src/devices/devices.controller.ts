// Deploy-9 fixture controller — mirrors the REAL core-backend-v2
// buildings.controller.ts decorator layout: a global PermissionsGuard +
// AuthenticationGuard (registered as APP_GUARD elsewhere) enforce auth; each
// route declares its requirement with a SetMetadata-based decorator. There is
// NO @UseGuards in this app — the pre-fix resolver therefore saw no auth.
import { Controller, Get, Post, Delete, Patch } from '@nestjs/common';
import { RequirePage, Authenticated, AnyPage, Public } from '../shared/auth.decorators';

@Controller('api/v1/devices')
export class DevicesController {
  // Page-gated read — the dominant case (@RequirePage on a @Get method).
  @Get()
  @RequirePage('devices.read')
  list() {}

  // Page-gated write (sensitive verb + page).
  @Post()
  @RequirePage('devices.write')
  create() {}

  // OR-over-pages requirement.
  @Get('summary')
  @AnyPage('devices.read', 'reports.read')
  summary() {}

  // Authenticated-only (no RBAC matrix) — still protected (401 before handler).
  @Get(':id/notes')
  @Authenticated()
  notes() {}

  // Page-gated destructive verb.
  @Delete(':id')
  @RequirePage('devices.write')
  remove() {}

  // Page-gated update.
  @Patch(':id')
  @RequirePage('devices.write')
  update() {}

  // Genuinely public (legacy AllowAny) — must NOT be flagged as protected, and
  // must be reported as no-auth by grafel_auth_coverage.
  @Public()
  @Get('health')
  health() {}
}
