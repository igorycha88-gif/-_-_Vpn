import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { message } from 'antd'
import * as authApi from '../api/auth'
import { clearTokens, getRefreshToken, setTokens } from '../store/auth'
import type { LoginRequest } from '../types'

export function useLogin() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: LoginRequest) => authApi.login(data),
    onSuccess: (data) => {
      setTokens(data.access_token, data.refresh_token)
      queryClient.invalidateQueries({ queryKey: ['session'] })
    },
  })
}

export function useLogout() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => {
      const refreshToken = getRefreshToken()
      if (refreshToken) {
        return authApi.logout(refreshToken)
      }
      return Promise.resolve()
    },
    onSettled: () => {
      clearTokens()
      queryClient.clear()
    },
  })
}

export function useSession() {
  return useQuery({
    queryKey: ['session'],
    queryFn: () => authApi.getSession(),
    retry: false,
    refetchOnWindowFocus: false,
    enabled: !!getRefreshToken(),
  })
}

export function useAuth() {
  const loginMutation = useLogin()
  const logoutMutation = useLogout()
  const sessionQuery = useSession()

  const handleLogin = async (data: LoginRequest) => {
    try {
      await loginMutation.mutateAsync(data)
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } } }
      message.error(err.response?.data?.error || 'Ошибка входа')
      throw error
    }
  }

  const handleLogout = async () => {
    await logoutMutation.mutateAsync()
  }

  return {
    isAuthenticated: sessionQuery.isSuccess,
    session: sessionQuery.data,
    isLoading: sessionQuery.isLoading,
    login: handleLogin,
    logout: handleLogout,
  }
}
