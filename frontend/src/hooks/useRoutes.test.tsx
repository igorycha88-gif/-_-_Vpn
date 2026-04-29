import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useRoutes, useCreateRule, useUpdateRule, useDeleteRule, useReorderRules } from './useRoutes'
import * as routesApi from '../api/routes'

vi.mock('../api/routes')

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

const mockRule = {
  id: 'rule-1', name: 'YouTube', type: 'domain', pattern: 'youtube.com',
  action: 'proxy', priority: 1, is_active: true, created_at: '', updated_at: '',
}

describe('useRoutes', () => {
  beforeEach(() => vi.clearAllMocks())

  it('fetches routing rules', async () => {
    vi.mocked(routesApi.listRules).mockResolvedValue([mockRule])
    const { result } = renderHook(() => useRoutes(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.data).toBeDefined())
    expect(result.current.data).toEqual([mockRule])
  })
})

describe('useCreateRule', () => {
  beforeEach(() => vi.clearAllMocks())

  it('creates rule', async () => {
    vi.mocked(routesApi.createRule).mockResolvedValue(mockRule)
    const { result } = renderHook(() => useCreateRule(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync({ name: 'YouTube', type: 'domain', pattern: 'youtube.com', action: 'proxy' })
    })
    expect(routesApi.createRule).toHaveBeenCalled()
  })
})

describe('useUpdateRule', () => {
  beforeEach(() => vi.clearAllMocks())

  it('updates rule', async () => {
    vi.mocked(routesApi.updateRule).mockResolvedValue({ ...mockRule, name: 'Updated' })
    const { result } = renderHook(() => useUpdateRule(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync({ id: 'rule-1', data: { name: 'Updated' } })
    })
    expect(routesApi.updateRule).toHaveBeenCalledWith('rule-1', { name: 'Updated' })
  })
})

describe('useDeleteRule', () => {
  beforeEach(() => vi.clearAllMocks())

  it('deletes rule', async () => {
    vi.mocked(routesApi.deleteRule).mockResolvedValue(undefined)
    const { result } = renderHook(() => useDeleteRule(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync('rule-1')
    })
    expect(routesApi.deleteRule).toHaveBeenCalledWith('rule-1')
  })
})

describe('useReorderRules', () => {
  beforeEach(() => vi.clearAllMocks())

  it('reorders rules', async () => {
    vi.mocked(routesApi.reorderRules).mockResolvedValue(undefined)
    const { result } = renderHook(() => useReorderRules(), { wrapper: createWrapper() })
    await act(async () => {
      await result.current.mutateAsync({ ids: ['2', '1'] })
    })
    expect(routesApi.reorderRules).toHaveBeenCalledWith({ ids: ['2', '1'] })
  })
})
