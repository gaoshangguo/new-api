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
import type { TFunction } from 'i18next'
import { z } from 'zod'

import { MANUAL_ADJUSTMENT_ENTRY_TYPES } from '../types'

export function getManualCreditFormSchema(t: TFunction) {
  return z.object({
    entry_type: z.enum(MANUAL_ADJUSTMENT_ENTRY_TYPES, {
      error: t('Select an adjustment type'),
    }),
    user_id: z.number().int().positive(t('Enter a valid customer user ID')),
    project_id: z
      .number()
      .int()
      .positive(t('Enter a valid project ID'))
      .optional(),
    amount: z.number().int().positive(t('Amount must be greater than 0')),
    external_reference: z
      .string()
      .trim()
      .min(1, t('External reference is required')),
    reason: z.string().trim().min(1, t('Reason is required')),
    note: z.string().trim().min(1, t('Note is required')),
    invoice_number: z.string().trim().optional(),
    external_finance_reference: z.string().trim().optional(),
    reconciliation_conclusion: z.string().trim().optional(),
  })
}

export type ManualCreditFormValues = z.infer<
  ReturnType<typeof getManualCreditFormSchema>
>

export const MANUAL_CREDIT_FORM_DEFAULT_VALUES: ManualCreditFormValues = {
  entry_type: 'manual_credit',
  user_id: undefined as unknown as number,
  project_id: undefined,
  amount: undefined as unknown as number,
  external_reference: '',
  reason: '',
  note: '',
  invoice_number: '',
  external_finance_reference: '',
  reconciliation_conclusion: '',
}
