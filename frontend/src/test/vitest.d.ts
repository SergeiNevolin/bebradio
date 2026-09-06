import 'vitest/globals'
import '@testing-library/jest-dom/vitest'

declare module 'vitest' {
  interface Assertion<T = unknown> {
    toBeInTheDocument(): T
    toHaveValue(value: string | number | string[]): T
    toHaveClass(className: string): T
  }
}
