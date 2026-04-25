import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid, Legend } from 'recharts'
import type { TrafficAggregate } from '../api/monitoring'

interface TrafficChartProps {
  data: TrafficAggregate[]
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export default function TrafficChart({ data }: TrafficChartProps) {
  const sorted = [...data].sort((a, b) => (b.rx + b.tx) - (a.rx + a.tx)).slice(0, 30)

  if (sorted.length === 0) {
    return (
      <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
        Нет данных о трафике
      </div>
    )
  }

  return (
    <ResponsiveContainer width="100%" height={350}>
      <BarChart data={sorted}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey="domain" tick={{ fontSize: 11 }} angle={-30} textAnchor="end" height={80} />
        <YAxis tickFormatter={formatBytes} />
        <Tooltip formatter={(value: number) => formatBytes(value)} />
        <Legend />
        <Bar dataKey="rx" fill="#1890ff" name="RX" />
        <Bar dataKey="tx" fill="#52c41a" name="TX" />
      </BarChart>
    </ResponsiveContainer>
  )
}
