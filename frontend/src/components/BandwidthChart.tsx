import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid, Legend } from 'recharts'
import type { PeerTrafficSummary } from '../types'
import { formatBytes } from '../utils/format'

interface BandwidthChartProps {
  peers: PeerTrafficSummary[]
}

export default function BandwidthChart({ peers }: BandwidthChartProps) {
  const onlinePeers = peers.filter(p => p.online)

  if (onlinePeers.length === 0) {
    return (
      <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
        Нет активных клиентов с подключением
      </div>
    )
  }

  const data = onlinePeers.map(p => ({
    name: p.peer_name,
    rx: Math.round(p.bandwidth_rate_rx * 8 / 1024),
    tx: Math.round(p.bandwidth_rate_tx * 8 / 1024),
    rxBytes: p.bandwidth_rate_rx,
    txBytes: p.bandwidth_rate_tx,
    conns: p.active_conns,
  }))

  return (
    <div>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="name" tick={{ fontSize: 11 }} />
          <YAxis tickFormatter={(v: number) => `${v} Kbps`} tick={{ fontSize: 11 }} />
          <Tooltip
            formatter={(value: number, name: string) => {
              const entry = data.find(d => d[name === 'rx' ? 'rx' : 'tx'] === value)
              const bytesPerSec = name === 'rx' ? (entry?.rxBytes ?? 0) : (entry?.txBytes ?? 0)
              return `${value} Kbps (${formatBytes(bytesPerSec)}/s)`
            }}
          />
          <Legend />
          <Line type="monotone" dataKey="rx" stroke="#1890ff" name="↓ Скачивание" strokeWidth={2} dot={{ r: 4 }} />
          <Line type="monotone" dataKey="tx" stroke="#52c41a" name="↑ Загрузка" strokeWidth={2} dot={{ r: 4 }} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
