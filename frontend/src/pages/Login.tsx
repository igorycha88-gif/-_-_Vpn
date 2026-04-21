import { Form, Input, Button, Card, Typography, message } from 'antd'
import { LockOutlined, MailOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'
import type { LoginRequest } from '../types'

const { Title } = Typography

export default function Login() {
  const navigate = useNavigate()
  const { login } = useAuth()

  const onFinish = async (values: LoginRequest) => {
    await login(values)
    message.success('Вход выполнен')
    navigate('/')
  }

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', background: '#f0f2f5' }}>
      <Card style={{ width: 400 }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <Title level={3}>SmartTraffic</Title>
          <p style={{ color: '#666' }}>Панель администратора</p>
        </div>
        <Form onFinish={onFinish} size="large">
          <Form.Item name="email" rules={[{ required: true, message: 'Введите email' }]}>
            <Input prefix={<MailOutlined />} placeholder="Email" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: 'Введите пароль' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="Пароль" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block>
              Войти
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
