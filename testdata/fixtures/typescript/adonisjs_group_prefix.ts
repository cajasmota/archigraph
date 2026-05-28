// AdonisJS route-group prefix fixture (#2934). Hand-written,
// dependency-manifest-free. Asserts that routes enclosed by a
// `Route.group(() => {...}).prefix('/x')` compose the group prefix, and that
// nested groups stack their prefixes.
import Route from '@ioc:Adonis/Core/Route'

// Single group prefix → /admin/users, /admin/users/{id}.
Route.group(() => {
  Route.get('/users', 'AdminUsersController.index')
  Route.post('/users', 'AdminUsersController.store')
  Route.get('/users/:id', 'AdminUsersController.show')
}).prefix('/admin')

// Nested groups → /api/v1/reports.
Route.group(() => {
  Route.group(() => {
    Route.get('/reports', 'ReportsController.index')
  }).prefix('/v1')
}).prefix('/api')

// Ungrouped route stays bare → /health (composition is a no-op here).
Route.get('/health', 'HealthController.show')
