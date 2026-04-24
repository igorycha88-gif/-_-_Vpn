import { useQuery } from '@tanstack/react-query'
import * as monitoringApi from '../api/monitoring'

export function useTrafficLogs(peerId?: string) {
  return useQuery({
    queryKey: ['monitoring', 'traffic', peerId],
    queryFn: () => monitoringApi.getTrafficLogs(peerId),
    refetchInterval: 10000,
  })
}

export function useTrafficAggregate(peerId?: string) {
  return useQuery({
    queryKey: ['monitoring', 'traffic-aggregate', peerId],
    queryFn: () => monitoringApi.getTrafficAggregate(peerId),
    refetchInterval: 10000,
  })
}

export function useRoutingLogs(peerId?: string) {
  return useQuery({
    queryKey: ['monitoring', 'logs', peerId],
    queryFn: () => monitoringApi.getRoutingLogs(peerId),
    refetchInterval: 10000,
  })
}

export function useAlerts() {
  return useQuery({
    queryKey: ['monitoring', 'alerts'],
    queryFn: () => monitoringApi.getAlerts(),
    refetchInterval: 30000,
  })
}

export function useMonitoringStats() {
  return useQuery({
    queryKey: ['monitoring', 'stats'],
    queryFn: () => monitoringApi.getMonitoringStats(),
    refetchInterval: 10000,
  })
}

export function usePeerMonitor(peerId: string | undefined) {
  return useQuery({
    queryKey: ['monitoring', 'peer', peerId],
    queryFn: () => monitoringApi.getPeerMonitor(peerId!),
    enabled: !!peerId,
    refetchInterval: 10000,
  })
}

export function usePeersStats() {
  return useQuery({
    queryKey: ['monitoring', 'peers-stats'],
    queryFn: () => monitoringApi.getPeersStats(),
    refetchInterval: 15000,
  })
}
