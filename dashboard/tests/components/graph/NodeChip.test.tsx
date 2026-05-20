import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { NodeChip } from '@/components/graph/NodeChip'

vi.mock('lucide-react', async (importOriginal) => {
  const SvgStub = (p: React.SVGProps<SVGSVGElement>) => <svg {...p} />
  const actual = await importOriginal<typeof import('lucide-react')>()
  return {
    ...actual,
    FunctionSquare: SvgStub,
    Box: SvgStub,
    Component: SvgStub,
    File: SvgStub,
    Server: SvgStub,
    Globe: SvgStub,
    Database: SvgStub,
    Radio: SvgStub,
    MessageSquare: SvgStub,
    Workflow: SvgStub,
    Shapes: SvgStub,
    Network: SvgStub,
    Link2: SvgStub,
    Hash: SvgStub,
    LayoutGrid: SvgStub,
    Code: SvgStub,
    Layers: SvgStub,
    Zap: SvgStub,
    Settings: SvgStub,
    Table: SvgStub,
    Play: SvgStub,
    Folder: SvgStub,
    Package: SvgStub,
    FileText: SvgStub,
    Puzzle: SvgStub,
  }
})

describe('NodeChip', () => {
  it('renders label', () => {
    render(<NodeChip kind="Component" label="OrderService" />)
    expect(screen.getByText('OrderService')).toBeInTheDocument()
  })

  it('renders as span when no onClick', () => {
    const { container } = render(<NodeChip kind="Function" label="hashPw" />)
    expect(container.querySelector('span')).toBeTruthy()
    expect(container.querySelector('button')).toBeFalsy()
  })

  it('renders as button when onClick provided', () => {
    render(<NodeChip kind="Function" label="doThing" onClick={() => {}} />)
    expect(screen.getByRole('button')).toBeInTheDocument()
  })

  it('shows repo in title attribute', () => {
    render(<NodeChip kind="Service" label="UserSvc" repo="acme-api" />)
    const el = screen.getByTitle('UserSvc (acme-api)')
    expect(el).toBeTruthy()
  })
})
