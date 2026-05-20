import { useState, useEffect, useCallback } from 'react'

type Theme = 'dark' | 'light'

function getInitialTheme(): Theme {
  if (typeof window === 'undefined') return 'dark'
  const stored = localStorage.getItem('theme')
  if (stored === 'dark' || stored === 'light') return stored
  // Default: system preference, fallback dark
  return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

function applyTheme(theme: Theme) {
  const html = document.documentElement
  if (theme === 'dark') {
    html.classList.add('dark')
  } else {
    html.classList.remove('dark')
  }
  localStorage.setItem('theme', theme)
}

/**
 * Manages light/dark theme state.
 * Persists to localStorage, syncs to .dark on <html>, and
 * exposes a toggle handler.
 */
export function useTheme() {
  const [theme, setTheme] = useState<Theme>(getInitialTheme)

  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  const toggle = useCallback(() => {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'))
  }, [])

  return { theme, toggle, isDark: theme === 'dark' }
}
