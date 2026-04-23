export type DeviceType = 'iphone' | 'android'

export interface Peer {
  id: string
  name: string
  email?: string
  device_type: DeviceType
  public_key: string
  private_key?: string
  address: string
  dns: string
  mtu: number
  is_active: boolean
  created_at: string
  updated_at: string
  total_rx: number
  total_tx: number
  last_seen?: string
}

export interface PeerCreateRequest {
  name: string
  email?: string
  device_type: DeviceType
}

export interface PeerStats {
  peer_id: string
  total_rx: number
  total_tx: number
  online: boolean
}

export interface RoutingRule {
  id: string
  name: string
  type: string
  pattern: string
  action: string
  priority: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface RoutingRuleCreateRequest {
  name: string
  type: string
  pattern: string
  action: string
  priority?: number
}

export interface RoutingRuleUpdateRequest {
  name?: string
  type?: string
  pattern?: string
  action?: string
  priority?: number
  is_active?: boolean
}

export interface ReorderRequest {
  ids: string[]
}

export interface Preset {
  id: string
  name: string
  description?: string
  rules: string
  is_builtin: boolean
  created_at: string
}

export interface PresetApplyResponse {
  applied_rules: number
}

export interface DNSSettings {
  id: number
  upstream_ru: string
  upstream_foreign: string
  block_ads: boolean
}

export interface DNSSettingsUpdateRequest {
  upstream_ru?: string
  upstream_foreign?: string
  block_ads?: boolean
}

export interface ServerStatus {
  ru: ServerInfo
  foreign: ServerInfo
}

export interface ServerInfo {
  online: boolean
  ip?: string
  uptime?: string
  cpu_usage?: string
  ram_usage?: string
  disk_usage?: string
}

export interface ServerStats {
  total_rx: number
  total_tx: number
  active_peers: number
  total_peers: number
  wg_status: string
  singbox_status: string
}

export interface TrafficLog {
  id: number
  peer_id?: string
  domain?: string
  dest_ip?: string
  dest_port?: number
  action: string
  bytes_rx: number
  bytes_tx: number
  timestamp: string
}

export interface TotalStats {
  total_rx: number
  total_tx: number
  active_peers: number
  total_peers: number
  rules_count: number
}

export interface Alert {
  id: string
  type: string
  message: string
  severity: string
  timestamp: string
}

export interface LoginRequest {
  email: string
  password: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface SessionResponse {
  user_id: string
  email: string
  role: string
}

export interface RefreshTokenRequest {
  refresh_token: string
}

export interface ApiError {
  error?: string
  errors?: Record<string, string>
}

export interface DNSPreset {
  id: string
  name: string
  servers: string
}
