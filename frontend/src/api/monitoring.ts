import client from './client'
import type { TrafficLog, TotalStats, Alert, Peer } from '../types'

export async function getTrafficLogs(peerId?: string): Promise<TrafficLog[]> {
  const params = peerId ? { peer_id: peerId } : {}
  const res = await client.get<TrafficLog[]>('/monitoring/traffic', { params })
  return res.data
}

export async function getRoutingLogs(peerId?: string): Promise<TrafficLog[]> {
  const params = peerId ? { peer_id: peerId } : {}
  const res = await client.get<TrafficLog[]>('/monitoring/logs', { params })
  return res.data
}

export async function getAlerts(): Promise<Alert[]> {
  const res = await client.get<Alert[]>('/monitoring/alerts')
  return res.data
}

export async function getMonitoringStats(): Promise<TotalStats> {
  const res = await client.get<TotalStats>('/monitoring/stats')
  return res.data
}

export interface PeerMonitorResponse {
  peer: Peer
  traffic_logs: TrafficLog[]
}

export async function getPeerMonitor(peerId: string): Promise<PeerMonitorResponse> {
  const res = await client.get<PeerMonitorResponse>(`/monitoring/peer/${peerId}`)
  return res.data
}
