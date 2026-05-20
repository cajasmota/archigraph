import { Navigate } from 'react-router-dom'

export function IndexRoute() {
  return <Navigate to="/api/fixture-a" replace />
}
