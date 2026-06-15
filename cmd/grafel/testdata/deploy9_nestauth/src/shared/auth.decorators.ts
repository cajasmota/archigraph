// Trimmed copy of core-backend-v2 src/shared/auth/permissions/permissions.decorators.ts
// (deploy-9 fixture — REAL idioms, not edited in place). Each decorator records a
// PermissionRequirement as route metadata; a single GLOBAL PermissionsGuard reads
// it via Reflector.getAllAndOverride([handler, class]).
import { SetMetadata } from '@nestjs/common';

export const PERMISSIONS_METADATA = 'upvate:permissions' as const;

export const Public = (): MethodDecorator & ClassDecorator =>
  SetMetadata(PERMISSIONS_METADATA, { kind: 'public' });

export const Authenticated = (): MethodDecorator & ClassDecorator =>
  SetMetadata(PERMISSIONS_METADATA, { kind: 'authenticated' });

export const RequirePage = (slug: string): MethodDecorator & ClassDecorator =>
  SetMetadata(PERMISSIONS_METADATA, { kind: 'page', pages: [slug] });

export const AnyPage = (...slugs: string[]): MethodDecorator & ClassDecorator =>
  SetMetadata(PERMISSIONS_METADATA, { kind: 'any-page', pages: slugs });

export const InternalKeyOrAuth = (): MethodDecorator & ClassDecorator =>
  SetMetadata(PERMISSIONS_METADATA, { kind: 'internal-key' });
