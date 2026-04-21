import { useState } from 'react'
import { Layout, Menu, Button, Typography } from 'antd'
import {
  DashboardOutlined,
  UserOutlined,
  PartitionOutlined,
  AppstoreOutlined,
  GlobalOutlined,
  MonitorOutlined,
  SettingOutlined,
  LogoutOutlined,
} from '@ant-design/icons'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth'

const { Header, Sider, Content } = Layout
const { Text } = Typography

const menuItems = [
  { key: '/', icon: <DashboardOutlined />, label: 'Dashboard' },
  { key: '/peers', icon: <UserOutlined />, label: 'WireGuard клиенты' },
  { key: '/routes', icon: <PartitionOutlined />, label: 'Маршрутизация' },
  { key: '/presets', icon: <AppstoreOutlined />, label: 'Пресеты' },
  { key: '/dns', icon: <GlobalOutlined />, label: 'DNS' },
  { key: '/monitoring', icon: <MonitorOutlined />, label: 'Мониторинг' },
  { key: '/settings', icon: <SettingOutlined />, label: 'Настройки' },
]

export default function AppLayout() {
  const [collapsed, setCollapsed] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const { session, logout } = useAuth()

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider collapsible collapsed={collapsed} onCollapse={setCollapsed}>
        <div style={{ height: 32, margin: 16, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Text strong style={{ color: '#fff', fontSize: collapsed ? 14 : 16, whiteSpace: 'nowrap' }}>
            {collapsed ? 'ST' : 'SmartTraffic'}
          </Text>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header style={{ padding: '0 24px', background: '#fff', display: 'flex', justifyContent: 'flex-end', alignItems: 'center', gap: 16 }}>
          <Text>{session?.email}</Text>
          <Button icon={<LogoutOutlined />} onClick={handleLogout}>
            Выйти
          </Button>
        </Header>
        <Content style={{ margin: 24 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
