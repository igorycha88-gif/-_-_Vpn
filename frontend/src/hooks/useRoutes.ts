import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { message } from 'antd'
import * as routesApi from '../api/routes'
import type { RoutingRuleCreateRequest, RoutingRuleUpdateRequest, ReorderRequest } from '../types'

export function useRoutes() {
  return useQuery({
    queryKey: ['routes'],
    queryFn: () => routesApi.listRules(),
  })
}

export function useCreateRule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: RoutingRuleCreateRequest) => routesApi.createRule(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['routes'] })
      message.success('Правило создано')
    },
    onError: () => {
      message.error('Ошибка создания правила')
    },
  })
}

export function useUpdateRule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: RoutingRuleUpdateRequest }) =>
      routesApi.updateRule(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['routes'] })
      message.success('Правило обновлено')
    },
    onError: () => {
      message.error('Ошибка обновления правила')
    },
  })
}

export function useDeleteRule() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => routesApi.deleteRule(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['routes'] })
      message.success('Правило удалено')
    },
    onError: () => {
      message.error('Ошибка удаления правила')
    },
  })
}

export function useReorderRules() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: ReorderRequest) => routesApi.reorderRules(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['routes'] })
    },
  })
}
