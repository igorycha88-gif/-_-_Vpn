import { useState } from 'react'
import { Tabs, Card, Table, Tag, Select, Spin, Alert, Typography, Row, Col, Badge, Empty, Progress, Button, Popconfirm, Tooltip } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, BellOutlined, ExclamationCircleOutlined, BarChartOutlined, DeleteOutlined, ClearOutlined, HistoryOutlined, LinkOutlined } from '@ant-design/icons'
import { useRoutingLogs, useMonitoringStats, useAlerts, usePeerMonitor, usePeersStats, useTrafficAggregate, usePeerSessions, useDeleteAlert, useClearAllAlerts } from '../hooks/useMonitoring'
import { usePeers } from '../hooks/usePeers'
import TrafficChart from '../components/TrafficChart'
import type { PeerTrafficSummary, PeerSession } from '../types'
import { formatBytes } from '../utils/format'

const { Text } = Typography

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

function formatRate(bytesPerSec: number): string {
  if (bytesPerSec <= 0) return '0'
  const kbps = bytesPerSec / 128
  if (kbps < 1) return `${(bytesPerSec * 8).toFixed(0)} bps`
  if (kbps < 1000) return `${kbps.toFixed(1)} Kbps`
  return `${(kbps / 1000).toFixed(2)} Mbps`
}

function sessionDuration(connectedAt?: string): string {
  if (!connectedAt) return '—'
  const diff = Date.now() - new Date(connectedAt).getTime()
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return `${sec} сек`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} мин`
  const hours = Math.floor(min / 60)
  const remainMin = min % 60
  if (hours < 24) return `${hours} ч ${remainMin} мин`
  return `${Math.floor(hours / 24)} дн ${hours % 24} ч`
}

function sessionDurationRange(start: string, end?: string): string {
  const s = new Date(start).getTime()
  const e = end ? new Date(end).getTime() : Date.now()
  const diff = Math.max(0, e - s)
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return `${sec} сек`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} мин`
  const hours = Math.floor(min / 60)
  const remainMin = min % 60
  if (hours < 24) return `${hours} ч ${remainMin} мин`
  return `${Math.floor(hours / 24)} дн ${hours % 24} ч`
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
  const [sessionPeerId, setSessionPeerId] = useState<string | undefined>()

  const { data: trafficLogs, isLoading: trafficLoading, error: trafficError } = useTrafficAggregate(selectedPeer)
  const { data: routingLogs, isLoading: logsLoading, error: logsError } = useRoutingLogs(selectedPeer)
  const { data: peerData } = usePeerMonitor(selectedPeer)
  const { data: peerSessions } = usePeerSessions(sessionPeerId)
  const deleteAlertMut = useDeleteAlert()
  const clearAllAlertsMut = useClearAllAlerts()

  const hasAnyError = statsError && trafficError && logsError

  const peerVpnStatus = (peerId: string): boolean => {
    const ps = peersStats?.find(s => s.peer_id === peerId)
    if (ps) return ps.online
    return false
  }

  const peerOptions = (peers ?? []).map((p) => {
    const vpnOnline = peerVpnStatus(p.id)
    return {
      value: p.id,
      label: (
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Badge status={vpnOnline ? 'success' : 'default'} />
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
  const totalBandwidthRx = (peersStats ?? []).reduce((acc, p) => acc + (p.online ? p.bandwidth_rate_rx : 0), 0)
  const totalBandwidthTx = (peersStats ?? []).reduce((acc, p) => acc + (p.online ? p.bandwidth_rate_tx : 0), 0)

  const peerStatsColumns = [
    {
      title: 'Клиент',
      key: 'name',
      render: (_: unknown, r: PeerTrafficSummary) => (
        <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Badge status={r.online ? 'success' : r.is_active ? 'default' : 'error'} />
          <Text strong>{r.peer_name}</Text>
        </span>
      ),
    },
    {
      title: 'Статус',
      key: 'status',
      width: 130,
      render: (_: unknown, r: PeerTrafficSummary) => {
        if (!r.is_active) {
          return <Tag color="red">Выключен</Tag>
        }
        return r.online ? (
          <Tag icon={<CheckCircleOutlined />} color="success">VPN</Tag>
        ) : (
          <Tag icon={<CloseCircleOutlined />} color="default">Офлайн</Tag>
        )
      },
    },
    {
      title: 'Скорость',
      key: 'bandwidth',
      width: 170,
      render: (_: unknown, r: PeerTrafficSummary) => {
        if (!r.online) return <Text type="secondary">—</Text>
        return (
          <div>
            <Text style={{ fontSize: 12, color: '#1890ff' }}>↓ {formatRate(r.bandwidth_rate_rx)}</Text>
            {' '}
            <Text style={{ fontSize: 12, color: '#52c41a' }}>↑ {formatRate(r.bandwidth_rate_tx)}</Text>
          </div>
        )
      },
    },
    {
      title: 'Соед.',
      key: 'active_conns',
      width: 80,
      sorter: (a: PeerTrafficSummary, b: PeerTrafficSummary) => a.active_conns - b.active_conns,
      render: (_: unknown, r: PeerTrafficSummary) => {
        if (!r.online) return <Text type="secondary">0</Text>
        return <Tag color={r.active_conns > 0 ? 'blue' : 'default'}>{r.active_conns}</Tag>
      },
    },
    {
      title: 'Сессия',
      key: 'session',
      width: 140,
      render: (_: unknown, r: PeerTrafficSummary) => {
        if (!r.connected_at) return <Text type="secondary">—</Text>
        return (
          <div>
            <Text style={{ fontSize: 12 }}>{sessionDuration(r.connected_at)}</Text>
            <br />
            <Text type="secondary" style={{ fontSize: 11 }}>
              {formatBytes(r.session_rx + r.session_tx)}
            </Text>
          </div>
        )
      },
    },
    {
      title: 'RX',
      dataIndex: 'total_rx',
      key: 'total_rx',
      width: 100,
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'TX',
      dataIndex: 'total_tx',
      key: 'total_tx',
      width: 100,
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'Доля',
      key: 'share',
      width: 160,
      render: (_: unknown, r: PeerTrafficSummary) => {
        const total = r.total_rx + r.total_tx
        const pct = totalAllTraffic > 0 ? Math.round((total / totalAllTraffic) * 100) : 0
        return (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Progress percent={pct} size="small" style={{ width: 80, marginBottom: 0 }} />
            <Text type="secondary" style={{ fontSize: 12 }}>{formatBytes(total)}</Text>
          </div>
        )
      },
    },
    {
      title: 'Топ домен',
      dataIndex: 'top_domain',
      key: 'top_domain',
      render: (v: string) => v ? <Tag>{v}</Tag> : '—',
    },
    {
      title: 'Действия',
      key: 'actions',
      width: 50,
      render: (_: unknown, r: PeerTrafficSummary) => (
        <Tooltip title="История сессий">
          <Button
            type="text"
            size="small"
            icon={<HistoryOutlined />}
            onClick={() => setSessionPeerId(r.peer_id)}
          />
        </Tooltip>
      ),
    },
    {
      title: 'Активность',
      dataIndex: 'last_seen',
      key: 'last_seen',
      width: 140,
      render: (v: string) => timeAgo(v),
    },
  ]

  const sessionColumns = [
    {
      title: 'Подключился',
      dataIndex: 'connected_at',
      key: 'connected_at',
      width: 180,
      render: (v: string) => new Date(v).toLocaleString('ru'),
    },
    {
      title: 'Отключился',
      dataIndex: 'disconnected_at',
      key: 'disconnected_at',
      width: 180,
      render: (v?: string) => v ? new Date(v).toLocaleString('ru') : <Tag color="green">Активна</Tag>,
    },
    {
      title: 'Длительность',
      key: 'duration',
      width: 120,
      render: (_: unknown, r: PeerSession) => sessionDurationRange(r.connected_at, r.disconnected_at),
    },
    {
      title: 'RX',
      dataIndex: 'bytes_rx',
      key: 'bytes_rx',
      width: 100,
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'TX',
      dataIndex: 'bytes_tx',
      key: 'bytes_tx',
      width: 100,
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'Соед.',
      dataIndex: 'connections_count',
      key: 'connections_count',
      width: 80,
      render: (v: number) => v || 0,
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
              <Text>Клиентов: <strong>{stats?.online_peers ?? 0}</strong> онлайн / <strong>{stats?.active_peers ?? 0}</strong> активных / {stats?.total_peers ?? 0} всего</Text>
              <Text>Трафик RX: <strong>{formatBytes(stats?.total_rx ?? 0)}</strong></Text>
              <Text>Трафик TX: <strong>{formatBytes(stats?.total_tx ?? 0)}</strong></Text>
              <Text>Общая скорость: <strong style={{ color: '#1890ff' }}>↓ {formatRate(totalBandwidthRx)}</strong> <strong style={{ color: '#52c41a' }}>↑ {formatRate(totalBandwidthTx)}</strong></Text>
              <Text>Правил: <strong>{stats?.rules_count ?? 0}</strong></Text>
            </div>
          </Card>
        </Spin>
      )}

      {(peers ?? []).length > 0 ? (
        <Card title="Клиенты" style={{ marginBottom: 16 }} size="small">
          <Row gutter={[12, 8]}>
            {(peers ?? []).map((p) => {
              const vpnOnline = peerVpnStatus(p.id)
              const rtPeer = peersStats?.find(s => s.peer_id === p.id)
              return (
                <Col key={p.id} xs={24} sm={12} md={8} lg={6}>
                  <Card
                    size="small"
                    hoverable
                    onClick={() => setSelectedPeer(p.id === selectedPeer ? undefined : p.id)}
                    style={{
                      borderColor: p.id === selectedPeer ? '#1890ff' : undefined,
                      background: p.id === selectedPeer ? '#f0f5ff' : undefined,
                      opacity: p.is_active ? 1 : 0.6,
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <Text strong ellipsis style={{ maxWidth: 120 }}>{p.name}</Text>
                      <div style={{ display: 'flex', gap: 4 }}>
                        {p.is_active ? (
                          vpnOnline ? (
                            <Tag icon={<CheckCircleOutlined />} color="success">VPN</Tag>
                          ) : (
                            <Tag icon={<CloseCircleOutlined />} color="default">Офлайн</Tag>
                          )
                        ) : (
                          <Tag color="red">Выключен</Tag>
                        )}
                      </div>
                    </div>
                    {vpnOnline && rtPeer && (
                      <div style={{ marginTop: 4 }}>
                        <Text style={{ fontSize: 12, color: '#1890ff' }}>↓ {formatRate(rtPeer.bandwidth_rate_rx)}</Text>
                        {' '}
                        <Text style={{ fontSize: 12, color: '#52c41a' }}>↑ {formatRate(rtPeer.bandwidth_rate_tx)}</Text>
                        {rtPeer.active_conns > 0 && (
                          <span style={{ marginLeft: 8 }}>
                            <LinkOutlined style={{ fontSize: 11 }} />
                            <Text style={{ fontSize: 11, marginLeft: 2 }}>{rtPeer.active_conns}</Text>
                          </span>
                        )}
                      </div>
                    )}
                    <div style={{ marginTop: 2 }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        RX: {formatBytes(p.total_rx)} / TX: {formatBytes(p.total_tx)}
                      </Text>
                    </div>
                    {vpnOnline && rtPeer?.connected_at && (
                      <div>
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          Сессия: {sessionDuration(rtPeer.connected_at)} ({formatBytes(rtPeer.session_rx + rtPeer.session_tx)})
                        </Text>
                      </div>
                    )}
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
            {peerData.realtime && (
              <>
                <Text>Соед.: <strong>{peerData.realtime.active_connections}</strong></Text>
                <Text>Скорость: <strong style={{ color: '#1890ff' }}>↓ {formatRate(peerData.realtime.bandwidth_rate_rx)}</strong> <strong style={{ color: '#52c41a' }}>↑ {formatRate(peerData.realtime.bandwidth_rate_tx)}</strong></Text>
                {peerData.realtime.connected_at && (
                  <Text>Сессия: <strong>{sessionDuration(peerData.realtime.connected_at)}</strong> ({formatBytes(peerData.realtime.session_rx + peerData.realtime.session_tx)})</Text>
                )}
              </>
            )}
            <Text>Последняя активность: <strong>{timeAgo(peerData.peer.last_seen)}</strong></Text>
            {isOnline(peerData.peer.last_seen) ? (
              <Tag icon={<CheckCircleOutlined />} color="success">Активен</Tag>
            ) : (
              <Tag icon={<CloseCircleOutlined />} color="default">Неактивен</Tag>
            )}
            <Button size="small" icon={<HistoryOutlined />} onClick={() => setSessionPeerId(selectedPeer)}>
              Сессии
            </Button>
          </div>
        </Card>
      )}

      {sessionPeerId && peerSessions && (
        <Card
          title={<span><HistoryOutlined /> История сессий</span>}
          style={{ marginBottom: 16 }}
          size="small"
          extra={<Button size="small" onClick={() => setSessionPeerId(undefined)}>Закрыть</Button>}
        >
          {peerSessions.length > 0 ? (
            <Table
              dataSource={peerSessions}
              columns={sessionColumns}
              rowKey="id"
              pagination={{ pageSize: 20 }}
              size="small"
            />
          ) : (
            <Empty description="Нет данных о сессиях" />
          )}
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
                    scroll={{ x: 1300 }}
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
              <div>
                {(alerts ?? []).length > 0 && (
                  <div style={{ marginBottom: 12 }}>
                    <Popconfirm
                      title="Очистить все алерты?"
                      onConfirm={() => clearAllAlertsMut.mutate()}
                      okText="Да"
                      cancelText="Нет"
                    >
                      <Button icon={<ClearOutlined />} danger size="small">
                        Очистить все
                      </Button>
                    </Popconfirm>
                  </div>
                )}
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
                    {
                      title: '',
                      key: 'actions',
                      width: 50,
                      render: (_: unknown, r: { id: string }) => (
                        <Popconfirm
                          title="Удалить алерт?"
                          onConfirm={() => deleteAlertMut.mutate(r.id)}
                          okText="Да"
                          cancelText="Нет"
                        >
                          <Button type="text" size="small" icon={<DeleteOutlined />} danger />
                        </Popconfirm>
                      ),
                    },
                  ]}
                  rowKey="id"
                  pagination={{ pageSize: 50 }}
                  size="small"
                />
              </div>
            ),
          },
        ]}
      />
    </div>
  )
}
