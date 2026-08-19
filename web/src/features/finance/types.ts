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

export const FINANCE_PERMISSION_RESOURCE = 'finance'

export const FINANCE_PERMISSION_ACTIONS = {
  READ: 'read',
  CREATE: 'create',
  APPROVE: 'approve',
} as const

export const FINANCE_MANUAL_CREDIT_QUERY_KEY = [
  'finance',
  'manual-credits',
  'pending',
] as const

export type ManualCreditStatus =
  | 'pending'
  | 'pending_escalation'
  | 'approved'
  | 'rejected'

export const MANUAL_ADJUSTMENT_ENTRY_TYPES = [
  'manual_credit',
  'manual_debit',
  'compensation',
  'freeze',
  'unfreeze',
] as const

export type ManualAdjustmentEntryType =
  (typeof MANUAL_ADJUSTMENT_ENTRY_TYPES)[number]

export type ManualCreditRequest = {
  id: number
  user_id: number
  project_id?: number | null
  entry_type: ManualAdjustmentEntryType
  amount: number
  external_reference: string
  reason: string
  note?: string
  invoice_number?: string
  external_finance_reference?: string
  reconciliation_conclusion?: string
  status: ManualCreditStatus
  created_by?: number
  approved_by?: number | null
  rejection_reason?: string
  duplicate_of_request_id?: number
  escalation_reason?: string
  created_at: number
  updated_at?: number
}

export type ApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}

export type ManualCreditList =
  | ManualCreditRequest[]
  | {
      items?: ManualCreditRequest[]
      records?: ManualCreditRequest[]
      total?: number
    }

export type CreateManualCreditPayload = {
  user_id: number
  project_id?: number
  entry_type: ManualAdjustmentEntryType
  amount: number
  external_reference: string
  reason: string
  note?: string
  invoice_number?: string
  external_finance_reference?: string
  reconciliation_conclusion?: string
}

export type BalanceLedger = {
  id: number
  company_id: number
  user_id: number
  project_id: number
  token_id: number
  request_id: string
  amount: number
  usage_quota: number
  funding_source: string
  balance_snapshot_available: boolean
  balance_before: number
  balance_after: number
  frozen_before: number
  frozen_after: number
  entry_type: ManualAdjustmentEntryType
  reference_type: string
  reference_id: number
  external_reference: string
  invoice_number: string
  external_finance_reference: string
  reconciliation_conclusion: string
  reason: string
  note: string
  created_by: number
  approved_by: number
  reverses_ledger_id: number
  created_at: number
}

export function getManualCreditItems(
  list: ManualCreditList | undefined
): ManualCreditRequest[] {
  if (!list) return []
  if (Array.isArray(list)) return list
  return list.items ?? list.records ?? []
}
