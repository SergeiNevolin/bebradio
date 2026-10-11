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
  const noop = () => {}
  it('показывает песню и блокирует Сохранить без правок', () => {
    render(<EditKaraokeModal song={SONG} onSaveMeta={vi.fn()} onChangeCover={noop} onClose={vi.fn()} />)
    expect(screen.getByRole('dialog', { name: 'Настройки караоке' })).toBeInTheDocument()
    expect(screen.getByLabelText('Название песни')).toHaveValue('Старое')
    expect(screen.getByLabelText('Автор песни')).toHaveValue('Автор')
    expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()
  })

  it('сохраняет мету и показывает Сохранено', async () => {
    const onSaveMeta = vi.fn(async () => {})
    render(<EditKaraokeModal song={SONG} onSaveMeta={onSaveMeta} onChangeCover={noop} onClose={vi.fn()} />)
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
      <EditKaraokeModal song={{ ...SONG, artist: null }} onSaveMeta={onSaveMeta} onChangeCover={noop} onClose={vi.fn()} />,
    )
    fireEvent.change(screen.getByLabelText('Название песни'), { target: { value: '   ' } })
    expect(screen.getByRole('button', { name: 'Сохранить' })).toBeDisabled()
    fireEvent.change(screen.getByLabelText('Название песни'), { target: { value: 'Ок' } })
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }))
    await waitFor(() => {
      expect(screen.getByText('Чужое')).toBeInTheDocument()
    })
  })

  it('кнопка обложки зовёт onChangeCover с файлом', () => {
    const onChangeCover = vi.fn()
    const { container } = render(
      <EditKaraokeModal song={SONG} onSaveMeta={vi.fn()} onChangeCover={onChangeCover} onClose={vi.fn()} />,
    )
    expect(screen.getByRole('button', { name: 'Загрузить обложку' })).toBeInTheDocument()
    const file = new File(['bytes'], 'cover.png', { type: 'image/png' })
    const input = container.querySelector('input[type="file"]') as HTMLInputElement
    fireEvent.change(input, { target: { files: [file] } })
    expect(onChangeCover).toHaveBeenCalledTimes(1)
    expect(onChangeCover.mock.calls[0][0]).toBe(file)
  })

  it('с обложкой показывает картинку и предлагает замену', () => {
    render(
      <EditKaraokeModal
        song={{ ...SONG, cover: '/api/karaoke/songs/t/cover.jpg' }}
        onSaveMeta={vi.fn()}
        onChangeCover={() => {}}
        onClose={vi.fn()}
      />,
    )
    expect(screen.getByRole('button', { name: 'Заменить обложку' })).toBeInTheDocument()
  })
})
