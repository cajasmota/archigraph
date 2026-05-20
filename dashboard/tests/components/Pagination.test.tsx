import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Pagination } from '@/components/paths/Pagination'

vi.mock('lucide-react', () => ({
  ChevronLeft: (props: React.SVGProps<SVGSVGElement>) => <svg {...props} />,
  ChevronRight: (props: React.SVGProps<SVGSVGElement>) => <svg {...props} />,
}))

describe('Pagination', () => {
  it('shows current range', () => {
    render(<Pagination page={1} pageSize={50} total={648} hasMore onPageChange={() => undefined} />)
    expect(screen.getByText('1–50 of 648')).toBeInTheDocument()
  })

  it('disables previous button on first page', () => {
    render(<Pagination page={1} pageSize={50} total={648} hasMore onPageChange={() => undefined} />)
    expect(screen.getByLabelText('Previous page')).toBeDisabled()
  })

  it('disables next button when !hasMore', () => {
    render(<Pagination page={13} pageSize={50} total={648} hasMore={false} onPageChange={() => undefined} />)
    expect(screen.getByLabelText('Next page')).toBeDisabled()
  })

  it('calls onPageChange with incremented page on next click', async () => {
    const user = userEvent.setup()
    const onPageChange = vi.fn()
    render(<Pagination page={2} pageSize={50} total={300} hasMore onPageChange={onPageChange} />)
    await user.click(screen.getByLabelText('Next page'))
    expect(onPageChange).toHaveBeenCalledWith(3)
  })

  it('calls onPageChange with decremented page on prev click', async () => {
    const user = userEvent.setup()
    const onPageChange = vi.fn()
    render(<Pagination page={3} pageSize={50} total={300} hasMore onPageChange={onPageChange} />)
    await user.click(screen.getByLabelText('Previous page'))
    expect(onPageChange).toHaveBeenCalledWith(2)
  })

  it('shows "No results" when total is 0', () => {
    render(<Pagination page={1} pageSize={50} total={0} hasMore={false} onPageChange={() => undefined} />)
    expect(screen.getByText('No results')).toBeInTheDocument()
  })
})
