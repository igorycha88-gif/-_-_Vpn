import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { message } from 'antd'
import * as presetsApi from '../api/presets'

export function usePresets() {
  return useQuery({
    queryKey: ['presets'],
    queryFn: () => presetsApi.listPresets(),
  })
}

export function useApplyPreset() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => presetsApi.applyPreset(id),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['routes'] })
      message.success(`Пресет применён (${data.applied_rules} правил)`)
    },
    onError: () => {
      message.error('Ошибка применения пресета')
    },
  })
}
