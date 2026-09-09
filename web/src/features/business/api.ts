/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

import {
  getBusinessItems,
  type ApiResponse,
  type BusinessAnnouncement,
  type BusinessChannelHealth,
  type BusinessCompany,
  type BusinessOperationsOverview,
  type BusinessPage,
  type BusinessProject,
  type PlatformReadOnlyUser,
  type PlatformReadOnlyLedgerEntry,
  type PlatformReadOnlyConsumption,
  type PlatformReadOnlyAdjustment,
  type BusinessAuditEvent,
  type SalesCustomer,
  type SalesCustomerSummary,
  type SalesUsageLog,
  type BusinessFollowUp,
  type SalesProjectReminder,
} from './types'

function responseData<T>(response: ApiResponse<T>): T {
  if (!response.success || response.data === undefined) {
    throw new Error(response.message || 'Business request failed')
  }
  return response.data
}

export async function getSalesCustomerLogs(
  companyId: number,
  startAt?: number,
  endAt?: number
): Promise<SalesUsageLog[]> {
  const response = await api.get<ApiResponse<BusinessPage<SalesUsageLog>>>(
    `/api/business/sales/customers/${companyId}/logs`,
    { params: { p: 1, page_size: 10, start_at: startAt, end_at: endAt } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getSalesCustomerLogsPage(
  companyId: number,
  params: {
    p?: number
    page_size?: number
    start_at?: number
    end_at?: number
  }
): Promise<{ items: SalesUsageLog[]; total: number }> {
  const response = await api.get<ApiResponse<BusinessPage<SalesUsageLog>>>(
    `/api/business/sales/customers/${companyId}/logs`,
    { params: { p: 1, page_size: 20, ...params } }
  )
  const data = responseData(response.data)
  return { items: getBusinessItems(data), total: data.total ?? 0 }
}

export async function getSalesCustomerFollowUps(
  companyId: number
): Promise<BusinessFollowUp[]> {
  const response = await api.get<ApiResponse<BusinessPage<BusinessFollowUp>>>(
    `/api/business/sales/customers/${companyId}/follow-ups`,
    { params: { p: 1, page_size: 50 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function createSalesCustomerFollowUp(
  companyId: number,
  payload: { title: string; note?: string; due_at?: number }
): Promise<BusinessFollowUp> {
  const response = await api.post<ApiResponse<BusinessFollowUp>>(
    `/api/business/sales/customers/${companyId}/follow-ups`,
    payload
  )
  return responseData(response.data)
}

export async function updateSalesCustomerFollowUp(
  followUpId: number,
  payload: { title: string; note?: string; status: 'open' | 'done'; due_at?: number }
): Promise<BusinessFollowUp> {
  const response = await api.put<ApiResponse<BusinessFollowUp>>(
    `/api/business/sales/follow-ups/${followUpId}`,
    payload
  )
  return responseData(response.data)
}

export async function getSalesCustomerReminders(): Promise<SalesProjectReminder[]> {
  const response = await api.get<ApiResponse<BusinessPage<SalesProjectReminder>>>(
    '/api/business/sales/reminders',
    { params: { p: 1, page_size: 50 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getBusinessCompanies(): Promise<BusinessCompany[]> {
  const response = await api.get<ApiResponse<BusinessPage<BusinessCompany>>>(
    '/api/business/companies',
    { params: { p: 1, page_size: 25 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getCompanyProjects(
  companyId: number
): Promise<BusinessProject[]> {
  const response = await api.get<ApiResponse<BusinessProject[]>>(
    `/api/business/companies/${companyId}/projects`
  )
  return responseData(response.data)
}

export async function getSalesCustomers(): Promise<SalesCustomer[]> {
  const response = await api.get<ApiResponse<BusinessPage<SalesCustomer>>>(
    '/api/business/sales/customers',
    { params: { p: 1, page_size: 25 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getSalesInvitationCode(): Promise<string> {
  const response = await api.get<ApiResponse<string>>('/api/user/aff')
  return responseData(response.data)
}

export async function getSalesCustomerSummary(
  companyId: number
): Promise<SalesCustomerSummary> {
  const response = await api.get<ApiResponse<SalesCustomerSummary>>(
    `/api/business/sales/customers/${companyId}/summary`
  )
  return responseData(response.data)
}

export async function getBusinessOperationsOverview(): Promise<BusinessOperationsOverview> {
  const response = await api.get<ApiResponse<BusinessOperationsOverview>>(
    '/api/business/operations/overview'
  )
  return responseData(response.data)
}

export async function getPlatformReadOnlyUsers(): Promise<PlatformReadOnlyUser[]> {
  const response = await api.get<ApiResponse<BusinessPage<PlatformReadOnlyUser>>>(
    '/api/business/operations/users',
    { params: { p: 1, page_size: 10 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getPlatformReadOnlyLedger(): Promise<PlatformReadOnlyLedgerEntry[]> {
  const response = await api.get<ApiResponse<BusinessPage<PlatformReadOnlyLedgerEntry>>>(
    '/api/business/operations/ledger',
    { params: { p: 1, page_size: 10 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getPlatformReadOnlyConsumptions(): Promise<PlatformReadOnlyConsumption[]> {
  const response = await api.get<ApiResponse<BusinessPage<PlatformReadOnlyConsumption>>>(
    '/api/business/operations/consumption',
    { params: { p: 1, page_size: 10 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getPlatformReadOnlyAdjustments(): Promise<PlatformReadOnlyAdjustment[]> {
  const response = await api.get<ApiResponse<BusinessPage<PlatformReadOnlyAdjustment>>>(
    '/api/business/operations/adjustments',
    { params: { p: 1, page_size: 10 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getBusinessAuditEvents(): Promise<BusinessAuditEvent[]> {
  const response = await api.get<ApiResponse<BusinessPage<BusinessAuditEvent>>>(
    '/api/business/operations/audit',
    { params: { p: 1, page_size: 10 } }
  )
  return getBusinessItems(responseData(response.data))
}

export async function getBusinessChannelHealth(): Promise<
  BusinessChannelHealth[]
> {
  const response = await api.get<ApiResponse<BusinessChannelHealth[]>>(
    '/api/business/operations/channel-health'
  )
  return responseData(response.data)
}

export async function createBusinessProject(
  companyId: number,
  payload: BusinessProjectMutation
) {
  const response = await api.post<ApiResponse<unknown>>('/api/business/projects', {
    ...payload,
    company_id: companyId,
  })
  return response.data
}

export async function updateBusinessProject(
  projectId: number,
  payload: BusinessProjectMutation
) {
  const response = await api.put<ApiResponse<unknown>>(
    `/api/business/projects/${projectId}`,
    payload
  )
  return response.data
}

export async function assignBusinessProjectToken(
  projectId: number,
  tokenId: number,
  reason?: string
) {
  const response = await api.post<ApiResponse<unknown>>(
    `/api/business/projects/${projectId}/tokens/${tokenId}`,
    reason ? { reason } : {}
  )
  return response.data
}

export type BusinessProjectSummary = {
  project: unknown
  balance: number
  token_count: number
  consumed_quota: number
  request_count: number
  error_count: number
  alerts: string[]
}

export async function getBusinessProjectSummary(
  projectId: number
): Promise<BusinessProjectSummary> {
  const response = await api.get<ApiResponse<BusinessProjectSummary>>(
    `/api/business/projects/${projectId}/summary`
  )
  return response.data.data as BusinessProjectSummary
}

export async function getBusinessProjectTokens(projectId: number) {
  const response = await api.get<ApiResponse<Array<{ id: number; name: string }>>>(
    `/api/business/projects/${projectId}/tokens`
  )
  return response.data.data
}

export type BusinessProjectMutation = {
  name: string
  budget_quota?: number
  low_balance_quota?: number
  model_limits?: string
  channel_limits?: string
  status?: number
  rate_limit_rpm?: number
  rate_limit_tpm?: number
  max_concurrent_requests?: number
  reason?: string
}

export async function getBusinessAnnouncements(): Promise<
  BusinessAnnouncement[]
> {
  const response = await api.get<ApiResponse<BusinessAnnouncement[]>>(
    '/api/business/operations/announcements'
  )
  return responseData(response.data)
}
