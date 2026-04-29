/* eslint-disable @typescript-eslint/no-explicit-any */
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, it, expect, vi } from 'vitest'
import Dashboard from './Dashboard'
import * as monitoringHook from '../hooks/useMonitoring'
import * as serversHook from '../hooks/useServers'

vi.mock('../hooks/useMonitoring', () => ({
  useMonitoringStats: vi.fn(),
}))

vi.mock('../hooks/useServers', () => ({
  useServersStatus: vi.fn(),
}))

function renderDashboard() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <Dashboard />
    </QueryClientProvider>,
  )
}

describe('Dashboard', () => {
  it('renders loading state', () => {
    vi.mocked(monitoringHook.useMonitoringStats).mockReturnValue({
      data: undefined, isLoading: true, error: null, isError: false,
    } as any)
    vi.mocked(serversHook.useServersStatus).mockReturnValue({
      data: undefined, isLoading: true, error: null, isError: false,
    } as any)
    renderDashboard()
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
  })

  it('renders stats when data is loaded', () => {
    vi.mocked(monitoringHook.useMonitoringStats).mockReturnValue({
      data: { total_peers: 10, online_peers: 3, total_rx: 1073741824, total_tx: 536870912, active_peers: 8, rules_count: 15 },
      isLoading: false, error: null, isError: false,
    } as any)
    vi.mocked(serversHook.useServersStatus).mockReturnValue({
      data: { ru: { online: true }, foreign: { online: false } },
      isLoading: false, error: null, isError: false,
    } as any)
    renderDashboard()
    expect(screen.getByText('10')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('15')).toBeInTheDocument()
  })

  it('renders error state for stats', () => {
    vi.mocked(monitoringHook.useMonitoringStats).mockReturnValue({
      data: undefined, isLoading: false, error: new Error('fail'), isError: true,
    } as any)
    vi.mocked(serversHook.useServersStatus).mockReturnValue({
      data: undefined, isLoading: false, error: null, isError: false,
    } as any)
    renderDashboard()
    expect(screen.getByText(/ошибк/i)).toBeInTheDocument()
  })
})
