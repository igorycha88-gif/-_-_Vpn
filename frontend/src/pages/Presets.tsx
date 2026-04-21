import { Card, List, Button, Tag, Popconfirm, Typography, Spin, Alert } from 'antd'
import { CheckOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { usePresets, useApplyPreset } from '../hooks/usePresets'
import type { Preset } from '../types'

const { Text, Paragraph } = Typography

export default function Presets() {
  const { data: presets, isLoading, error } = usePresets()
  const applyMutation = useApplyPreset()

  if (error) return <Alert type="error" message="Ошибка загрузки пресетов" />

  return (
    <div>
      <h2>Пресеты маршрутизации</h2>
      <Spin spinning={isLoading}>
        <List
          grid={{ gutter: 16, xs: 1, sm: 2, lg: 3 }}
          dataSource={presets ?? []}
          renderItem={(preset: Preset) => (
            <List.Item>
              <Card
                title={preset.name}
                extra={
                  preset.is_builtin ? (
                    <Tag color="blue">Встроенный</Tag>
                  ) : (
                    <Tag>Пользовательский</Tag>
                  )
                }
                actions={[
                  <Popconfirm
                    key="apply"
                    title="Применить пресет? Текущие правила будут заменены."
                    onConfirm={() => applyMutation.mutate(preset.id)}
                  >
                    <Button
                      type="primary"
                      icon={<ThunderboltOutlined />}
                      loading={applyMutation.isPending}
                    >
                      Применить
                    </Button>
                  </Popconfirm>,
                ]}
              >
                <Paragraph>{preset.description || 'Без описания'}</Paragraph>
                <Text type="secondary">
                  <CheckOutlined /> {JSON.parse(preset.rules).length} правил
                </Text>
              </Card>
            </List.Item>
          )}
        />
      </Spin>
    </div>
  )
}
