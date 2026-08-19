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
*/
import { api } from '@/lib/api'

export type UserDashboardSummary = {
  available_quota: number
  frozen_quota: number
  used_quota: number
  request_count: number
  today_used: number
  month_used: number
  error_rate_24h: number
  pending_items: Array<{
    id: number
    title: string
    content: string
    last_detected_at: number
  }>
}

export async function getSelfDashboardSummary() {
  const res = await api.get<{ success: boolean; data: UserDashboardSummary }>(
    '/api/business/self/dashboard'
  )
  return res.data.data
}

export type CompanyProfile = {
  id: number
  name: string
  credit_code: string
  contact_name: string
  contact_email: string
  contact_phone: string
  invoice_remark: string
  owner_user_id: number
}

export async function getMyCompany() {
  const res = await api.get<{ success: boolean; data: CompanyProfile | null }>(
    '/api/business/self/company'
  )
  return res.data.data
}

export async function createSelfCompany(profile: Partial<CompanyProfile>) {
  const res = await api.post<{ success: boolean; data: CompanyProfile }>(
    '/api/business/companies/self',
    profile
  )
  return res.data.data
}

export async function updateSelfCompany(
  id: number,
  profile: Partial<CompanyProfile>
) {
  const res = await api.put<{ success: boolean; data: CompanyProfile }>(
    `/api/business/companies/self/${id}`,
    profile
  )
  return res.data.data
}
