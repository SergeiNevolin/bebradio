import { renderHook, act } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import { useVolume } from '../hooks/useVolume'

describe('useVolume', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns default volume 0.7', () => {
    const { result } = renderHook(() => useVolume())
    expect(result.current.volume).toBe(0.7)
    expect(result.current.muted).toBe(false)
  })

  it('reads stored volume from localStorage', () => {
    localStorage.setItem('player_volume', '0.3')
    const { result } = renderHook(() => useVolume())
    expect(result.current.volume).toBe(0.3)
  })

  it('setVolume clamps to 0-1', () => {
    const { result } = renderHook(() => useVolume())
    act(() => result.current.setVolume(1.5))
    expect(result.current.volume).toBe(1)
    act(() => result.current.setVolume(-0.5))
    expect(result.current.volume).toBe(0)
  })

  it('setVolume unmutes when value > 0', () => {
    const { result } = renderHook(() => useVolume())
    act(() => result.current.toggleMute())
    expect(result.current.muted).toBe(true)
    act(() => result.current.setVolume(0.5))
    expect(result.current.muted).toBe(false)
  })

  it('toggleMute toggles muted state', () => {
    const { result } = renderHook(() => useVolume())
    act(() => result.current.toggleMute())
    expect(result.current.muted).toBe(true)
    act(() => result.current.toggleMute())
    expect(result.current.muted).toBe(false)
  })

  it('persists volume to localStorage', () => {
    const { result } = renderHook(() => useVolume())
    act(() => result.current.setVolume(0.42))
    expect(localStorage.getItem('player_volume')).toBe('0.42')
  })

  it('persists muted state to localStorage', () => {
    const { result } = renderHook(() => useVolume())
    act(() => result.current.toggleMute())
    expect(localStorage.getItem('player_muted')).toBe('true')
  })
})
