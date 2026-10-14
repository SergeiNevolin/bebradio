// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import SingingCat, {
  CAT_LOUD_SRC,
  CAT_SILENT_SRC,
  CAT_SOFT_SRC,
} from './SingingCat'
import type { CatMode } from './SingingCat'

function visibleSrc(mode: CatMode): string | null {
  const { container } = render(<SingingCat mode={mode} />)
  const imgs = container.querySelectorAll('img')
  expect(imgs).toHaveLength(3)
  // активна ровно одна: без полупрозрачных состояний (иначе просвечивает фон)
  const visible = [...imgs].filter((img) => !img.className.includes('hidden'))
  expect(visible).toHaveLength(1)
  return visible[0].getAttribute('src')
}

describe('SingingCat', () => {
  it('молчание: виден cat-silent', () => {
    expect(visibleSrc('silent')).toBe(CAT_SILENT_SRC)
  })

  it('тихое пение: виден cat-soft-sing', () => {
    expect(visibleSrc('soft')).toBe(CAT_SOFT_SRC)
  })

  it('громкое пение: виден cat-loud-sing', () => {
    expect(visibleSrc('loud')).toBe(CAT_LOUD_SRC)
  })

  it('декоративный: правый нижний угол, не перехватывает клики', () => {
    const { container } = render(<SingingCat mode="silent" />)
    const box = container.firstElementChild
    expect(box?.getAttribute('aria-hidden')).toBe('true')
    expect(box?.className).toContain('pointer-events-none')
    expect(box?.className).toContain('absolute')
    expect(box?.className).toContain('bottom-3')
    expect(box?.className).toContain('right-3')
  })
})
