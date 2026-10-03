import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import KaraokePage from '../pages/KaraokePage'

describe('KaraokePage', () => {
  it('открывает караоке в iframe на всю область под навбаром', () => {
    render(<KaraokePage />)
    expect(screen.getByTitle('Караоке')).toHaveAttribute('src', '/karaoke/')
  })
})
