import { Card, Tag } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import type { ServerInfo } from '../types'

interface ServerStatusProps {
  title: string
  info: ServerInfo
}

export default function ServerStatus({ title, info }: ServerStatusProps) {
  return (
    <Card
      title={title}
      extra={
        info.online ? (
          <Tag icon={<CheckCircleOutlined />} color="success">Онлайн</Tag>
        ) : (
          <Tag icon={<CloseCircleOutlined />} color="error">Офлайн</Tag>
        )
      }
    >
      {info.ip && <p><strong>IP:</strong> {info.ip}</p>}
      {info.uptime && <p><strong>Uptime:</strong> {info.uptime}</p>}
      {info.cpu_usage && <p><strong>CPU:</strong> {info.cpu_usage}</p>}
      {info.ram_usage && <p><strong>RAM:</strong> {info.ram_usage}</p>}
      {info.disk_usage && <p><strong>Диск:</strong> {info.disk_usage}</p>}
    </Card>
  )
}
