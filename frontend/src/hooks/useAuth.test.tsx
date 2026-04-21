import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useLogin, useLogout, useSession } from './useAuth'
import * as authApi from '../api/auth'
import * as authStore from '../store/auth'

vi.mock('../api/auth')
vi.mock('../store/auth')

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    )
  }
}

describe('useLogin', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls login API and stores tokens on success', async () => {
    const mockTokens = {
      access_token: 'access-123',
      refresh_token: 'refresh-456',
      expires_in: 900,
    }
    vi.mocked(authApi.login).mockResolvedValue(mockTokens)
    vi.mocked(authStore.setTokens).mockImplementation(() => {})

    const { result } = renderHook(() => useLogin(), {
      wrapper: createWrapper(),
    })

    await act(async () => {
      await result.current.mutateAsync({ email: 'admin@test.com', password: 'pass123' })
    })

    expect(authApi.login).toHaveBeenCalledWith({ email: 'admin@test.com', password: 'pass123' })
    expect(authStore.setTokens).toHaveBeenCalledWith('access-123', 'refresh-456')
  })

  it('does not store tokens on failure', async () => {
    vi.mocked(authApi.login).mockRejectedValue(new Error('Unauthorized'))

    const { result } = renderHook(() => useLogin(), {
      wrapper: createWrapper(),
    })

    await act(async () => {
      try {
        await result.current.mutateAsync({ email: 'admin@test.com', password: 'wrong' })
      } catch {
        // expected error
      }
    })

    expect(authStore.setTokens).not.toHaveBeenCalled()
  })
})

describe('useLogout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('calls logout API with refresh token', async () => {
    vi.mocked(authStore.getRefreshToken).mockReturnValue('refresh-456')
    vi.mocked(authApi.logout).mockResolvedValue(undefined)
    vi.mocked(authStore.clearTokens).mockImplementation(() => {})

    const { result } = renderHook(() => useLogout(), {
      wrapper: createWrapper(),
    })

    await act(async () => {
      await result.current.mutateAsync()
    })

    expect(authApi.logout).toHaveBeenCalledWith('refresh-456')
    expect(authStore.clearTokens).toHaveBeenCalled()
  })

  it('skips API call when no refresh token', async () => {
    vi.mocked(authStore.getRefreshToken).mockReturnValue(null)
    vi.mocked(authStore.clearTokens).mockImplementation(() => {})

    const { result } = renderHook(() => useLogout(), {
      wrapper: createWrapper(),
    })

    await act(async () => {
      await result.current.mutateAsync()
    })

    expect(authApi.logout).not.toHaveBeenCalled()
    expect(authStore.clearTokens).toHaveBeenCalled()
  })
})

describe('useSession', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('does not fetch session when no refresh token', () => {
    vi.mocked(authStore.getRefreshToken).mockReturnValue(null)

    renderHook(() => useSession(), {
      wrapper: createWrapper(),
    })

    expect(authApi.getSession).not.toHaveBeenCalled()
  })

  it('fetches session when refresh token exists', async () => {
    vi.mocked(authStore.getRefreshToken).mockReturnValue('refresh-456')
    vi.mocked(authApi.getSession).mockResolvedValue({
      user_id: '1',
      email: 'admin@test.com',
      role: 'admin',
    })

    renderHook(() => useSession(), {
      wrapper: createWrapper(),
    })

    await waitFor(() => {
      expect(authApi.getSession).toHaveBeenCalled()
    })
  })
})
