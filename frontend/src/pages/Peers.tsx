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
import type { Peer, PeerCreateRequest, DeviceType, ConfigMode } from '../types'
import { formatBytes } from '../utils/format'
import { downloadWithAuth } from '../utils/download'

const { Text } = Typography

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
    downloadWithAuth(`/api/v1/wg/peers/${peer.id}/config`, `${peer.name}.json`)
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
      title: 'Режим',
      dataIndex: 'config_mode',
      key: 'config_mode',
      render: (v: ConfigMode | undefined) => {
        const mode = v ?? 'tun'
        return mode === 'proxy'
          ? <Tag color="orange">Proxy · Cisco-safe</Tag>
          : <Tag color="purple">TUN · весь трафик</Tag>
      },
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
      render: (_: unknown, record: Peer) => {
        if (!record.is_active) {
          return <Tag color="red">Отключён</Tag>
        }
        const online = record.last_seen && (Date.now() - new Date(record.last_seen).getTime()) < 120_000
        return online
          ? <Tag color="green">Онлайн</Tag>
          : <Tag color="default">Активен</Tag>
      },
    },
    {
      title: 'Трафик RX ↓ / TX ↑',
      key: 'traffic',
      render: (_: unknown, record: Peer) => {
        const total = record.total_rx + record.total_tx
        return (
          <div>
            <div><Text type="secondary" style={{ fontSize: 12 }}>↓ {formatBytes(record.total_rx)}</Text></div>
            <div><Text type="secondary" style={{ fontSize: 12 }}>↑ {formatBytes(record.total_tx)}</Text></div>
            <div><Text style={{ fontSize: 11 }}>Σ {formatBytes(total)}</Text></div>
          </div>
        )
      },
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
        <Form form={form} layout="vertical" onFinish={handleCreate} initialValues={{ device_type: 'iphone', config_mode: 'tun' }}>
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
          <Form.Item
            name="config_mode"
            label="Режим конфигурации"
            tooltip="TUN — весь трафик устройства (ломает Cisco AnyConnect). Proxy — мирный с Cisco, но только TCP (отключите QUIC в браузере)."
          >
            <Select placeholder="Выберите режим">
              <Select.Option value="tun">TUN — весь трафик (без Cisco)</Select.Option>
              <Select.Option value="proxy">Proxy — Cisco-safe (масOS)</Select.Option>
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
