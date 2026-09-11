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

import type {
  SalesAccount,
  SalesAccountPage,
  SalesAccountPayload,
  SalesAccountProfilePayload,
} from './types'

type ApiEnvelope<T> = { success: boolean; message?: string; data?: T }

export const SALES_ACCOUNTS_QUERY_KEY = ['business', 'sales-accounts'] as const

export async function getSalesAccounts(): Promise<SalesAccount[]> {
  const response = await api.get<ApiEnvelope<SalesAccountPage>>(
    '/api/business/sales/accounts',
    { params: { p: 1, page_size: 100 } }
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to load sales accounts')
  }
  return response.data.data?.items ?? []
}

export async function createSalesAccount(
  payload: SalesAccountPayload
): Promise<void> {
  const response = await api.post<ApiEnvelope<{ user_id: number }>>(
    '/api/business/sales/accounts',
    payload
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to create sales account')
  }
}

export async function promoteUserToSalesAccount(
  userId: number,
  payload: SalesAccountProfilePayload
): Promise<void> {
  const response = await api.post<ApiEnvelope<{ user_id: number }>>(
    '/api/business/sales/accounts/promote',
    { user_id: userId, ...payload }
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to set sales account')
  }
}

export async function updateSalesAccountProfile(
  userId: number,
  payload: SalesAccountProfilePayload
): Promise<void> {
  const response = await api.put<ApiEnvelope<SalesAccount>>(
    `/api/business/sales/accounts/${userId}`,
    payload
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to update sales account')
  }
}

export async function setSalesAccountStatus(
  userId: number,
  status: number
): Promise<void> {
  const response = await api.put<ApiEnvelope<unknown>>(
    `/api/business/sales/accounts/${userId}/status`,
    { status }
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to update sales account')
  }
}
