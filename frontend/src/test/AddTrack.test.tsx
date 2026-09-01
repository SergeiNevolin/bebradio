import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import AddTrack from '../components/AddTrack'
import { ToastProvider } from '../context/ToastContext'

function renderWithToast(ui: React.ReactElement) {
  return render(<ToastProvider>{ui}</ToastProvider>)
}

describe('AddTrack', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('renders input and button', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    expect(screen.getByPlaceholderText('Search or paste YouTube URL...')).toBeInTheDocument()
    expect(screen.getByText('Add')).toBeInTheDocument()
  })

  it('calls onAdd with url on submit', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: true })
    renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=123' } })
    fireEvent.click(screen.getByText('Add'))

    expect(onAdd).toHaveBeenCalledWith('https://youtube.com/watch?v=123')
  })

  it('clears input on success and shows toast', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: true })
    renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=123' } })
    fireEvent.click(screen.getByText('Add'))

    expect(await screen.findByText('Added!')).toBeInTheDocument()
    expect(input).toHaveValue('')
  })

  it('shows error toast on failure', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: false, error: 'Failed to add track' })
    renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=bad' } })
    fireEvent.click(screen.getByText('Add'))

    expect(await screen.findByText('Failed to add track')).toBeInTheDocument()
  })

  it('does not submit empty url', () => {
    const onAdd = vi.fn()
    renderWithToast(<AddTrack onAdd={onAdd} />)
    fireEvent.click(screen.getByText('Add'))
    expect(onAdd).not.toHaveBeenCalled()
  })

  it('submits on Enter key via form', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: true })
    const { container } = renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=123' } })
    fireEvent.submit(container.querySelector('form')!)

    expect(onAdd).toHaveBeenCalledWith('https://youtube.com/watch?v=123')
  })

  it('disables input and button while adding', async () => {
    let resolveAdd: (v: { success: boolean }) => void
    const onAdd = vi.fn().mockImplementation(() => new Promise((r) => { resolveAdd = r }))
    renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=123' } })
    fireEvent.click(screen.getByText('Add'))

    await waitFor(() => {
      expect(input).toBeDisabled()
    })
    expect(screen.getByText('Adding...')).toBeInTheDocument()

    resolveAdd!({ success: true })
    await waitFor(() => {
      expect(input).not.toBeDisabled()
    })
  })

  it('shows clear button when input has text', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')

    expect(screen.queryByRole('button', { name: '×' })).not.toBeInTheDocument()

    fireEvent.change(input, { target: { value: 'test' } })
    expect(screen.getByText('×')).toBeInTheDocument()
  })

  it('clears input when clear button is clicked', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')

    fireEvent.change(input, { target: { value: 'test query' } })
    fireEvent.click(screen.getByText('×'))

    expect(input).toHaveValue('')
  })

  it('shows searching spinner during search', async () => {
    vi.useFakeTimers()
    let resolveFetch: (v: unknown) => void
    global.fetch = vi.fn().mockImplementation(() =>
      new Promise((r) => { resolveFetch = r })
    )

    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')

    fireEvent.change(input, { target: { value: 'rock music' } })
    await vi.advanceTimersByTimeAsync(400)

    expect(document.querySelector('.search-spinner')).toBeInTheDocument()

    resolveFetch!({ json: () => [] })
    await vi.advanceTimersByTimeAsync(0)
    vi.useRealTimers()
  })

  it('shows no results message when search returns empty', async () => {
    vi.useFakeTimers()
    global.fetch = vi.fn().mockResolvedValue({ json: () => Promise.resolve([]) })

    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    const input = screen.getByPlaceholderText('Search or paste YouTube URL...')

    fireEvent.change(input, { target: { value: 'xyznonexistent' } })
    await vi.advanceTimersByTimeAsync(500)

    expect(screen.getByText('No results found')).toBeInTheDocument()
    vi.useRealTimers()
  })

  it('disables button when input is empty', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    expect(screen.getByText('Add')).toBeDisabled()
  })
})
