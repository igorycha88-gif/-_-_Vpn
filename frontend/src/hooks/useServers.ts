import { useQuery } from '@tanstack/react-query'
import * as serversApi from '../api/servers'

export function useServersStatus() {
  return useQuery({
    queryKey: ['servers', 'status'],
    queryFn: () => serversApi.getServersStatus(),
    refetchInterval: 15000,
  })
}

export function useRuStats() {
  return useQuery({
    queryKey: ['servers', 'ru'],
    queryFn: () => serversApi.getRuStats(),
    refetchInterval: 15000,
  })
}

export function useForeignStats() {
  return useQuery({
    queryKey: ['servers', 'foreign'],
    queryFn: () => serversApi.getForeignStats(),
    refetchInterval: 15000,
  })
}
