import { Row, Col, Card, Statistic, Spin, Alert } from 'antd'
import {
  UserOutlined,
  CloudOutlined,
  PartitionOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons'
import { useMonitoringStats } from '../hooks/useMonitoring'
import { useServersStatus } from '../hooks/useServers'
import ServerStatus from '../components/ServerStatus'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

export default function Dashboard() {
  const { data: stats, isLoading: statsLoading, error: statsError } = useMonitoringStats()
  const { data: serverStatus, isLoading: serversLoading, error: serversError } = useServersStatus()

  if (statsError) return <Alert type="error" message="Ошибка загрузки статистики" />
  if (serversError) return <Alert type="error" message="Ошибка загрузки статуса серверов" />

  return (
    <div>
      <h2>Dashboard</h2>
      <Spin spinning={statsLoading}>
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
                title="Активных клиентов"
                value={stats?.active_peers ?? 0}
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
