import { render } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import * as icons from '../components/player/icons'

const components = Object.entries(icons).filter(
  ([name]) => name.endsWith('Icon'),
) as [string, (props: { size?: number }) => JSX.Element][]

describe('player icons', () => {
  it('exports the full transport / toggle / volume / octicon set', () => {
    const names = components.map(([n]) => n).sort()
    expect(names).toEqual(
      [
        'ChevronDownIcon',
        'ChevronUpIcon',
        'HeartFillIcon',
        'HeartIcon',
        'NextIcon',
        'PauseIcon',
        'PlayIcon',
        'PrevIcon',
        'QueueIcon',
        'RepeatIcon',
        'RepeatOneIcon',
        'ShuffleIcon',
        'VolumeHighIcon',
        'VolumeLowIcon',
        'VolumeMutedIcon',
      ].sort(),
    )
  })

  it.each(components)('%s renders an <svg> with a viewBox', (_name, Comp) => {
    const { container } = render(<Comp />)
    const svg = container.querySelector('svg')
    expect(svg).not.toBeNull()
    expect(svg).toHaveAttribute('viewBox')
    expect(svg).toHaveAttribute('fill', 'currentColor')
  })

  it('honours the size prop', () => {
    const { container } = render(<icons.PlayIcon size={28} />)
    const svg = container.querySelector('svg')!
    expect(svg.getAttribute('width')).toBe('28')
    expect(svg.getAttribute('height')).toBe('28')
  })
})
