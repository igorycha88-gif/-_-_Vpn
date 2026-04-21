import { useState, useCallback } from 'react'
import { Table, Button, Space, Tag, Popconfirm, message } from 'antd'
import {
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  ReloadOutlined,
  HolderOutlined,
} from '@ant-design/icons'
import { useRoutes, useDeleteRule, useReorderRules } from '../hooks/useRoutes'
import RuleEditor from '../components/RuleEditor'
import type { RoutingRule, RoutingRuleCreateRequest } from '../types'
import { useCreateRule, useUpdateRule } from '../hooks/useRoutes'

const ACTION_COLORS: Record<string, string> = {
  direct: 'green',
  proxy: 'blue',
  block: 'red',
}

const ACTION_LABELS: Record<string, string> = {
  direct: 'Напрямую',
  proxy: 'Прокси',
  block: 'Блок',
}

const TYPE_LABELS: Record<string, string> = {
  domain: 'Домен',
  domain_suffix: 'Суффикс',
  domain_keyword: 'Ключевое слово',
  ip: 'IP/CIDR',
  geoip: 'GeoIP',
  port: 'Порт',
  regex: 'Regex',
}

export default function RoutingRules() {
  const { data: rules, isLoading, refetch } = useRoutes()
  const createMutation = useCreateRule()
  const updateMutation = useUpdateRule()
  const deleteMutation = useDeleteRule()
  const reorderMutation = useReorderRules()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<RoutingRule | null>(null)

  const handleCreate = async (values: RoutingRuleCreateRequest) => {
    await createMutation.mutateAsync(values)
    setEditorOpen(false)
  }

  const handleEdit = (rule: RoutingRule) => {
    setEditingRule(rule)
    setEditorOpen(true)
  }

  const handleUpdate = async (values: RoutingRuleCreateRequest) => {
    if (!editingRule) return
    await updateMutation.mutateAsync({ id: editingRule.id, data: values })
    setEditorOpen(false)
    setEditingRule(null)
  }

  const handleToggleActive = async (rule: RoutingRule) => {
    await updateMutation.mutateAsync({
      id: rule.id,
      data: { is_active: !rule.is_active },
    })
    message.success(rule.is_active ? 'Правило отключено' : 'Правило включено')
  }

  const handleDragEnd = useCallback(
    (fromIndex: number, toIndex: number) => {
      if (!rules) return
      const newRules = [...rules]
      const [moved] = newRules.splice(fromIndex, 1)
      newRules.splice(toIndex, 0, moved)
      reorderMutation.mutate({ ids: newRules.map((r) => r.id) })
    },
    [rules, reorderMutation],
  )

  const columns = [
    {
      title: '',
      width: 40,
      render: () => <HolderOutlined style={{ cursor: 'grab' }} />,
    },
    {
      title: '#',
      dataIndex: 'priority',
      key: 'priority',
      width: 60,
    },
    {
      title: 'Название',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: 'Тип',
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => <Tag>{TYPE_LABELS[type] || type}</Tag>,
    },
    {
      title: 'Шаблон',
      dataIndex: 'pattern',
      key: 'pattern',
      render: (pattern: string) => <code>{pattern}</code>,
    },
    {
      title: 'Действие',
      dataIndex: 'action',
      key: 'action',
      render: (action: string) => (
        <Tag color={ACTION_COLORS[action]}>{ACTION_LABELS[action] || action}</Tag>
      ),
    },
    {
      title: 'Статус',
      dataIndex: 'is_active',
      key: 'is_active',
      render: (active: boolean, record: RoutingRule) => (
        <Tag
          color={active ? 'green' : 'default'}
          style={{ cursor: 'pointer' }}
          onClick={() => handleToggleActive(record)}
        >
          {active ? 'Вкл' : 'Выкл'}
        </Tag>
      ),
    },
    {
      title: 'Действия',
      key: 'actions',
      render: (_: unknown, record: RoutingRule) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          <Popconfirm title="Удалить правило?" onConfirm={() => deleteMutation.mutate(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2>Правила маршрутизации</h2>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => refetch()}>Обновить</Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => { setEditingRule(null); setEditorOpen(true) }}
          >
            Добавить
          </Button>
        </Space>
      </div>

      <Table
        dataSource={rules ?? []}
        columns={columns}
        rowKey="id"
        loading={isLoading}
        pagination={false}
        onRow={(_, index) => ({
          draggable: true,
          onDragStart: (e) => {
            e.dataTransfer.setData('text/plain', String(index ?? 0))
          },
          onDrop: (e) => {
            e.preventDefault()
            const fromIndex = Number(e.dataTransfer.getData('text/plain'))
            const toIndex = index ?? 0
            if (!isNaN(fromIndex) && fromIndex !== toIndex) {
              handleDragEnd(fromIndex, toIndex)
            }
          },
          onDragOver: (e) => e.preventDefault(),
        })}
      />

      <RuleEditor
        open={editorOpen}
        initialValues={editingRule ? {
          name: editingRule.name,
          type: editingRule.type,
          pattern: editingRule.pattern,
          action: editingRule.action,
          priority: editingRule.priority,
        } : undefined}
        onCancel={() => { setEditorOpen(false); setEditingRule(null) }}
        onSubmit={editingRule ? handleUpdate : handleCreate}
        loading={createMutation.isPending || updateMutation.isPending}
      />
    </div>
  )
}
