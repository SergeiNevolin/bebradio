import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

vi.mock('../context/PlayerContext', () => ({
  usePlayer: () => ({ player: { pause: vi.fn() } }),
}))
vi.mock('../karaoke/App', () => ({ default: () => <div>KARAOKE_APP</div> }))
vi.mock('../karaoke/lib/theme', () => ({ applyAccent: vi.fn(), getStoredAccent: () => '' }))

import KaraokePage from '../pages/KaraokePage'

describe('KaraokePage', () => {
  it('монтирует караоке нативно на всю область под навбаром, без iframe', () => {
    const { container } = render(<KaraokePage />)
    expect(screen.getByText('KARAOKE_APP')).toBeTruthy()
    expect(container.querySelector('iframe')).toBeNull()
    expect(container.querySelector('.karaoke-root')).not.toBeNull()
  })
})
