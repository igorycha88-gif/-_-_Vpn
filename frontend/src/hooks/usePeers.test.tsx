import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { usePeers, useCreatePeer, useDeletePeer, useTogglePeer } from './usePeers'
import * as peersApi from '../api/peers'
import type { Peer } from '../types'

vi.mock('../api/peers')
vi.mock('antd', async () => {
  const actual = await vi.importActual<typeof import('antd')>('antd')
  return actual
})

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

const mockPeer: Peer = {
  id: 'peer-1', name: 'Test Peer', device_type: 'iphone',
  public_key: 'uuid-1', private_key: 'pk', address: '10.0.0.2',
  dns: '1.1.1.1', mtu: 1280, is_active: true,
  created_at: '2024-01-01', updated_at: '2024-01-01',
  total_rx: 0, total_tx: 0,
}

describe('usePeers', () => {
  beforeEach(() => vi.clearAllMocks())

  it('fetches peers list', async () => {
    vi.mocked(peersApi.listPeers).mockResolvedValue([mockPeer])
    const { result } = renderHook(() => usePeers(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.data).toBeDefined())
    expect(result.current.data).toEqual([mockPeer])
  })
})

describe('useCreatePeer', () => {
  beforeEach(() => vi.clearAllMocks())

  it('creates peer and invalidates list', async () => {
    vi.mocked(peersApi.createPeer).mockResolvedValue(mockPeer)
    vi.mocked(peersApi.listPeers).mockResolvedValue([])

    const { result } = renderHook(() => useCreatePeer(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.mutateAsync({ name: 'Test Peer', device_type: 'iphone' })
    })

    expect(peersApi.createPeer).toHaveBeenCalledWith({ name: 'Test Peer', device_type: 'iphone' })
  })
})

describe('useDeletePeer', () => {
  beforeEach(() => vi.clearAllMocks())

  it('deletes peer', async () => {
    vi.mocked(peersApi.deletePeer).mockResolvedValue(undefined)

    const { result } = renderHook(() => useDeletePeer(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.mutateAsync('peer-1')
    })

    expect(peersApi.deletePeer).toHaveBeenCalledWith('peer-1')
  })
})

describe('useTogglePeer', () => {
  beforeEach(() => vi.clearAllMocks())

  it('toggles peer active state', async () => {
    vi.mocked(peersApi.togglePeer).mockResolvedValue(undefined)

    const { result } = renderHook(() => useTogglePeer(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.mutateAsync({ id: 'peer-1', active: false })
    })

    expect(peersApi.togglePeer).toHaveBeenCalledWith('peer-1', false)
  })
})
