import client from './client'
import type { Preset, PresetApplyResponse } from '../types'

export async function listPresets(): Promise<Preset[]> {
  const res = await client.get<Preset[]>('/presets')
  return res.data
}

export async function applyPreset(id: string): Promise<PresetApplyResponse> {
  const res = await client.post<PresetApplyResponse>(`/presets/${id}/apply`)
  return res.data
}
