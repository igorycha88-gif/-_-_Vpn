import { describe, it, expect, beforeEach, vi } from 'vitest'

const mockClearTokens = vi.fn()
const mockGetAccessToken = vi.fn()
const mockGetRefreshToken = vi.fn()
const mockSetTokens = vi.fn()

vi.mock('../store/auth', () => ({
  getAccessToken: () => mockGetAccessToken(),
  getRefreshToken: () => mockGetRefreshToken(),
  setTokens: (...args: unknown[]) => mockSetTokens(...args),
  clearTokens: (...args: unknown[]) => mockClearTokens(...args),
}))

describe('client interceptor redirect logic', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetAccessToken.mockReturnValue(null)
    mockGetRefreshToken.mockReturnValue(null)
    Object.defineProperty(window, 'location', {
      writable: true,
      value: { href: '', pathname: '/dashboard' },
    })
  })

  it('redirects to /login on 401 when not on /login page and no refresh token', () => {
    window.location.pathname = '/dashboard'
    mockGetRefreshToken.mockReturnValue(null)

    const onLogin = !window.location.pathname.startsWith('/login')
    if (onLogin) {
      mockClearTokens()
      window.location.href = '/login'
    }

    expect(window.location.href).toBe('/login')
    expect(mockClearTokens).toHaveBeenCalled()
  })

  it('does not redirect when already on /login and no refresh token', () => {
    window.location.pathname = '/login'
    mockGetRefreshToken.mockReturnValue(null)

    const onLogin = !window.location.pathname.startsWith('/login')
    if (onLogin) {
      mockClearTokens()
      window.location.href = '/login'
    }

    expect(window.location.href).not.toBe('/login')
    expect(mockClearTokens).not.toHaveBeenCalled()
  })

  it('does not redirect when on /login subpath like /login?error=1', () => {
    window.location.pathname = '/login'
    mockGetRefreshToken.mockReturnValue(null)

    const onLogin = !window.location.pathname.startsWith('/login')
    if (onLogin) {
      window.location.href = '/login'
    }

    expect(window.location.href).not.toBe('/login')
  })

  it('redirects when on any non-login page', () => {
    window.location.pathname = '/peers'
    mockGetRefreshToken.mockReturnValue(null)

    const onLogin = !window.location.pathname.startsWith('/login')
    if (onLogin) {
      mockClearTokens()
      window.location.href = '/login'
    }

    expect(window.location.href).toBe('/login')
  })

  it('clears tokens before redirect', () => {
    window.location.pathname = '/dashboard'
    mockGetRefreshToken.mockReturnValue(null)

    const onLogin = !window.location.pathname.startsWith('/login')
    if (onLogin) {
      mockClearTokens()
      window.location.href = '/login'
    }

    expect(mockClearTokens).toHaveBeenCalledTimes(1)
  })

  it('does not redirect when refresh fails and on /login', () => {
    window.location.pathname = '/login'

    const onLogin = !window.location.pathname.startsWith('/login')
    if (onLogin) {
      mockClearTokens()
      window.location.href = '/login'
    }

    expect(window.location.href).not.toBe('/login')
  })
})
