import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { message } from 'antd'
import * as peersApi from '../api/peers'
import type { PeerCreateRequest } from '../types'

export function usePeers() {
  return useQuery({
    queryKey: ['peers'],
    queryFn: () => peersApi.listPeers(),
  })
}

export function usePeer(id: string) {
  return useQuery({
    queryKey: ['peers', id],
    queryFn: () => peersApi.getPeer(id),
    enabled: !!id,
  })
}

export function useCreatePeer() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: PeerCreateRequest) => peersApi.createPeer(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['peers'] })
      message.success('Клиент создан')
    },
    onError: () => {
      message.error('Ошибка создания клиента')
    },
  })
}

export function useDeletePeer() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => peersApi.deletePeer(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['peers'] })
      message.success('Клиент удалён')
    },
    onError: () => {
      message.error('Ошибка удаления клиента')
    },
  })
}

export function useTogglePeer() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) =>
      peersApi.togglePeer(id, active),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['peers'] })
    },
    onError: () => {
      message.error('Ошибка переключения клиента. Возможно, sing-box не перезапущен.')
    },
  })
}
