import { describe, it, expect, beforeEach, vi } from 'vitest'
import { getTokens, setTokens, clearTokens, getAccessToken, getRefreshToken } from './auth'

class LocalStorageMock {
  private store: Record<string, string> = {}
  getItem(key: string) { return this.store[key] ?? null }
  setItem(key: string, value: string) { this.store[key] = value }
  removeItem(key: string) { delete this.store[key] }
  clear() { this.store = {} }
  get length() { return Object.keys(this.store).length }
  key(): string | null { return null }
}

describe('auth store', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', new LocalStorageMock())
  })

  it('returns null when no tokens stored', () => {
    expect(getTokens()).toBeNull()
    expect(getAccessToken()).toBeNull()
    expect(getRefreshToken()).toBeNull()
  })

  it('stores and retrieves tokens', () => {
    setTokens('access-123', 'refresh-456')
    expect(getAccessToken()).toBe('access-123')
    expect(getRefreshToken()).toBe('refresh-456')
    const tokens = getTokens()
    expect(tokens).toEqual({ accessToken: 'access-123', refreshToken: 'refresh-456' })
  })

  it('clears tokens', () => {
    setTokens('a', 'b')
    clearTokens()
    expect(getTokens()).toBeNull()
    expect(getAccessToken()).toBeNull()
    expect(getRefreshToken()).toBeNull()
  })

  it('handles corrupted localStorage data', () => {
    localStorage.setItem('smarttraffic_tokens', 'not-json')
    expect(getTokens()).toBeNull()
    expect(getAccessToken()).toBeNull()
    expect(getRefreshToken()).toBeNull()
  })

  it('overwrites previous tokens on setTokens', () => {
    setTokens('old-access', 'old-refresh')
    setTokens('new-access', 'new-refresh')
    expect(getAccessToken()).toBe('new-access')
    expect(getRefreshToken()).toBe('new-refresh')
  })
})
