import { render } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import ScrollRow from '../components/ScrollRow'

function trackEl(container: HTMLElement): HTMLDivElement {
  const el = container.querySelector('.scrollRowTrack') as HTMLDivElement
  // jsdom без раскладки: задаём геометрию вручную
  Object.defineProperties(el, {
    scrollWidth: { value: 1000, configurable: true },
    clientWidth: { value: 200, configurable: true },
  })
  return el
}

function wheel(deltaY: number): WheelEvent {
  return new WheelEvent('wheel', { deltaY, deltaX: 0, cancelable: true, bubbles: true })
}

describe('ScrollRow wheel', () => {
  it('крутит рейл и гасит событие, страница не едет', () => {
    const { container } = render(
      <ScrollRow>
        <span>card</span>
      </ScrollRow>,
    )
    const el = trackEl(container)
    const ev = wheel(100)
    el.dispatchEvent(ev)
    expect(ev.defaultPrevented).toBe(true)
    expect(el.scrollLeft).toBe(100)
  })

  it('на краях событие не гасит — крутится страница', () => {
    const { container } = render(
      <ScrollRow>
        <span>card</span>
      </ScrollRow>,
    )
    const el = trackEl(container)
    // правый край: вниз больше некуда
    el.scrollLeft = 800
    const down = wheel(100)
    el.dispatchEvent(down)
    expect(down.defaultPrevented).toBe(false)
    // левый край: вверх больше некуда
    el.scrollLeft = 0
    const up = wheel(-100)
    el.dispatchEvent(up)
    expect(up.defaultPrevented).toBe(false)
  })
})
