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
import { isAxiosError } from 'axios'

import { api } from '@/lib/api'

import { buildQueryParams } from './lib/query-params'
import type {
  GetLogsParams,
  GetLogsResponse,
  GetLogStatsParams,
  GetLogStatsResponse,
  GetMidjourneyLogsParams,
  GetTaskLogsParams,
  UserInfo,
} from './types'

// ============================================================================
// Generic API Helpers
// ============================================================================

function buildApiPath(endpoint: string, isAdmin: boolean): string {
  return isAdmin ? endpoint : `${endpoint}/self`
}

async function fetchLogs<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogsResponse> {
  const paramRecord = params as unknown as Record<string, unknown>
  const queryParams = buildQueryParams({
    p: paramRecord.p || 1,
    page_size: paramRecord.page_size || 20,
    ...params,
  })
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}?${queryParams}`)
  return res.data
}

async function fetchLogStats<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogStatsResponse> {
  const queryParams = buildQueryParams(
    params as unknown as Record<string, unknown>
  )
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}/stat?${queryParams}`)
  return res.data
}

// ============================================================================
// Common Log APIs
// ============================================================================

export const getAllLogs = (params: GetLogsParams = {}) =>
  fetchLogs('/api/log', params, true)

export const getUserLogs = (
  params: Omit<GetLogsParams, 'username' | 'channel'> = {}
) => fetchLogs('/api/log', params, false)

export async function exportUsageLogs(
  params: GetLogsParams,
  isAdmin: boolean
): Promise<Blob> {
  const filters: Record<string, unknown> = { ...params }
  delete filters.p
  delete filters.page_size
  if (!isAdmin) {
    delete filters.username
    delete filters.channel
  }
  const path = `${buildApiPath('/api/log', isAdmin)}/export`
  let data: Blob
  let failureCode = 'LOG_EXPORT_FAILED'
  try {
    const response = await api.get<Blob>(
      `${path}?${buildQueryParams(filters)}`,
      {
        responseType: 'blob',
        skipErrorHandler: true,
        disableDuplicate: true,
      }
    )
    data = response.data
  } catch (error) {
    if (!isAxiosError(error) || !(error.response?.data instanceof Blob)) {
      throw Object.assign(new Error('Usage log export failed'), {
        code: 'LOG_EXPORT_FAILED',
      })
    }
    if (error.response.status === 429) failureCode = 'LOG_EXPORT_BUSY'
    data = error.response.data
  }
  if (
    data.type.split(';')[0] ===
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
  ) {
    return data
  }
  let code = failureCode
  try {
    const payload: unknown = JSON.parse(await data.text())
    if (
      payload &&
      typeof payload === 'object' &&
      'code' in payload &&
      typeof payload.code === 'string' &&
      payload.code.startsWith('LOG_EXPORT_')
    ) {
      code = payload.code
    }
  } catch {
    // A proxy/network failure must not be saved as an Excel file.
  }
  throw Object.assign(new Error('Usage log export failed'), { code })
}

export const getLogStats = (params: GetLogStatsParams = {}) =>
  fetchLogStats('/api/log', params, true)

export const getUserLogStats = (
  params: Omit<GetLogStatsParams, 'username' | 'channel'> = {}
) => fetchLogStats('/api/log', params, false)

export async function getUserInfo(
  userId: number
): Promise<{ success: boolean; message?: string; data?: UserInfo }> {
  const res = await api.get(`/api/user/${userId}`)
  return res.data
}

// ============================================================================
// MjProxy (Drawing) Logs API
// ============================================================================

export const getAllMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, true)

export const getUserMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, false)

// ============================================================================
// Task Logs API
// ============================================================================

export const getAllTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, true)

export const getUserTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, false)
