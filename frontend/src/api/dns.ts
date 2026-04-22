import client from './client'
import type { DNSSettings, DNSSettingsUpdateRequest, DNSPreset } from '../types'

export async function getDnsSettings(): Promise<DNSSettings> {
  const res = await client.get<DNSSettings>('/dns/settings')
  return res.data
}

export async function updateDnsSettings(data: DNSSettingsUpdateRequest): Promise<DNSSettings> {
  const res = await client.put<DNSSettings>('/dns/settings', data)
  return res.data
}

export async function listDnsPresets(): Promise<DNSPreset[]> {
  const res = await client.get<DNSPreset[]>('/dns/presets')
  return res.data
}
