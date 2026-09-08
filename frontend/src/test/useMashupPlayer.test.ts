import { createElement } from 'react'
import { render, act } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import { useMashupPlayer, type MashupPlayer } from '../hooks/useMashupPlayer'
import type { Mashup } from '../types'

function m(id: string): Mashup {
  return {
    id,
    owner_id: 'o1',
    title: `Track ${id}`,
    artist: 'DJ Test',
    duration: 120,
    size_bytes: 0,
    status: 'ready',
    has_cover: false,
    plays: 0,
    likes: 0,
    created_at: '2026-01-01T00:00:00Z',
    stream_url: `/api/mashups/media/${id}`,
  }
}

/** Mounts the hook inside a component that provides a real <audio> element. */
function setup() {
  const ref: { current: MashupPlayer | null } = { current: null }
  function Harness() {
    const player = useMashupPlayer()
    ref.current = player
    return createElement('audio', { ref: player.audioRef })
  }
  render(createElement(Harness))
  return ref as { current: MashupPlayer }
}

describe('useMashupPlayer', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('starts with shuffle off, repeat off, no buffer', () => {
    const p = setup()
    expect(p.current.shuffle).toBe(false)
    expect(p.current.repeat).toBe('off')
    expect(p.current.buffered).toBe(0)
  })

  it('cycleRepeat walks off -> all -> one -> off', () => {
    const p = setup()
    act(() => p.current.cycleRepeat())
    expect(p.current.repeat).toBe('all')
    act(() => p.current.cycleRepeat())
    expect(p.current.repeat).toBe('one')
    act(() => p.current.cycleRepeat())
    expect(p.current.repeat).toBe('off')
  })

  it('next() stops on the last track when repeat is off', () => {
    const p = setup()
    act(() => p.current.setList([m('a'), m('b'), m('c')]))
    act(() => p.current.playAt(2))
    act(() => p.current.next())
    expect(p.current.index).toBe(2)
  })

  it('next() wraps to the first track when repeat is "all"', () => {
    const p = setup()
    act(() => p.current.setList([m('a'), m('b'), m('c')]))
    act(() => p.current.playAt(2))
    act(() => p.current.cycleRepeat()) // -> all
    act(() => p.current.next())
    expect(p.current.index).toBe(0)
    expect(p.current.current?.id).toBe('a')
  })

  it('onEnded with repeat "one" rewinds the current track instead of advancing', () => {
    const p = setup()
    act(() => p.current.setList([m('a'), m('b')]))
    act(() => p.current.playAt(0))
    act(() => p.current.cycleRepeat()) // off -> all
    act(() => p.current.cycleRepeat()) // all -> one

    const el = p.current.audioRef.current!
    el.currentTime = 42
    act(() => {
      el.dispatchEvent(new Event('ended'))
    })

    expect(el.currentTime).toBe(0)
    expect(p.current.index).toBe(0)
  })

  it('shuffle walks every track exactly once before it can repeat', () => {
    const p = setup()
    act(() => p.current.setList([m('a'), m('b'), m('c'), m('d')]))
    act(() => p.current.playAt(0))
    act(() => p.current.toggleShuffle())

    const seen = [p.current.current!.id]
    for (let k = 0; k < 3; k++) {
      act(() => p.current.next())
      seen.push(p.current.current!.id)
    }
    expect(new Set(seen).size).toBe(4)
  })

  it('rebuilds the walk order when the list changes, keeping the current track', () => {
    const p = setup()
    act(() => p.current.setList([m('a'), m('b'), m('c')]))
    act(() => p.current.playAt(1))
    act(() => p.current.setList([m('a'), m('b'), m('c'), m('d'), m('e')]))
    expect(p.current.current?.id).toBe('b')
    act(() => p.current.next())
    expect(p.current.current?.id).toBe('c')
  })

  // ── reload persistence ────────────────────────────────────────────
  const KEY = 'mashup-player'

  it('writes a snapshot to localStorage once something is playing', () => {
    const p = setup()
    act(() => p.current.setList([m('a'), m('b')]))
    act(() => p.current.playAt(1))
    const saved = JSON.parse(localStorage.getItem(KEY)!)
    expect(saved.id).toBe('b')
    expect(saved.repeat).toBe('off')
    expect(saved.shuffle).toBe(false)
  })

  it('restores shuffle and repeat saved from a previous session', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ id: 'x', position: 0, playing: false, shuffle: true, repeat: 'all' }),
    )
    const p = setup()
    expect(p.current.shuffle).toBe(true)
    expect(p.current.repeat).toBe('all')
  })

  it('restores the saved track and offset once the list arrives', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ id: 'b', position: 33, playing: false, shuffle: false, repeat: 'off' }),
    )
    const p = setup()
    expect(p.current.index).toBe(-1) // nothing to restore onto yet
    expect(localStorage.getItem(KEY)).not.toBeNull() // and the snapshot survives the wait

    act(() => p.current.setList([m('a'), m('b'), m('c')]))
    expect(p.current.index).toBe(1)
    expect(p.current.current?.id).toBe('b')

    const el = p.current.audioRef.current!
    act(() => el.dispatchEvent(new Event('loadedmetadata')))
    expect(el.currentTime).toBe(33)
    expect(p.current.isPlaying).toBe(false)
  })

  it('ignores a saved track that is no longer in the list', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ id: 'gone', position: 10, playing: false, shuffle: false, repeat: 'off' }),
    )
    const p = setup()
    act(() => p.current.setList([m('a'), m('b')]))
    expect(p.current.index).toBe(-1)
    act(() => p.current.playAt(0))
    expect(p.current.current?.id).toBe('a')
  })
})
