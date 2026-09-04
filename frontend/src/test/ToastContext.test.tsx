import { render, screen, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ToastProvider, useToast } from '../context/ToastContext'

function TestConsumer() {
  const { showToast } = useToast()
  return (
    <div>
      <button onClick={() => showToast('it works')}>success</button>
      <button onClick={() => showToast('oops', 'error')}>error</button>
    </div>
  )
}

describe('ToastContext', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders toast on showToast', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    )
    act(() => screen.getByText('success').click())
    expect(screen.getByText('it works')).toBeInTheDocument()
  })

  it('renders error toast with correct class', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    )
    act(() => screen.getByText('error').click())
    const toast = screen.getByText('oops')
    expect(toast).toHaveClass('toast-error')
  })

  it('auto-dismisses after 3 seconds', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    )
    act(() => screen.getByText('success').click())
    expect(screen.getByText('it works')).toBeInTheDocument()
    act(() => vi.advanceTimersByTime(3000))
    expect(screen.queryByText('it works')).not.toBeInTheDocument()
  })

  it('renders success toast with correct class', () => {
    render(
      <ToastProvider>
        <TestConsumer />
      </ToastProvider>
    )
    act(() => screen.getByText('success').click())
    const toast = screen.getByText('it works')
    expect(toast).toHaveClass('toast-success')
  })
})
