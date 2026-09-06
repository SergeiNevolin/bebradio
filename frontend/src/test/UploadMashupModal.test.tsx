import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import UploadMashupModal from '../components/mashup/UploadMashupModal'

const { uploadMashup } = vi.hoisted(() => ({ uploadMashup: vi.fn() }))
vi.mock('../lib/api', () => ({ api: { uploadMashup } }))

const showToast = vi.fn()
vi.mock('../context/ToastContext', () => ({ useToast: () => ({ showToast }) }))

function selectFile(name = 'mix.mp3') {
  const input = screen.getByLabelText('Audio file') as HTMLInputElement
  const file = new File(['audio-bytes'], name, { type: 'audio/mpeg' })
  fireEvent.change(input, { target: { files: [file] } })
  return file
}

describe('UploadMashupModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('prefills the title from the file name', () => {
    render(<UploadMashupModal onClose={vi.fn()} onUploaded={vi.fn()} />)
    selectFile('Summer Bootleg.mp3')
    expect(screen.getByLabelText('Title')).toHaveValue('Summer Bootleg')
  })

  it('uploads the file with title and artist, then closes', async () => {
    uploadMashup.mockResolvedValue({ id: 'm1', status: 'processing' })
    const onClose = vi.fn()
    const onUploaded = vi.fn()
    render(<UploadMashupModal onClose={onClose} onUploaded={onUploaded} />)

    const file = selectFile()
    fireEvent.change(screen.getByLabelText('Title'), { target: { value: 'My Mix' } })
    fireEvent.change(screen.getByLabelText('Artist'), { target: { value: 'Me' } })
    fireEvent.click(screen.getByRole('button', { name: 'Upload' }))

    await waitFor(() => expect(uploadMashup).toHaveBeenCalled())
    expect(uploadMashup.mock.calls[0][0]).toEqual({ file, title: 'My Mix', artist: 'Me' })
    await waitFor(() => expect(onUploaded).toHaveBeenCalledWith({ id: 'm1', status: 'processing' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('shows an error and stays open when the upload fails', async () => {
    uploadMashup.mockRejectedValue(new Error('Upload quota reached'))
    const onClose = vi.fn()
    render(<UploadMashupModal onClose={onClose} onUploaded={vi.fn()} />)

    selectFile()
    fireEvent.click(screen.getByRole('button', { name: 'Upload' }))

    expect(await screen.findByText('Upload quota reached')).toBeInTheDocument()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('will not submit without a file', () => {
    render(<UploadMashupModal onClose={vi.fn()} onUploaded={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Upload' })).toBeDisabled()
  })
})
