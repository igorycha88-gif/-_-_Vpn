import { Card, Form, Input, Switch, Button, Spin, Alert, message } from 'antd'
import { useDnsSettings, useUpdateDnsSettings } from '../hooks/useDns'
import type { DNSSettingsUpdateRequest } from '../types'

export default function DnsSettings() {
  const { data: settings, isLoading, error } = useDnsSettings()
  const updateMutation = useUpdateDnsSettings()
  const [form] = Form.useForm()

  if (error) return <Alert type="error" message="Ошибка загрузки DNS настроек" />
  if (isLoading) return <Spin size="large" style={{ display: 'block', margin: '40px auto' }} />

  const handleSave = async (values: DNSSettingsUpdateRequest) => {
    await updateMutation.mutateAsync(values)
    message.success('DNS настройки сохранены')
  }

  return (
    <div>
      <h2>DNS настройки</h2>
      <Card style={{ maxWidth: 600 }}>
        <Form
          form={form}
          layout="vertical"
          initialValues={settings}
          onFinish={handleSave}
        >
          <Form.Item
            name="upstream_ru"
            label="DNS для российских доменов"
            rules={[{ required: true, message: 'Обязательное поле' }]}
          >
            <Input placeholder="77.88.8.8,77.88.8.1" />
          </Form.Item>
          <Form.Item
            name="upstream_foreign"
            label="DNS для зарубежных доменов"
            rules={[{ required: true, message: 'Обязательное поле' }]}
          >
            <Input placeholder="1.1.1.1,8.8.8.8" />
          </Form.Item>
          <Form.Item name="block_ads" label="Блокировать рекламу" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={updateMutation.isPending}>
              Сохранить
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
