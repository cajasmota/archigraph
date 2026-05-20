import { Link, NavLink, Outlet, useParams } from 'react-router-dom'
import { useState, useEffect } from 'react'
import {
  GitBranch, Network, Workflow, Radio, Globe, BookOpen,
  Moon, Sun, Settings, Menu, X,
} from 'lucide-react'
import { useRegistry } from '@/hooks/shared/useRegistry'
import { useThemeContext } from '@/context/ThemeContext'
import { GroupSwitcher } from '@/components/layout/GroupSwitcher'

const GROUP_DEFAULT = 'fixture-a'

export function AppLayout() {
  const { group = GROUP_DEFAULT } = useParams()
  const { data: registry } = useRegistry()
  const groups = registry?.groups ?? []

  // Keyboard shortcut: g h → go home
  useEffect(() => {
    let lastG = false
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'g' || e.key === 'G') {
        lastG = true
        setTimeout(() => { lastG = false }, 1000)
        return
      }
      if (lastG && (e.key === 'h' || e.key === 'H')) {
        e.preventDefault()
        window.location.href = '/'
      }
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [])

  // Mobile sidebar state
  const [drawerOpen, setDrawerOpen] = useState(false)
  const closeDrawer = () => setDrawerOpen(false)

  return (
    <div className="flex flex-col h-screen bg-slate-950 text-slate-200">
      {/* ── Top nav ──────────────────────────────────────────────────────────── */}
      <header className="flex items-center gap-4 px-4 h-12 border-b border-slate-800 flex-shrink-0 bg-slate-950/90 backdrop-blur-sm z-20">
        {/* Mobile hamburger — only on small screens */}
        <button
          type="button"
          aria-label="Open group navigation"
          aria-expanded={drawerOpen}
          onClick={() => setDrawerOpen((v) => !v)}
          className="lg:hidden p-1.5 rounded text-slate-500 hover:text-slate-300 hover:bg-slate-800 transition-colors"
        >
          {drawerOpen ? <X className="w-4 h-4" /> : <Menu className="w-4 h-4" />}
        </button>

        {/* Logo */}
        <Link to="/" className="flex items-center gap-2 font-bold text-sm tracking-tight text-sky-400 hover:text-sky-300">
          <GitBranch className="w-5 h-5" aria-hidden />
          archigraph
        </Link>

        {/* Surface nav — 5 chips (Graph / Flows / Topology / Paths / Docs) */}
        <nav className="flex items-center gap-1 ml-4" aria-label="Surface navigation">
          <NavItem to={`/graph/${group}`} icon={<Network className="w-4 h-4" />} label="Graph" />
          <NavItem to={`/flows/${group}`} icon={<Workflow className="w-4 h-4" />} label="Flows" />
          <NavItem to={`/topology/${group}`} icon={<Radio className="w-4 h-4" />} label="Topology" />
          <NavItem to={`/paths/${group}`} icon={<Globe className="w-4 h-4" />} label="Paths" />
          <NavItem to={`/docs/${group}`} icon={<BookOpen className="w-4 h-4" />} label="Docs" />
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
          <Link
            to="/settings"
            className="p-1.5 rounded text-slate-500 hover:text-slate-300 hover:bg-slate-800 transition-colors"
            aria-label="Settings"
          >
            <Settings className="w-4 h-4" />
          </Link>
        </div>
      </header>

      {/* ── Body: sidebar + content ───────────────────────────────────────────── */}
      <div className="flex flex-1 min-h-0 overflow-hidden">
        {/* ── Desktop sidebar ─────────────────────────────────────────────────── */}
        <aside
          className="hidden lg:flex flex-col w-52 min-w-[180px] border-r border-slate-800 bg-slate-950/80 py-3 overflow-y-auto flex-shrink-0"
          aria-label="Group navigation"
          data-testid="group-sidebar"
        >
          {groups.length > 0 ? (
            <GroupSwitcher groups={groups} />
          ) : (
            <p className="px-3 text-xs text-slate-600">Loading…</p>
          )}
        </aside>

        {/* ── Mobile slide-out drawer ──────────────────────────────────────────── */}
        {drawerOpen && (
          <>
            {/* Backdrop */}
            <div
              className="lg:hidden fixed inset-0 z-30 bg-black/50"
              aria-hidden
              onClick={closeDrawer}
            />
            {/* Drawer panel */}
            <div
              className="lg:hidden fixed top-12 left-0 bottom-0 z-40 w-64 bg-slate-950 border-r border-slate-800 py-3 overflow-y-auto"
              aria-label="Group navigation"
              data-testid="group-drawer"
            >
              <GroupSwitcher groups={groups} onNavigate={closeDrawer} />
            </div>
          </>
        )}

        {/* ── Page content ─────────────────────────────────────────────────────── */}
        <main className="flex-1 overflow-hidden">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

interface NavItemProps {
  to: string
  icon: React.ReactNode
  label: string
}

function NavItem({ to, icon, label }: NavItemProps) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        [
          'flex items-center gap-1.5 px-3 py-1.5 rounded text-sm transition-colors',
          isActive
            ? 'bg-slate-800 text-slate-200'
            : 'text-slate-500 hover:bg-slate-800/60 hover:text-slate-300',
        ].join(' ')
      }
    >
      {icon}
      {label}
    </NavLink>
  )
}

function ThemeToggle() {
  const { isDark, toggle } = useThemeContext()

  return (
    <button
      type="button"
      aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
      className="p-1.5 rounded text-slate-500 hover:text-slate-300 hover:bg-slate-800 transition-colors"
      onClick={toggle}
    >
      {isDark ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
    </button>
  )
}
