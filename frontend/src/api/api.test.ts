import { describe, it, expect, vi, beforeEach } from 'vitest'
import * as peersApi from './peers'
import * as routesApi from './routes'
import * as presetsApi from './presets'
import * as dnsApi from './dns'
import * as serversApi from './servers'

vi.mock('./client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

import client from './client'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const mockClient: any = client

describe('peers API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('listPeers calls GET /wg/peers', async () => {
    const peers = [{ id: '1', name: 'Test', device_type: 'iphone', public_key: 'pk', private_key: 'sk', address: 'addr', dns: '1.1.1.1', mtu: 1280, is_active: true, created_at: '', updated_at: '', total_rx: 0, total_tx: 0 }]
    mockClient.get.mockResolvedValue({ data: peers })
    const result = await peersApi.listPeers()
    expect(mockClient.get).toHaveBeenCalledWith('/wg/peers')
    expect(result).toEqual(peers)
  })

  it('createPeer calls POST /wg/peers', async () => {
    const peer = { id: '1', name: 'New', device_type: 'iphone', public_key: 'pk', private_key: 'sk', address: 'addr', dns: '1.1.1.1', mtu: 1280, is_active: true, created_at: '', updated_at: '', total_rx: 0, total_tx: 0 }
    mockClient.post.mockResolvedValue({ data: peer })
    const result = await peersApi.createPeer({ name: 'New', device_type: 'iphone' })
    expect(mockClient.post).toHaveBeenCalledWith('/wg/peers', { name: 'New', device_type: 'iphone' })
    expect(result).toEqual(peer)
  })

  it('deletePeer calls DELETE /wg/peers/:id', async () => {
    mockClient.delete.mockResolvedValue({ data: null })
    await peersApi.deletePeer('peer-1')
    expect(mockClient.delete).toHaveBeenCalledWith('/wg/peers/peer-1')
  })

  it('getPeerStats calls GET /wg/peers/:id/stats', async () => {
    const stats = { peer_id: '1', total_rx: 100, total_tx: 200, online: true }
    mockClient.get.mockResolvedValue({ data: stats })
    const result = await peersApi.getPeerStats('1')
    expect(mockClient.get).toHaveBeenCalledWith('/wg/peers/1/stats')
    expect(result).toEqual(stats)
  })

  it('togglePeer calls PUT /wg/peers/:id/toggle', async () => {
    mockClient.put.mockResolvedValue({ data: null })
    await peersApi.togglePeer('1', false)
    expect(mockClient.put).toHaveBeenCalledWith('/wg/peers/1/toggle', { active: false })
  })

  it('getPeerConfigUrl returns correct URL', () => {
    expect(peersApi.getPeerConfigUrl('abc')).toBe('/api/v1/wg/peers/abc/config')
  })

  it('getPeerQrUrl returns correct URL', () => {
    expect(peersApi.getPeerQrUrl('abc')).toBe('/api/v1/wg/peers/abc/qr')
  })
})

describe('routes API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('listRules calls GET /routes', async () => {
    mockClient.get.mockResolvedValue({ data: [] })
    await routesApi.listRules()
    expect(mockClient.get).toHaveBeenCalledWith('/routes')
  })

  it('createRule calls POST /routes', async () => {
    const rule = { id: '1', name: 'Test', type: 'domain', pattern: 'test.com', action: 'direct', priority: 1, is_active: true, created_at: '', updated_at: '' }
    mockClient.post.mockResolvedValue({ data: rule })
    const result = await routesApi.createRule({ name: 'Test', type: 'domain', pattern: 'test.com', action: 'direct' })
    expect(mockClient.post).toHaveBeenCalledWith('/routes', { name: 'Test', type: 'domain', pattern: 'test.com', action: 'direct' })
    expect(result).toEqual(rule)
  })

  it('updateRule calls PUT /routes/:id', async () => {
    const rule = { id: '1', name: 'Updated', type: 'domain', pattern: 'test.com', action: 'direct', priority: 1, is_active: true, created_at: '', updated_at: '' }
    mockClient.put.mockResolvedValue({ data: rule })
    const result = await routesApi.updateRule('1', { name: 'Updated' })
    expect(mockClient.put).toHaveBeenCalledWith('/routes/1', { name: 'Updated' })
    expect(result).toEqual(rule)
  })

  it('deleteRule calls DELETE /routes/:id', async () => {
    mockClient.delete.mockResolvedValue({ data: null })
    await routesApi.deleteRule('1')
    expect(mockClient.delete).toHaveBeenCalledWith('/routes/1')
  })

  it('reorderRules calls PUT /routes/reorder', async () => {
    mockClient.put.mockResolvedValue({ data: null })
    await routesApi.reorderRules({ ids: ['1', '2'] })
    expect(mockClient.put).toHaveBeenCalledWith('/routes/reorder', { ids: ['1', '2'] })
  })
})

describe('presets API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('listPresets calls GET /presets', async () => {
    mockClient.get.mockResolvedValue({ data: [] })
    await presetsApi.listPresets()
    expect(mockClient.get).toHaveBeenCalledWith('/presets')
  })

  it('applyPreset calls POST /presets/:id/apply', async () => {
    mockClient.post.mockResolvedValue({ data: { applied_rules: 3 } })
    const result = await presetsApi.applyPreset('preset-1')
    expect(mockClient.post).toHaveBeenCalledWith('/presets/preset-1/apply')
    expect(result).toEqual({ applied_rules: 3 })
  })
})

describe('dns API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getDnsSettings calls GET /dns/settings', async () => {
    const settings = { id: 1, upstream_ru: '77.88.8.8', upstream_foreign: '1.1.1.1', block_ads: false }
    mockClient.get.mockResolvedValue({ data: settings })
    const result = await dnsApi.getDnsSettings()
    expect(mockClient.get).toHaveBeenCalledWith('/dns/settings')
    expect(result).toEqual(settings)
  })

  it('updateDnsSettings calls PUT /dns/settings', async () => {
    const settings = { id: 1, upstream_ru: '8.8.8.8', upstream_foreign: '1.1.1.1', block_ads: true }
    mockClient.put.mockResolvedValue({ data: settings })
    const result = await dnsApi.updateDnsSettings({ upstream_ru: '8.8.8.8', block_ads: true })
    expect(mockClient.put).toHaveBeenCalledWith('/dns/settings', { upstream_ru: '8.8.8.8', block_ads: true })
    expect(result).toEqual(settings)
  })

  it('listDnsPresets calls GET /dns/presets', async () => {
    mockClient.get.mockResolvedValue({ data: [] })
    await dnsApi.listDnsPresets()
    expect(mockClient.get).toHaveBeenCalledWith('/dns/presets')
  })
})

describe('servers API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('getServersStatus calls GET /servers/status', async () => {
    const status = { ru: { online: true }, foreign: { online: false } }
    mockClient.get.mockResolvedValue({ data: status })
    const result = await serversApi.getServersStatus()
    expect(mockClient.get).toHaveBeenCalledWith('/servers/status')
    expect(result).toEqual(status)
  })

  it('getRuStats calls GET /servers/ru/stats', async () => {
    mockClient.get.mockResolvedValue({ data: {} })
    await serversApi.getRuStats()
    expect(mockClient.get).toHaveBeenCalledWith('/servers/ru/stats')
  })

  it('getForeignStats calls GET /servers/foreign/stats', async () => {
    mockClient.get.mockResolvedValue({ data: {} })
    await serversApi.getForeignStats()
    expect(mockClient.get).toHaveBeenCalledWith('/servers/foreign/stats')
  })
})
