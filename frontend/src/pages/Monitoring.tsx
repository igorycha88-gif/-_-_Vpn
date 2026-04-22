import { useState } from 'react'
import { Tabs, Card, Table, Tag, Select, Spin, Alert, Typography, Row, Col, Badge } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, BellOutlined } from '@ant-design/icons'
import { useTrafficLogs, useRoutingLogs, useMonitoringStats, useAlerts, usePeerMonitor } from '../hooks/useMonitoring'
import { usePeers } from '../hooks/usePeers'
import TrafficChart from '../components/TrafficChart'

const { Text } = Typography

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

function timeAgo(dateStr?: string): string {
  if (!dateStr) return 'никогда'
  const diff = Date.now() - new Date(dateStr).getTime()
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return `${sec} сек назад`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} мин назад`
  const hours = Math.floor(min / 60)
  if (hours < 24) return `${hours} ч назад`
  return `${Math.floor(hours / 24)} дн назад`
}

function isOnline(lastSeen?: string): boolean {
  if (!lastSeen) return false
  return (Date.now() - new Date(lastSeen).getTime()) < 120_000
}

export default function Monitoring() {
  const { data: peers } = usePeers()
  const { data: stats, isLoading: statsLoading, error: statsError } = useMonitoringStats()
  const { data: alerts } = useAlerts()
  const [selectedPeer, setSelectedPeer] = useState<string | undefined>()

  const { data: trafficLogs, isLoading: trafficLoading } = useTrafficLogs(selectedPeer)
  const { data: routingLogs, isLoading: logsLoading } = useRoutingLogs(selectedPeer)
  const { data: peerData } = usePeerMonitor(selectedPeer)

  if (statsError) return <Alert type="error" message="Ошибка загрузки мониторинга" />

  const peerOptions = (peers ?? []).map((p) => {
    const online = isOnline(p.last_seen)
    return {
      value: p.id,
      label: (
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Badge status={online ? 'success' : 'default'} />
          {p.name}
          <Text type="secondary" style={{ fontSize: 12 }}>
            ({formatBytes(p.total_rx + p.total_tx)})
          </Text>
        </span>
      ),
    }
  })

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
        <Tag color={action === 'direct' ? 'green' : action === 'proxy' ? 'blue' : action === 'transfer' ? 'purple' : 'red'}>
          {action === 'direct' ? 'Напрямую' : action === 'proxy' ? 'Прокси' : action === 'transfer' ? 'Трафик' : 'Блок'}
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

      {(peers ?? []).length > 0 && (
        <Card title="Клиенты" style={{ marginBottom: 16 }} size="small">
          <Row gutter={[12, 8]}>
            {(peers ?? []).map((p) => {
              const online = isOnline(p.last_seen)
              return (
                <Col key={p.id} xs={24} sm={12} md={8} lg={6}>
                  <Card
                    size="small"
                    hoverable
                    onClick={() => setSelectedPeer(p.id === selectedPeer ? undefined : p.id)}
                    style={{
                      borderColor: p.id === selectedPeer ? '#1890ff' : undefined,
                      background: p.id === selectedPeer ? '#f0f5ff' : undefined,
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <Text strong ellipsis style={{ maxWidth: 120 }}>{p.name}</Text>
                      {online ? (
                        <Tag icon={<CheckCircleOutlined />} color="success">Онлайн</Tag>
                      ) : (
                        <Tag icon={<CloseCircleOutlined />} color="default">Офлайн</Tag>
                      )}
                    </div>
                    <div style={{ marginTop: 4 }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        RX: {formatBytes(p.total_rx)} / TX: {formatBytes(p.total_tx)}
                      </Text>
                    </div>
                    <div>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        Был: {timeAgo(p.last_seen)}
                      </Text>
                    </div>
                  </Card>
                </Col>
              )
            })}
          </Row>
        </Card>
      )}

      <div style={{ marginBottom: 16, display: 'flex', gap: 16, alignItems: 'center' }}>
        <Select
          allowClear
          placeholder="Фильтр по клиенту"
          style={{ width: 250 }}
          options={peerOptions.map((o) => ({ value: o.value, label: typeof o.label === 'string' ? o.label : peers?.find(p => p.id === o.value)?.name || '' })) }
          onChange={(v) => setSelectedPeer(v)}
          value={selectedPeer}
        />
        {(alerts ?? []).length > 0 && (
          <Tag icon={<BellOutlined />} color="blue">{alerts!.length} алертов</Tag>
        )}
      </div>

      {selectedPeer && peerData && (
        <Card style={{ marginBottom: 16 }} size="small">
          <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', alignItems: 'center' }}>
            <Text>Клиент: <strong>{peerData.peer.name}</strong></Text>
            <Text>IP: <strong>{peerData.peer.address}</strong></Text>
            <Text>RX: <strong>{formatBytes(peerData.peer.total_rx)}</strong></Text>
            <Text>TX: <strong>{formatBytes(peerData.peer.total_tx)}</strong></Text>
            <Text>Всего: <strong>{formatBytes(peerData.peer.total_rx + peerData.peer.total_tx)}</strong></Text>
            <Text>Последняя активность: <strong>{timeAgo(peerData.peer.last_seen)}</strong></Text>
            {isOnline(peerData.peer.last_seen) ? (
              <Tag icon={<CheckCircleOutlined />} color="success">Онлайн</Tag>
            ) : (
              <Tag icon={<CloseCircleOutlined />} color="default">Офлайн</Tag>
            )}
          </div>
        </Card>
      )}

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
          {
            key: 'alerts',
            label: (
              <span>
                <BellOutlined /> Алерты {(alerts ?? []).length > 0 ? `(${alerts!.length})` : ''}
              </span>
            ),
            children: (
              <Table
                dataSource={alerts ?? []}
                columns={[
                  {
                    title: 'Время',
                    dataIndex: 'timestamp',
                    key: 'timestamp',
                    width: 180,
                    render: (v: string) => new Date(v).toLocaleString('ru'),
                  },
                  {
                    title: 'Тип',
                    dataIndex: 'type',
                    key: 'type',
                    render: (v: string) => {
                      const colors: Record<string, string> = {
                        peer_online: 'green',
                        peer_offline: 'orange',
                      }
                      return <Tag color={colors[v] || 'blue'}>{v}</Tag>
                    },
                  },
                  { title: 'Сообщение', dataIndex: 'message', key: 'message' },
                  {
                    title: 'Серьёзность',
                    dataIndex: 'severity',
                    key: 'severity',
                    render: (v: string) => {
                      const colors: Record<string, string> = {
                        info: 'blue',
                        warning: 'orange',
                        error: 'red',
                      }
                      return <Tag color={colors[v] || 'default'}>{v}</Tag>
                    },
                  },
                ]}
                rowKey="id"
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
