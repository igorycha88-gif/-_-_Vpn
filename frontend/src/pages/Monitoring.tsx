import { useState } from 'react'
import { Tabs, Card, Table, Tag, Select, Spin, Alert, Typography } from 'antd'
import { useTrafficLogs, useRoutingLogs, useMonitoringStats } from '../hooks/useMonitoring'
import { usePeers } from '../hooks/usePeers'
import TrafficChart from '../components/TrafficChart'

const { Text } = Typography

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

export default function Monitoring() {
  const { data: peers } = usePeers()
  const { data: stats, isLoading: statsLoading, error: statsError } = useMonitoringStats()
  const [selectedPeer, setSelectedPeer] = useState<string | undefined>()

  const { data: trafficLogs, isLoading: trafficLoading } = useTrafficLogs(selectedPeer)
  const { data: routingLogs, isLoading: logsLoading } = useRoutingLogs(selectedPeer)

  if (statsError) return <Alert type="error" message="Ошибка загрузки мониторинга" />

  const peerOptions = (peers ?? []).map((p) => ({ value: p.id, label: p.name }))

  const logColumns = [
    {
      title: 'Время',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      render: (v: string) => new Date(v).toLocaleString('ru'),
    },
    { title: 'Домен', dataIndex: 'domain', key: 'domain', render: (v: string) => v || '—' },
    { title: 'IP', dataIndex: 'dest_ip', key: 'dest_ip', render: (v: string) => v || '—' },
    { title: 'Порт', dataIndex: 'dest_port', key: 'dest_port', render: (v: number) => v || '—' },
    {
      title: 'Действие',
      dataIndex: 'action',
      key: 'action',
      render: (action: string) => (
        <Tag color={action === 'direct' ? 'green' : action === 'proxy' ? 'blue' : 'red'}>
          {action === 'direct' ? 'Напрямую' : action === 'proxy' ? 'Прокси' : 'Блок'}
        </Tag>
      ),
    },
    {
      title: 'RX',
      dataIndex: 'bytes_rx',
      key: 'bytes_rx',
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'TX',
      dataIndex: 'bytes_tx',
      key: 'bytes_tx',
      render: (v: number) => formatBytes(v),
    },
  ]

  return (
    <div>
      <h2>Мониторинг</h2>

      <Spin spinning={statsLoading}>
        <Card style={{ marginBottom: 16 }}>
          <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap' }}>
            <Text>Клиентов: <strong>{stats?.active_peers ?? 0}</strong> / {stats?.total_peers ?? 0}</Text>
            <Text>Трафик RX: <strong>{formatBytes(stats?.total_rx ?? 0)}</strong></Text>
            <Text>Трафик TX: <strong>{formatBytes(stats?.total_tx ?? 0)}</strong></Text>
            <Text>Правил: <strong>{stats?.rules_count ?? 0}</strong></Text>
          </div>
        </Card>
      </Spin>

      <div style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="Фильтр по клиенту"
          style={{ width: 250 }}
          options={peerOptions}
          onChange={setSelectedPeer}
        />
      </div>

      <Tabs
        items={[
          {
            key: 'traffic',
            label: 'Трафик',
            children: (
              <Spin spinning={trafficLoading}>
                <TrafficChart data={trafficLogs ?? []} />
              </Spin>
            ),
          },
          {
            key: 'logs',
            label: 'Логи',
            children: (
              <Table
                dataSource={routingLogs ?? []}
                columns={logColumns}
                rowKey="id"
                loading={logsLoading}
                pagination={{ pageSize: 50 }}
                size="small"
              />
            ),
          },
        ]}
      />
    </div>
  )
}
