// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import EditKaraokeModal from './EditKaraokeModal'

const SONG = {
  id: 't',
  title: 'Старое',
  audio: 'songs/t/minus.mp3',
  lines: 1,
  duration: 65,
  language: 'ru',
  artist: 'Автор',
}

afterEach(() => cleanup())

describe('EditKaraokeModal', () => {
  it('показывает песню и блокирует Сохранить без правок', () => {
    render(<EditKaraokeModal song={SONG} onSaveMeta={vi.fn()} onClose={vi.fn()} />)
    expect(screen.getByRole('dialog', { name: 'Настройки караоке' })).toBeInTheDocument()
    expect(screen.getByLabelText('Название песни')).toHaveValue('Старое')
    expect(screen.getByLabelText('Автор песни')).toHaveValue('Автор')
    expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()
  })

  it('сохраняет мету и показывает Сохранено', async () => {
    const onSaveMeta = vi.fn(async () => {})
    render(<EditKaraokeModal song={SONG} onSaveMeta={onSaveMeta} onClose={vi.fn()} />)
    fireEvent.change(screen.getByLabelText('Название песни'), { target: { value: 'Новое' } })
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => {
      expect(onSaveMeta).toHaveBeenCalledWith('Новое', 'Автор')
    })
    expect(screen.getByRole('button', { name: 'Сохранено' })).toBeInTheDocument()
  })

  it('пустое название не сохраняет, ошибку сервера показывает', async () => {
    const onSaveMeta = vi.fn(async () => {
      throw new Error('Чужое')
    })
    render(
      <EditKaraokeModal song={{ ...SONG, artist: null }} onSaveMeta={onSaveMeta} onClose={vi.fn()} />,
    )
    fireEvent.change(screen.getByLabelText('Название песни'), { target: { value: '   ' } })
    expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Название песни'), { target: { value: 'Ок' } })
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => {
      expect(screen.getByText('Чужое')).toBeInTheDocument()
    })
  })
})
