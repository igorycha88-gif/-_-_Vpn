import client from './client'
import type { ServerStatus, ServerStats } from '../types'

export async function getServersStatus(): Promise<ServerStatus> {
  const res = await client.get<ServerStatus>('/servers/status')
  return res.data
}

export async function getRuStats(): Promise<ServerStats> {
  const res = await client.get<ServerStats>('/servers/ru/stats')
  return res.data
}

export async function getForeignStats(): Promise<ServerStats> {
  const res = await client.get<ServerStats>('/servers/foreign/stats')
  return res.data
}
