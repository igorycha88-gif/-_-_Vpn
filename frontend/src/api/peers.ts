import client from './client'
import type { Peer, PeerCreateRequest, PeerStats } from '../types'

export async function listPeers(): Promise<Peer[]> {
  const res = await client.get<Peer[]>('/wg/peers')
  return res.data
}

export async function createPeer(data: PeerCreateRequest): Promise<Peer> {
  const res = await client.post<Peer>('/wg/peers', data)
  return res.data
}

export async function getPeer(id: string): Promise<Peer> {
  const res = await client.get<Peer>(`/wg/peers/${id}`)
  return res.data
}

export async function deletePeer(id: string): Promise<void> {
  await client.delete(`/wg/peers/${id}`)
}

export function getPeerConfigUrl(id: string): string {
  return `/api/v1/wg/peers/${id}/config`
}

export function getPeerQrUrl(id: string): string {
  return `/api/v1/wg/peers/${id}/qr`
}

export async function getPeerStats(id: string): Promise<PeerStats> {
  const res = await client.get<PeerStats>(`/wg/peers/${id}/stats`)
  return res.data
}

export async function togglePeer(id: string, active: boolean): Promise<void> {
  await client.put(`/wg/peers/${id}/toggle`, { active })
}
