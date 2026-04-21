import client from './client'
import type { LoginRequest, TokenPair, SessionResponse, RefreshTokenRequest } from '../types'

export async function login(data: LoginRequest): Promise<TokenPair> {
  const res = await client.post<TokenPair>('/auth/login', data)
  return res.data
}

export async function refresh(data: RefreshTokenRequest): Promise<TokenPair> {
  const res = await client.post<TokenPair>('/auth/refresh', data)
  return res.data
}

export async function getSession(): Promise<SessionResponse> {
  const res = await client.get<SessionResponse>('/auth/session')
  return res.data
}

export async function logout(refreshToken: string): Promise<void> {
  await client.post('/auth/logout', { refresh_token: refreshToken })
}

export async function logoutAll(): Promise<void> {
  await client.post('/auth/logout-all')
}
