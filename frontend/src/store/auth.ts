const TOKEN_KEY = 'smarttraffic_tokens'

export function getTokens(): { accessToken: string; refreshToken: string } | null {
  const raw = localStorage.getItem(TOKEN_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw)
  } catch {
    return null
  }
}

export function setTokens(accessToken: string, refreshToken: string): void {
  localStorage.setItem(TOKEN_KEY, JSON.stringify({ accessToken, refreshToken }))
}

export function clearTokens(): void {
  localStorage.removeItem(TOKEN_KEY)
}

export function getAccessToken(): string | null {
  const tokens = getTokens()
  return tokens?.accessToken ?? null
}

export function getRefreshToken(): string | null {
  const tokens = getTokens()
  return tokens?.refreshToken ?? null
}
