import { Row, Col, Card, Statistic, Spin, Alert } from 'antd'
import {
  UserOutlined,
  CloudOutlined,
  PartitionOutlined,
  CheckCircleOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
} from '@ant-design/icons'
import { useMonitoringStats, usePeersStats } from '../hooks/useMonitoring'
import { useServersStatus } from '../hooks/useServers'
import ServerStatus from '../components/ServerStatus'
import { formatBytes } from '../utils/format'

function formatRate(bytesPerSec: number): string {
  if (bytesPerSec <= 0) return '0'
  const kbps = bytesPerSec / 128
  if (kbps < 1) return `${(bytesPerSec * 8).toFixed(0)} bps`
  if (kbps < 1000) return `${kbps.toFixed(1)} Kbps`
  return `${(kbps / 1000).toFixed(2)} Mbps`
}

export default function Dashboard() {
  const { data: stats, isLoading: statsLoading, error: statsError } = useMonitoringStats()
  const { data: peersStats, isLoading: peersStatsLoading } = usePeersStats()
  const { data: serverStatus, isLoading: serversLoading, error: serversError } = useServersStatus()

  if (statsError) return <Alert type="error" message="Ошибка загрузки статистики" />
  if (serversError) return <Alert type="error" message="Ошибка загрузки статуса серверов" />

  const totalBandwidthRx = (peersStats ?? []).reduce((acc, p) => acc + (p.online ? p.bandwidth_rate_rx : 0), 0)
  const totalBandwidthTx = (peersStats ?? []).reduce((acc, p) => acc + (p.online ? p.bandwidth_rate_tx : 0), 0)
  const totalConns = (peersStats ?? []).reduce((acc, p) => acc + (p.online ? p.active_conns : 0), 0)

  return (
    <div>
      <h2>Dashboard</h2>
      <Spin spinning={statsLoading || peersStatsLoading}>
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={24} sm={12} lg={6}>
            <Card>
              <Statistic
                title="Всего клиентов"
                value={stats?.total_peers ?? 0}
                prefix={<UserOutlined />}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <Card>
              <Statistic
                title="Онлайн клиентов"
                value={stats?.online_peers ?? 0}
                prefix={<CheckCircleOutlined />}
                valueStyle={{ color: '#52c41a' }}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <Card>
              <Statistic
                title="Входящий трафик"
                value={formatBytes(stats?.total_rx ?? 0)}
                prefix={<CloudOutlined />}
              />
            </Card>
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <Card>
              <Statistic
                title="Правил маршрутизации"
                value={stats?.rules_count ?? 0}
                prefix={<PartitionOutlined />}
              />
            </Card>
          </Col>
        </Row>
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={24} sm={8}>
            <Card>
              <Statistic
                title="Скорость загрузки"
                value={formatRate(totalBandwidthRx)}
                prefix={<ArrowDownOutlined />}
                valueStyle={{ color: '#1890ff' }}
              />
            </Card>
          </Col>
          <Col xs={24} sm={8}>
            <Card>
              <Statistic
                title="Скорость отдачи"
                value={formatRate(totalBandwidthTx)}
                prefix={<ArrowUpOutlined />}
                valueStyle={{ color: '#52c41a' }}
              />
            </Card>
          </Col>
          <Col xs={24} sm={8}>
            <Card>
              <Statistic
                title="Активных соединений"
                value={totalConns}
              />
            </Card>
          </Col>
        </Row>
      </Spin>

      <Spin spinning={serversLoading}>
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={12}>
            <ServerStatus title="Российский сервер" info={serverStatus?.ru ?? { online: false }} />
          </Col>
          <Col xs={24} lg={12}>
            <ServerStatus title="Зарубежный сервер" info={serverStatus?.foreign ?? { online: false }} />
          </Col>
        </Row>
      </Spin>
    </div>
  )
}
