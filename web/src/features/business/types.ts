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

export const BUSINESS_PERMISSION_ACTIONS = {
  READ: 'read',
} as const

export const BUSINESS_PERMISSION_RESOURCES = {
  COMPANY: 'business_company',
  PROJECT: 'business_project',
  SALES: 'business_sales',
  REPORT: 'business_report',
  OPERATIONS: 'business_operations',
  FINANCE: 'finance',
} as const

export const BUSINESS_WORKBENCH_READ_RESOURCES = [
  BUSINESS_PERMISSION_RESOURCES.COMPANY,
  BUSINESS_PERMISSION_RESOURCES.PROJECT,
  BUSINESS_PERMISSION_RESOURCES.SALES,
  BUSINESS_PERMISSION_RESOURCES.REPORT,
  BUSINESS_PERMISSION_RESOURCES.OPERATIONS,
  BUSINESS_PERMISSION_RESOURCES.FINANCE,
] as const

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
}

export type BusinessPage<T> = {
  items?: T[]
  records?: T[]
  total?: number
}

export type BusinessCompany = {
  id: number
  name: string
  credit_code: string
  contact_name: string
  owner_user_id: number
}

export type BusinessProject = {
  id: number
  company_id: number
  name: string
  owner_user_id: number
  budget_quota: number
  low_balance_quota: number
  model_limits: string
  channel_limits: string
  status: number
}

export type SalesCustomer = {
  id: number
  name: string
}

export type SalesProject = {
  id: number
  name: string
  budget_quota: number
  low_balance_quota: number
  status: number
}

export type SalesCustomerSummary = {
  customer: SalesCustomer
  balance: number | null
  balance_available: boolean
  projects: SalesProject[]
  usage: {
    last_7_days_quota: number
    last_30_days_quota: number
    last_30_days_calls: number
    last_30_days_credit: number
    new_projects_30_days: number
  }
  top_models: Array<{
    model_name: string
    quota: number
    calls: number
  }>
  alerts: {
    low_balance_project_count: number
  }
}

export type SalesUsageLog = {
  id: number
  created_at: number
  type: number
  model_name: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  use_time: number
  is_stream: boolean
  token_identifier: string
  request_id: string
}

export type BusinessFollowUp = {
  id: number
  company_id: number
  customer_user_id: number
  sales_user_id: number
  title: string
  note: string
  status: 'open' | 'done'
  due_at: number
  created_at: number
  updated_at: number
}

export type SalesProjectReminder = {
  id: number
  company_id: number
  project_id: number
  owner_user_id: number
  reminder_type: string
  state: string
  title: string
  content: string
  metric_value: number
  threshold_value: number
  first_detected_at: number
  last_detected_at: number
  resolved_at: number
}

export type BusinessOperationsOverview = {
  company_count: number
  project_count: number
  pending_adjustment_count: number
  request_count: number
  error_count: number
  consumed_quota: number
  enabled_channel_count: number
  degraded_channel_count: number
  cost_ratio: number
  revenue_quota: number
  cost_quota: number
  gross_margin_quota: number
  top_models: Array<{ model_name: string; quota: number; calls: number }>
  top_companies: Array<{ model_name: string; quota: number; calls: number }>
}

export type PlatformReadOnlyUser = {
  id: number
  username: string
  display_name: string
  quota: number
  used_quota: number
  request_count: number
  status: number
  created_at: number
}

export type PlatformReadOnlyLedgerEntry = {
  id: number
  company_id: number
  user_id: number
  amount: number
  entry_type: string
  created_at: number
}

export type PlatformReadOnlyConsumption = {
  id: number
  company_id: number
  project_id: number
  model_name: string
  quota: number
  channel_id: number
  created_at: number
}

export type PlatformReadOnlyAdjustment = {
  id: number
  company_id: number
  user_id: number
  amount: number
  entry_type: string
  status: string
  created_at: number
}

export type BusinessAuditEvent = {
  id: number
  action: string
  resource: string
  resource_id: number
  reason: string
  actor_username: string
  actor_ip: string
  created_at: number
}

export type BusinessChannelHealth = {
  id: number
  status: number
  response_time: number
  test_time: number
}

export type BusinessAnnouncement = {
  id: number
  title: string
  content: string
  status: string
  starts_at: number
  ends_at: number
}

export function getBusinessItems<T>(
  page: BusinessPage<T> | T[] | undefined
): T[] {
  if (!page) return []
  if (Array.isArray(page)) return page
  return page.items ?? page.records ?? []
}
