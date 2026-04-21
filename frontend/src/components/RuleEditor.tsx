import { Modal, Form, Input, Select, InputNumber } from 'antd'
import type { RoutingRuleCreateRequest } from '../types'

const RULE_TYPES = [
  { value: 'domain', label: 'Домен' },
  { value: 'domain_suffix', label: 'Суффикс домена' },
  { value: 'domain_keyword', label: 'Ключевое слово' },
  { value: 'ip', label: 'IP / CIDR' },
  { value: 'geoip', label: 'GeoIP (код страны)' },
  { value: 'port', label: 'Порт' },
  { value: 'regex', label: 'Regex' },
]

const ACTIONS = [
  { value: 'direct', label: 'Напрямую' },
  { value: 'proxy', label: 'Через прокси' },
  { value: 'block', label: 'Блокировать' },
]

interface RuleEditorProps {
  open: boolean
  initialValues?: Partial<RoutingRuleCreateRequest>
  onCancel: () => void
  onSubmit: (values: RoutingRuleCreateRequest) => void
  loading?: boolean
}

export default function RuleEditor({ open, initialValues, onCancel, onSubmit, loading }: RuleEditorProps) {
  const [form] = Form.useForm()

  return (
    <Modal
      title={initialValues ? 'Редактирование правила' : 'Новое правило маршрутизации'}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={loading}
      destroyOnClose
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={initialValues}
        onFinish={onSubmit}
      >
        <Form.Item name="name" label="Название" rules={[{ required: true, message: 'Обязательное поле' }]}>
          <Input />
        </Form.Item>
        <Form.Item name="type" label="Тип" rules={[{ required: true, message: 'Выберите тип' }]}>
          <Select options={RULE_TYPES} />
        </Form.Item>
        <Form.Item name="pattern" label="Шаблон" rules={[{ required: true, message: 'Обязательное поле' }]}>
          <Input placeholder="например: .ru, 192.168.0.0/24, google.com" />
        </Form.Item>
        <Form.Item name="action" label="Действие" rules={[{ required: true, message: 'Выберите действие' }]}>
          <Select options={ACTIONS} />
        </Form.Item>
        <Form.Item name="priority" label="Приоритет">
          <InputNumber min={0} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Modal>
  )
}
