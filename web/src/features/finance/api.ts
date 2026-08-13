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
  ApiResponse,
  BalanceLedger,
  CreateManualCreditPayload,
  ManualCreditList,
  ManualCreditRequest,
} from './types'

const manualCreditsPath = '/api/business/finance/manual-credits'

export async function getManualCreditRequests(): Promise<
  ApiResponse<ManualCreditList>
> {
  const response = await api.get(manualCreditsPath, {
    params: { status: '' },
  })
  return response.data
}

export async function createManualCreditRequest(
  payload: CreateManualCreditPayload
): Promise<ApiResponse<ManualCreditRequest>> {
  const response = await api.post(manualCreditsPath, payload)
  return response.data
}

export async function approveManualCreditRequest(
  id: number
): Promise<ApiResponse<BalanceLedger>> {
  const response = await api.post(`${manualCreditsPath}/${id}/approve`)
  return response.data
}

export async function approveEscalatedManualCreditRequest(
  id: number,
  reason: string
): Promise<ApiResponse<BalanceLedger>> {
  const response = await api.post(`${manualCreditsPath}/${id}/approve-escalation`, {
    reason,
  })
  return response.data
}

export async function rejectManualCreditRequest(
  id: number,
  reason: string
): Promise<ApiResponse<ManualCreditRequest>> {
  const response = await api.post(`${manualCreditsPath}/${id}/reject`, {
    reason,
  })
  return response.data
}

export type BalanceLedgerExportFilters = {
  user_id?: number
  project_id?: number
  token_id?: number
  entry_type?: string
  model_name?: string
  start_at?: number
  end_at?: number
}

export async function exportBalanceLedgers(
  filters: BalanceLedgerExportFilters
): Promise<Blob> {
  const response = await api.get('/api/business/finance/ledger/export', {
    responseType: 'blob',
    params: filters,
  })
  return response.data
}

export async function exportBusinessConsumptions(
  filters: BalanceLedgerExportFilters
): Promise<Blob> {
  const response = await api.get('/api/business/finance/consumption/export', {
    responseType: 'blob',
    params: filters,
  })
  return response.data
}

// 销售导出（P0-20）：强制按归属公司限定，服务端以 company_id 圈定数据范围。
export async function exportSalesCustomerLedgers(
  companyId: number,
  filters: BalanceLedgerExportFilters
): Promise<Blob> {
  const response = await api.get(
    `/api/business/sales/customers/${companyId}/ledger/export`,
    {
      responseType: 'blob',
      params: filters,
    }
  )
  return response.data
}

export async function exportSalesCustomerConsumptions(
  companyId: number,
  filters: BalanceLedgerExportFilters
): Promise<Blob> {
  const response = await api.get(
    `/api/business/sales/customers/${companyId}/consumption/export`,
    {
      responseType: 'blob',
      params: filters,
    }
  )
  return response.data
}
