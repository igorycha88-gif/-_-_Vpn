import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'
import type { TrafficLog } from '../types'

interface TrafficChartProps {
  data: TrafficLog[]
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export default function TrafficChart({ data }: TrafficChartProps) {
  const chartData = data.slice(0, 50).map((log) => ({
    domain: log.domain || log.dest_ip || '—',
    rx: log.bytes_rx,
    tx: log.bytes_tx,
  }))

  if (chartData.length === 0) {
    return <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>Нет данных о трафике</div>
  }

  return (
    <ResponsiveContainer width="100%" height={300}>
      <BarChart data={chartData}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="domain" tick={{ fontSize: 11 }} angle={-30} textAnchor="end" height={80} />
        <YAxis tickFormatter={formatBytes} />
        <Tooltip formatter={(value: number) => formatBytes(value)} />
        <Bar dataKey="rx" fill="#1890ff" name="RX" />
        <Bar dataKey="tx" fill="#52c41a" name="TX" />
      </BarChart>
    </ResponsiveContainer>
  )
}
