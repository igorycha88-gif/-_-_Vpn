import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter } from 'react-router-dom'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import Login from './Login'
import * as authApi from '../api/auth'

vi.mock('../api/auth')
vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd')
  return actual
})

function renderLogin() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Login />
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

describe('Login page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders login form', () => {
    renderLogin()
    expect(screen.getByPlaceholderText('Email')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Пароль')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /войти/i })).toBeInTheDocument()
  })

  it('shows title', () => {
    renderLogin()
    expect(screen.getByText('SmartTraffic')).toBeInTheDocument()
  })

  it('submits form with email and password', async () => {
    const user = userEvent.setup()
    vi.mocked(authApi.login).mockResolvedValue({
      access_token: 'token',
      refresh_token: 'refresh',
      expires_in: 900,
    })

    renderLogin()

    await user.type(screen.getByPlaceholderText('Email'), 'admin@smarttraffic.local')
    await user.type(screen.getByPlaceholderText('Пароль'), 'admin123')
    await user.click(screen.getByRole('button', { name: /войти/i }))

    await waitFor(() => {
      expect(authApi.login).toHaveBeenCalledWith({
        email: 'admin@smarttraffic.local',
        password: 'admin123',
      })
    })
  })

  it('does not call useSession (no /auth/session request)', () => {
    vi.mocked(authApi.getSession).mockResolvedValue({
      user_id: '1',
      email: 'admin@test.com',
      role: 'admin',
    })

    renderLogin()

    expect(authApi.getSession).not.toHaveBeenCalled()
  })

  it('does not call useAuth hook (no session query)', () => {
    renderLogin()
    expect(authApi.getSession).not.toHaveBeenCalled()
    expect(authApi.login).not.toHaveBeenCalled()
  })
})
