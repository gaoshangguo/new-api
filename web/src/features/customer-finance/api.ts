import { api } from '@/lib/api'

export type CustomerFinanceFilter = {
  project_id?: number
  token_id?: number
  model_name?: string
  start_at?: number
  end_at?: number
}

export type BalanceLedgerRecord = {
  id: number
  project_id: number
  token_id: number
  amount: number
  entry_type: string
  balance_after: number
  created_at: number
}

export type ConsumptionRecord = {
  id: number
  project_id: number
  token_id: number
  quota: number
  model_name: string
  created_at: number
}

type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
}

type PageResult<T> = {
  items?: T[]
  records?: T[]
}

export async function getMyBalanceLedgers(
  filter: CustomerFinanceFilter
): Promise<ApiResponse<PageResult<BalanceLedgerRecord>>> {
  const response = await api.get('/api/business/self/ledger', {
    params: { ...filter, page_size: 50 },
  })
  return response.data
}

export async function getMyBusinessConsumptions(
  filter: CustomerFinanceFilter
): Promise<ApiResponse<PageResult<ConsumptionRecord>>> {
  const response = await api.get('/api/business/self/consumption', {
    params: { ...filter, page_size: 50 },
  })
  return response.data
}

export function pageItems<T>(page: PageResult<T> | undefined): T[] {
  if (!page) return []
  return page.items ?? page.records ?? []
}
