import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
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

  it('renders search input without a separate submit button', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    expect(screen.getByPlaceholderText('Search')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Add' })).not.toBeInTheDocument()
  })

  it('calls onAdd with url on submit', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: true })
    const { container } = renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=123' } })
    fireEvent.submit(container.querySelector('form')!)

    expect(onAdd).toHaveBeenCalledWith('https://youtube.com/watch?v=123')
  })

  it('clears input on success and shows toast', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: true })
    const { container } = renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=123' } })
    fireEvent.submit(container.querySelector('form')!)

    expect(await screen.findByText('Added!')).toBeInTheDocument()
    expect(input).toHaveValue('')
  })

  it('shows error toast on failure', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: false, error: 'Failed to add track' })
    const { container } = renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=bad' } })
    fireEvent.submit(container.querySelector('form')!)

    expect(await screen.findByText('Failed to add track')).toBeInTheDocument()
  })

  it('does not submit empty url', () => {
    const onAdd = vi.fn()
    const { container } = renderWithToast(<AddTrack onAdd={onAdd} />)
    fireEvent.submit(container.querySelector('form')!)
    expect(onAdd).not.toHaveBeenCalled()
  })

  it('submits on Enter key via form', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: true })
    const { container } = renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=123' } })
    fireEvent.submit(container.querySelector('form')!)

    expect(onAdd).toHaveBeenCalledWith('https://youtube.com/watch?v=123')
  })

  it('disables input while adding', async () => {
    let resolveAdd: (v: { success: boolean }) => void
    const onAdd = vi.fn().mockImplementation(() => new Promise((r) => { resolveAdd = r }))
    const { container } = renderWithToast(<AddTrack onAdd={onAdd} />)

    const input = screen.getByPlaceholderText('Search')
    fireEvent.change(input, { target: { value: 'https://youtube.com/watch?v=123' } })
    fireEvent.submit(container.querySelector('form')!)

    await waitFor(() => {
      expect(input).toBeDisabled()
    })

    resolveAdd!({ success: true })
    await waitFor(() => {
      expect(input).not.toBeDisabled()
    })
  })

  it('shows clear button when input has text', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    const input = screen.getByPlaceholderText('Search')

    expect(screen.queryByRole('button', { name: '×' })).not.toBeInTheDocument()

    fireEvent.change(input, { target: { value: 'test' } })
    expect(screen.getByText('×')).toBeInTheDocument()
  })

  it('clears input when clear button is clicked', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    const input = screen.getByPlaceholderText('Search')

    fireEvent.change(input, { target: { value: 'test query' } })
    fireEvent.click(screen.getByText('×'))

    expect(input).toHaveValue('')
  })

  it('shows searching spinner during search', async () => {
    vi.useFakeTimers()
    let resolveFetch: (v: unknown) => void
    globalThis.fetch = vi.fn().mockImplementation(() =>
      new Promise((r) => { resolveFetch = r })
    )

    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    const input = screen.getByPlaceholderText('Search')

    fireEvent.change(input, { target: { value: 'rock music' } })
    await vi.advanceTimersByTimeAsync(400)

    expect(document.querySelector('.searchSpinner')).toBeInTheDocument()

    resolveFetch!({ json: () => [] })
    await vi.advanceTimersByTimeAsync(0)
    vi.useRealTimers()
  })

  it('shows no results message when search returns empty', async () => {
    vi.useFakeTimers()
    globalThis.fetch = vi.fn().mockResolvedValue({ json: () => Promise.resolve([]) })

    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    const input = screen.getByPlaceholderText('Search')

    fireEvent.change(input, { target: { value: 'xyznonexistent' } })
    await vi.advanceTimersByTimeAsync(500)

    expect(screen.getByText('No results found')).toBeInTheDocument()
    vi.useRealTimers()
  })

  it('shows the Filters label above the source chips', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} onAddById={vi.fn()} />)
    expect(screen.getByText('Filters')).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'Search source filter' })).toBeInTheDocument()
  })

  it('stays YouTube-only without onAddById', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    expect(screen.queryByRole('group', { name: 'Search source filter' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'bebradio' })).not.toBeInTheDocument()
  })

  it('shows YouTube and bebradio hits in one list with badges', async () => {
    const ytHit = {
      id: 'y1', title: 'Tube Song', artist: 'Channel', thumbnail: '',
      duration: 200, url: 'https://youtube.com/watch?v=y1',
    }
    const mashup = {
      id: 'm1', title: 'Bootleg', artist: 'DJ A', thumbnail: '',
      duration: 120, url: '/api/tracks/m1/audio', status: 'ready',
    }
    globalThis.fetch = vi.fn().mockImplementation((url: string) => {
      if (String(url).includes('/api/search')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([ytHit]) })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve([mashup]) })
    })
    renderWithToast(<AddTrack onAdd={vi.fn()} onAddById={vi.fn()} />)

    expect(screen.getByRole('button', { name: 'All' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.change(screen.getByPlaceholderText('Search'), {
      target: { value: 'boot' },
    })

    expect(await screen.findByText('Tube Song')).toBeInTheDocument()
    expect(screen.getByText('Bootleg')).toBeInTheDocument()
    const dropdown = document.querySelector('div[class*="searchDropdown"]') as HTMLElement
    const rows = within(dropdown).getAllByText(/Tube Song|Bootleg/)
    expect(rows).toHaveLength(2)
    expect(within(dropdown).getByText('YouTube')).toBeInTheDocument()
    expect(within(dropdown).getByText('bebradio')).toBeInTheDocument()
  })

  it('source filters narrow the unified list', async () => {
    const ytHit = {
      id: 'y1', title: 'Tube Song', artist: 'Channel', thumbnail: '',
      duration: 200, url: 'https://youtube.com/watch?v=y1',
    }
    const mashup = {
      id: 'm1', title: 'Bootleg', artist: 'DJ A', thumbnail: '',
      duration: 120, url: '/api/tracks/m1/audio', status: 'ready',
    }
    globalThis.fetch = vi.fn().mockImplementation((url: string) => {
      if (String(url).includes('/api/search')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([ytHit]) })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve([mashup]) })
    })
    renderWithToast(<AddTrack onAdd={vi.fn()} onAddById={vi.fn()} />)

    fireEvent.change(screen.getByPlaceholderText('Search'), {
      target: { value: 'boot' },
    })
    expect(await screen.findByText('Bootleg')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'YouTube' }))
    expect(screen.queryByText('Bootleg')).not.toBeInTheDocument()
    expect(screen.getByText('Tube Song')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'bebradio' }))
    expect(screen.queryByText('Tube Song')).not.toBeInTheDocument()
    expect(screen.getByText('Bootleg')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'All' }))
    expect(screen.getByText('Tube Song')).toBeInTheDocument()
    expect(screen.getByText('Bootleg')).toBeInTheDocument()
  })

  it('adds a mashup by id from the unified list', async () => {
    const mashup = {
      id: 'm1', title: 'Bootleg', artist: 'DJ A', thumbnail: '',
      duration: 120, url: '/api/tracks/m1/audio', status: 'ready',
    }
    globalThis.fetch = vi.fn().mockImplementation((url: string) => {
      if (String(url).includes('/api/search')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve([mashup]) })
    })
    const onAddById = vi.fn().mockResolvedValue({ success: true })
    renderWithToast(<AddTrack onAdd={vi.fn()} onAddById={onAddById} />)

    fireEvent.change(screen.getByPlaceholderText('Search'), {
      target: { value: 'boot' },
    })
    expect(await screen.findByText('Bootleg')).toBeInTheDocument()
    const addDropdown = document.querySelector('div[class*="searchDropdown"]') as HTMLElement
    fireEvent.click(within(addDropdown).getByRole('button', { name: 'Add' }))
    await waitFor(() => {
      expect(onAddById).toHaveBeenCalledWith('m1')
    })
  })

  it('disables Add for mashups that are not ready', async () => {
    const mashup = {
      id: 'm2', title: 'Raw Take', artist: 'DJ B', thumbnail: '',
      duration: 0, url: '', status: 'processing',
    }
    globalThis.fetch = vi.fn().mockImplementation((url: string) => {
      if (String(url).includes('/api/search')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve([]) })
      }
      return Promise.resolve({ ok: true, json: () => Promise.resolve([mashup]) })
    })
    renderWithToast(<AddTrack onAdd={vi.fn()} onAddById={vi.fn()} />)

    fireEvent.change(screen.getByPlaceholderText('Search'), {
      target: { value: 'raw' },
    })
    expect(await screen.findByText('Raw Take')).toBeInTheDocument()
    const disabledDropdown = document.querySelector('div[class*="searchDropdown"]') as HTMLElement
    expect(within(disabledDropdown).getByRole('button', { name: 'Add' })).toBeDisabled()
  })
})
