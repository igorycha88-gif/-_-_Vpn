import { useState } from 'react'
import {
  Table,
  Button,
  Modal,
  Form,
  Input,
  Tag,
  Space,
  Switch,
  message,
  Popconfirm,
  Typography,
  Select,
} from 'antd'
import {
  PlusOutlined,
  DeleteOutlined,
  QrcodeOutlined,
  DownloadOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import { usePeers, useCreatePeer, useDeletePeer, useTogglePeer } from '../hooks/usePeers'
import QrModal from '../components/QrModal'
import type { Peer, PeerCreateRequest, DeviceType } from '../types'

const { Text } = Typography

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`
}

export default function Peers() {
  const { data: peers, isLoading, refetch } = usePeers()
  const createMutation = useCreatePeer()
  const deleteMutation = useDeletePeer()
  const toggleMutation = useTogglePeer()
  const [createOpen, setCreateOpen] = useState(false)
  const [qrPeer, setQrPeer] = useState<Peer | null>(null)
  const [form] = Form.useForm()

  const handleCreate = async (values: PeerCreateRequest) => {
    await createMutation.mutateAsync(values)
    setCreateOpen(false)
    form.resetFields()
  }

  const handleDownloadConfig = (peer: Peer) => {
    const token = localStorage.getItem('smarttraffic_tokens')
    let authParam = ''
    if (token) {
      try {
        const parsed = JSON.parse(token)
        authParam = `?token=${parsed.accessToken}`
      } catch {
        // ignore
      }
    }
    const link = document.createElement('a')
    link.href = `/api/v1/wg/peers/${peer.id}/config${authParam}`
    link.download = `${peer.name}.json`
    link.click()
  }

  const handleToggle = async (peer: Peer) => {
    await toggleMutation.mutateAsync({ id: peer.id, active: !peer.is_active })
    message.success(peer.is_active ? 'Клиент отключён' : 'Клиент включён')
  }

  const columns = [
    {
      title: 'Имя',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: Peer) => (
        <Space>
          <Text strong>{name}</Text>
          {record.email && <Text type="secondary">({record.email})</Text>}
        </Space>
      ),
    },
    {
      title: 'Устройство',
      dataIndex: 'device_type',
      key: 'device_type',
      render: (v: DeviceType) => (
        <Tag color={v === 'iphone' ? 'blue' : 'green'}>
          {v === 'iphone' ? 'iPhone' : 'Android'}
        </Tag>
      ),
    },
    {
      title: 'UUID',
      dataIndex: 'public_key',
      key: 'public_key',
      render: (v: string) => v ? `${v.slice(0, 8)}…` : '—',
    },
    {
      title: 'Статус',
      key: 'status',
      render: (_: unknown, record: Peer) => (
        <Tag color={record.is_active ? 'green' : 'red'}>
          {record.is_active ? 'Активен' : 'Отключён'}
        </Tag>
      ),
    },
    {
      title: 'Трафик (RX/TX)',
      key: 'traffic',
      render: (_: unknown, record: Peer) => `${formatBytes(record.total_rx)} / ${formatBytes(record.total_tx)}`,
    },
    {
      title: 'Последняя активность',
      dataIndex: 'last_seen',
      key: 'last_seen',
      render: (v: string | null) => v ? new Date(v).toLocaleString('ru') : '—',
    },
    {
      title: 'Действия',
      key: 'actions',
      render: (_: unknown, record: Peer) => (
        <Space>
          <Switch
            size="small"
            checked={record.is_active}
            onChange={() => handleToggle(record)}
          />
          <Button
            size="small"
            icon={<QrcodeOutlined />}
            onClick={() => setQrPeer(record)}
          />
          <Button
            size="small"
            icon={<DownloadOutlined />}
            onClick={() => handleDownloadConfig(record)}
          />
          <Popconfirm
            title="Удалить клиента?"
            onConfirm={() => deleteMutation.mutate(record.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>VLESS клиенты</h2>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>Обновить</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
            Добавить
          </Button>
        </Space>
      </div>

      <Table
        dataSource={peers ?? []}
        columns={columns}
        rowKey="id"
        loading={isLoading}
        pagination={{ pageSize: 20 }}
      />

      <Modal
        title="Новый клиент VLESS"
        open={createOpen}
        onCancel={() => { setCreateOpen(false); form.resetFields() }}
        onOk={() => form.submit()}
        confirmLoading={createMutation.isPending}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={handleCreate} initialValues={{ device_type: 'iphone' }}>
          <Form.Item name="name" label="Имя" rules={[{ required: true, message: 'Обязательное поле' }]}>
            <Input placeholder="Имя устройства" />
          </Form.Item>
          <Form.Item name="email" label="Email">
            <Input placeholder="user@example.com" />
          </Form.Item>
          <Form.Item name="device_type" label="Тип устройства" rules={[{ required: true, message: 'Выберите тип устройства' }]}>
            <Select placeholder="Выберите устройство">
              <Select.Option value="iphone">iPhone</Select.Option>
              <Select.Option value="android">Android</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      <QrModal
        open={!!qrPeer}
        peerId={qrPeer?.id ?? null}
        peerName={qrPeer?.name ?? ''}
        onClose={() => setQrPeer(null)}
      />
    </div>
  )
}
