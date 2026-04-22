import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { message } from 'antd'
import * as dnsApi from '../api/dns'
import type { DNSSettingsUpdateRequest } from '../types'

export function useDnsSettings() {
  return useQuery({
    queryKey: ['dns'],
    queryFn: () => dnsApi.getDnsSettings(),
  })
}

export function useUpdateDnsSettings() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: DNSSettingsUpdateRequest) => dnsApi.updateDnsSettings(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['dns'] })
      message.success('DNS настройки обновлены')
    },
    onError: () => {
      message.error('Ошибка обновления DNS настроек')
    },
  })
}

export function useDnsPresets() {
  return useQuery({
    queryKey: ['dns-presets'],
    queryFn: () => dnsApi.listDnsPresets(),
    staleTime: Infinity,
  })
}
