import { Card, Button, Typography, Space, Popconfirm, message } from 'antd'
import { LogoutOutlined, ExportOutlined } from '@ant-design/icons'
import { useAuth } from '../hooks/useAuth'
import { useNavigate } from 'react-router-dom'

const { Title, Paragraph } = Typography

export default function Settings() {
  const { logout } = useAuth()
  const navigate = useNavigate()

  const handleLogoutAll = async () => {
    await logout()
    navigate('/login')
    message.success('Все сессии завершены')
  }

  return (
    <div>
      <h2>Настройки системы</h2>

      <Card style={{ maxWidth: 600, marginBottom: 24 }}>
        <Title level={4}>Сессии</Title>
        <Paragraph>Завершить все активные сессии авторизации.</Paragraph>
        <Popconfirm title="Завершить все сессии?" onConfirm={handleLogoutAll}>
          <Button danger icon={<LogoutOutlined />}>
            Завершить все сессии
          </Button>
        </Popconfirm>
      </Card>

      <Card style={{ maxWidth: 600, marginBottom: 24 }}>
        <Title level={4}>Конфигурация</Title>
        <Paragraph>Экспорт и импорт конфигурации системы (в разработке).</Paragraph>
        <Space>
          <Button icon={<ExportOutlined />} disabled>
            Экспорт
          </Button>
          <Button disabled>
            Импорт
          </Button>
        </Space>
      </Card>

      <Card style={{ maxWidth: 600 }}>
        <Title level={4}>О системе</Title>
        <Paragraph><strong>SmartTraffic</strong> v1.0.0</Paragraph>
        <Paragraph>Система управления сетевым трафиком: рунет напрямую, остальной мир — через зарубежный прокси.</Paragraph>
      </Card>
    </div>
  )
}
