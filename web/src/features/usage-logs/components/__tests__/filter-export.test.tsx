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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AxiosHeaders } from 'axios'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { usageLogsSearchSchema } from '../../lib/search-schema'
import { CommonLogsFilterBar } from '../common-logs-filter-bar'
import { UsageLogsProvider, useUsageLogsContext } from '../usage-logs-provider'

function FilterFixture() {
  const table = useReactTable({
    data: [],
    columns: [],
    getCoreRowModel: getCoreRowModel(),
    initialState: { columnFilters: [{ id: 'model_name', value: 'old-model' }] },
  })
  const context = useUsageLogsContext()
  return (
    <>
      <button type='button' onClick={() => context.setViewScope('self')}>
        Only Mine
      </button>
      <CommonLogsFilterBar table={table} />
    </>
  )
}

const previousUser = useAuthStore.getState().auth.user

beforeEach(() => {
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'admin', role: ROLE.ADMIN })
  class DownloadURL extends URL {
    static createObjectURL = vi.fn(() => 'blob:filtered-logs')
    static revokeObjectURL = vi.fn()
  }
  vi.stubGlobal('URL', DownloadURL)
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: String(url).includes('/export?')
      ? new Blob(['xlsx'], {
          type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
        })
      : { success: true, data: { quota: 0, rpm: 0, tpm: 0 } },
    status: 200,
    statusText: 'OK',
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  }))
})

afterEach(() => {
  useAuthStore.getState().auth.setUser(previousUser)
  vi.unstubAllGlobals()
})

async function renderFilters() {
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
    component: () => (
      <UsageLogsProvider>
        <Outlet />
      </UsageLogsProvider>
    ),
  })
  const logs = createRoute({
    getParentRoute: () => authenticated,
    path: '/usage-logs/$section',
    validateSearch: usageLogsSearchSchema,
    component: FilterFixture,
  })
  const router = createRouter({
    routeTree: root.addChildren([authenticated.addChildren([logs])]),
    history: createMemoryHistory({
      initialEntries: [
        '/usage-logs/common?model=old-model&type=%5B%222%22%5D&startTime=100000&endTime=200000&username=alice&channel=%227%22',
      ],
    }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByRole('button', { name: 'Export' })
}

test('exports the current draft filters without stale page filters overriding edited inputs', async () => {
  await renderFilters()
  fireEvent.change(screen.getByPlaceholderText('Model Name'), {
    target: { value: 'new-model' },
  })
  fireEvent.change(screen.getByPlaceholderText('Group'), {
    target: { value: 'vip' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Export' }))
  await waitFor(() => expect(URL.createObjectURL).toHaveBeenCalled())
  const request = vi
    .mocked(api.get)
    .mock.calls.find(([url]) => String(url).includes('/export?'))
  expect(request).toBeDefined()
  const url = new URL(String(request?.[0]), 'https://example.test')
  expect(url.pathname).toBe('/api/log/export')
  expect(Object.fromEntries(url.searchParams)).toEqual({
    type: '2',
    model_name: 'new-model',
    group: 'vip',
    username: 'alice',
    channel: '7',
    start_timestamp: '100',
    end_timestamp: '200',
  })
})

test('an administrator switching to Only Mine exports only personal logs', async () => {
  await renderFilters()
  fireEvent.click(screen.getByRole('button', { name: 'Only Mine' }))
  fireEvent.click(screen.getByRole('button', { name: 'Export' }))
  await waitFor(() => expect(URL.createObjectURL).toHaveBeenCalled())
  const request = vi
    .mocked(api.get)
    .mock.calls.find(([url]) => String(url).includes('/export?'))
  const url = new URL(String(request?.[0]), 'https://example.test')
  expect(url.pathname).toBe('/api/log/self/export')
  expect(url.searchParams.has('username')).toBe(false)
  expect(url.searchParams.has('channel')).toBe(false)
})
