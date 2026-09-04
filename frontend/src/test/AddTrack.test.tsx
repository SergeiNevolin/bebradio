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

    expect(onAdd).toHaveBeenCalledWith('https://youtube.com/watch?v=123', 'youtube')
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

    expect(onAdd).toHaveBeenCalledWith('https://youtube.com/watch?v=123', 'youtube')
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
    globalThis.fetch = vi.fn().mockImplementation(() =>
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
    globalThis.fetch = vi.fn().mockResolvedValue({ json: () => Promise.resolve([]) })

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
  // --- Platform picker ---

  it('offers YouTube and VK, with YouTube selected by default', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'YouTube' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'VK' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('switches the placeholder when VK is picked', () => {
    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'VK' }))

    expect(screen.getByPlaceholderText('Search or paste VK URL...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'VK' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('searches the picked platform', async () => {
    vi.useFakeTimers()
    const fetchMock = vi.fn().mockResolvedValue({ json: () => Promise.resolve([]) })
    globalThis.fetch = fetchMock

    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'VK' }))
    fireEvent.change(screen.getByPlaceholderText('Search or paste VK URL...'), {
      target: { value: 'кино' },
    })
    await vi.advanceTimersByTimeAsync(500)

    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toMatchObject({
      query: 'кино',
      source: 'vk',
    })
    vi.useRealTimers()
  })

  it('re-runs the current query when the platform changes', async () => {
    vi.useFakeTimers()
    const fetchMock = vi.fn().mockResolvedValue({ json: () => Promise.resolve([]) })
    globalThis.fetch = fetchMock

    renderWithToast(<AddTrack onAdd={vi.fn()} />)
    fireEvent.change(screen.getByPlaceholderText('Search or paste YouTube URL...'), {
      target: { value: 'nirvana' },
    })
    await vi.advanceTimersByTimeAsync(500)
    expect(JSON.parse(fetchMock.mock.calls[0][1].body).source).toBe('youtube')

    fireEvent.click(screen.getByRole('button', { name: 'VK' }))
    await vi.advanceTimersByTimeAsync(500)

    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toMatchObject({
      query: 'nirvana',
      source: 'vk',
    })
    vi.useRealTimers()
  })

  it('adds a pasted URL under the picked platform', async () => {
    const onAdd = vi.fn().mockResolvedValue({ success: true })
    renderWithToast(<AddTrack onAdd={onAdd} />)

    fireEvent.click(screen.getByRole('button', { name: 'VK' }))
    fireEvent.change(screen.getByPlaceholderText('Search or paste VK URL...'), {
      target: { value: 'https://vk.com/audio-2001_78' },
    })
    fireEvent.click(screen.getByText('Add'))

    expect(onAdd).toHaveBeenCalledWith('https://vk.com/audio-2001_78', 'vk')
  })

  it('adds a search result under the platform it came from', async () => {
    vi.useFakeTimers()
    const onAdd = vi.fn().mockResolvedValue({ success: true })
    globalThis.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve([
        { id: '-1_2', title: 'Song', artist: 'Band', thumbnail: '', duration: 100,
          url: 'https://vk.com/audio-1_2', source: 'vk' },
      ]),
    })

    renderWithToast(<AddTrack onAdd={onAdd} />)
    fireEvent.click(screen.getByRole('button', { name: 'VK' }))
    fireEvent.change(screen.getByPlaceholderText('Search or paste VK URL...'), {
      target: { value: 'band' },
    })
    await vi.advanceTimersByTimeAsync(500)

    fireEvent.click(screen.getByText('Song'))
    await vi.advanceTimersByTimeAsync(0)

    expect(onAdd).toHaveBeenCalledWith('https://vk.com/audio-1_2', 'vk')
    vi.useRealTimers()
  })
})
