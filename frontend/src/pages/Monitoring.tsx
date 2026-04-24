import { useState } from 'react'
import { Tabs, Card, Table, Tag, Select, Spin, Alert, Typography, Row, Col, Badge, Empty, Progress } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, BellOutlined, ExclamationCircleOutlined, BarChartOutlined } from '@ant-design/icons'
import { useRoutingLogs, useMonitoringStats, useAlerts, usePeerMonitor, usePeersStats, useTrafficAggregate } from '../hooks/useMonitoring'
import { usePeers } from '../hooks/usePeers'
import TrafficChart from '../components/TrafficChart'
import type { PeerTrafficSummary } from '../types'

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

function renderAction(action: string) {
  const map: Record<string, { label: string; color: string }> = {
    direct: { label: 'Напрямую', color: 'green' },
    proxy: { label: 'Прокси', color: 'blue' },
    vless_transfer: { label: 'VLESS', color: 'purple' },
    tunnel_transfer: { label: 'Тоннель', color: 'geekblue' },
    block: { label: 'Блок', color: 'red' },
    transfer: { label: 'Трафик', color: 'purple' },
  }
  const info = map[action] || { label: action, color: 'default' }
  return <Tag color={info.color}>{info.label}</Tag>
}

export default function Monitoring() {
  const { data: peers, error: peersError } = usePeers()
  const { data: stats, isLoading: statsLoading, error: statsError } = useMonitoringStats()
  const { data: alerts, error: alertsError } = useAlerts()
  const { data: peersStats, isLoading: peersStatsLoading, error: peersStatsError } = usePeersStats()
  const [selectedPeer, setSelectedPeer] = useState<string | undefined>()

  const { data: trafficLogs, isLoading: trafficLoading, error: trafficError } = useTrafficAggregate(selectedPeer)
  const { data: routingLogs, isLoading: logsLoading, error: logsError } = useRoutingLogs(selectedPeer)
  const { data: peerData } = usePeerMonitor(selectedPeer)

  const hasAnyError = statsError && trafficError && logsError

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
      render: (action: string) => renderAction(action),
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

  const totalAllTraffic = (peersStats ?? []).reduce((acc, p) => acc + p.total_rx + p.total_tx, 0)

  const peerStatsColumns = [
    {
      title: 'Клиент',
      key: 'name',
      render: (_: unknown, r: PeerTrafficSummary) => (
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Badge status={r.online ? 'success' : 'default'} />
          <Text strong>{r.peer_name}</Text>
        </span>
      ),
    },
    {
      title: 'Статус',
      key: 'status',
      width: 100,
      render: (_: unknown, r: PeerTrafficSummary) =>
        r.online ? (
          <Tag icon={<CheckCircleOutlined />} color="success">Онлайн</Tag>
        ) : (
          <Tag icon={<CloseCircleOutlined />} color="default">Офлайн</Tag>
        ),
    },
    {
      title: 'RX',
      dataIndex: 'total_rx',
      key: 'total_rx',
      width: 120,
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'TX',
      dataIndex: 'total_tx',
      key: 'total_tx',
      width: 120,
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'Доля трафика',
      key: 'share',
      width: 200,
      render: (_: unknown, r: PeerTrafficSummary) => {
        const total = r.total_rx + r.total_tx
        const pct = totalAllTraffic > 0 ? Math.round((total / totalAllTraffic) * 100) : 0
        return (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Progress percent={pct} size="small" style={{ width: 100, marginBottom: 0 }} />
            <Text type="secondary" style={{ fontSize: 12 }}>{formatBytes(total)}</Text>
          </div>
        )
      },
    },
    {
      title: 'Соединений (24ч)',
      dataIndex: 'conn_count',
      key: 'conn_count',
      width: 130,
      render: (v: number) => v.toLocaleString('ru'),
    },
    {
      title: 'Топ домен',
      dataIndex: 'top_domain',
      key: 'top_domain',
      render: (v: string) => v ? <Tag>{v}</Tag> : '—',
    },
    {
      title: 'Последняя активность',
      dataIndex: 'last_seen',
      key: 'last_seen',
      width: 160,
      render: (v: string) => timeAgo(v),
    },
  ]

  if (hasAnyError) {
    return (
      <div>
        <h2>Мониторинг</h2>
        <Alert
          type="error"
          message="Ошибка загрузки мониторинга"
          description="Не удалось подключиться к API серверу. Проверьте что бэкенд запущен и доступен."
          showIcon
          icon={<ExclamationCircleOutlined />}
        />
      </div>
    )
  }

  return (
    <div>
      <h2>Мониторинг</h2>

      {statsError ? (
        <Alert type="warning" message="Не удалось загрузить статистику" style={{ marginBottom: 16 }} showIcon closable />
      ) : (
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
      )}

      {(peers ?? []).length > 0 ? (
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
      ) : !statsLoading && (
        <Card style={{ marginBottom: 16 }}>
          <Empty description="Нет VLESS клиентов. Добавьте клиентов на странице управления." />
        </Card>
      )}

      {peersError && (
        <Alert type="warning" message="Ошибка загрузки клиентов" style={{ marginBottom: 16 }} showIcon />
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
            key: 'peers-stats',
            label: (
              <span>
                <BarChartOutlined /> Статистика по клиентам
              </span>
            ),
            children: peersStatsError ? (
              <Alert type="error" message="Ошибка загрузки статистики" description="Не удалось получить статистику по клиентам." showIcon />
            ) : (
              <Spin spinning={peersStatsLoading}>
                {(peersStats ?? []).length > 0 ? (
                  <Table
                    dataSource={peersStats ?? []}
                    columns={peerStatsColumns}
                    rowKey="peer_id"
                    pagination={{ pageSize: 20 }}
                    size="small"
                  />
                ) : (
                  <Empty description="Нет данных о трафике клиентов" />
                )}
              </Spin>
            ),
          },
          {
            key: 'traffic',
            label: 'Трафик',
            children: trafficError ? (
              <Alert type="error" message="Ошибка загрузки данных трафика" description="Не удалось получить логи трафика от сервера." showIcon />
            ) : (
              <Spin spinning={trafficLoading}>
                <TrafficChart data={trafficLogs ?? []} />
              </Spin>
            ),
          },
          {
            key: 'logs',
            label: 'Логи',
            children: logsError ? (
              <Alert type="error" message="Ошибка загрузки логов маршрутизации" description="Не удалось получить логи от сервера." showIcon />
            ) : (
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
            children: alertsError ? (
              <Alert type="error" message="Ошибка загрузки алертов" showIcon />
            ) : (
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
                        peer: 'green',
                        system: 'blue',
                        tunnel: 'orange',
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
