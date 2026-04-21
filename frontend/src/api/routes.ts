import client from './client'
import type { RoutingRule, RoutingRuleCreateRequest, RoutingRuleUpdateRequest, ReorderRequest } from '../types'

export async function listRules(): Promise<RoutingRule[]> {
  const res = await client.get<RoutingRule[]>('/routes')
  return res.data
}

export async function createRule(data: RoutingRuleCreateRequest): Promise<RoutingRule> {
  const res = await client.post<RoutingRule>('/routes', data)
  return res.data
}

export async function getRule(id: string): Promise<RoutingRule> {
  const res = await client.get<RoutingRule>(`/routes/${id}`)
  return res.data
}

export async function updateRule(id: string, data: RoutingRuleUpdateRequest): Promise<RoutingRule> {
  const res = await client.put<RoutingRule>(`/routes/${id}`, data)
  return res.data
}

export async function deleteRule(id: string): Promise<void> {
  await client.delete(`/routes/${id}`)
}

export async function reorderRules(data: ReorderRequest): Promise<void> {
  await client.put('/routes/reorder', data)
}
