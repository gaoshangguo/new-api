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
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { usePersonalNotifications } from '@/hooks/use-personal-notifications'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

function createWrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return {
    client,
    wrapper: ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  }
}

afterEach(() => {
  useAuthStore.getState().auth.reset()
})

describe('personal notification account isolation', () => {
  it('does not request a personal inbox while logged out', () => {
    useAuthStore.getState().auth.reset()
    const get = vi.spyOn(api, 'get')
    const { wrapper } = createWrapper()
    const { result } = renderHook(usePersonalNotifications, { wrapper })
    expect(result.current).toBeUndefined()
    expect(get).not.toHaveBeenCalled()
  })

  it('hides the previous inbox immediately when the account changes or logs out', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            items: [
              {
                id: 10,
                type: 'low_balance',
                quota: 19,
                threshold: 20,
                created_at: 100,
                read_at: 0,
              },
            ],
            unread_count: 1,
          },
        },
      })
      .mockResolvedValueOnce({
        data: { success: true, data: { items: [], unread_count: 0 } },
      })
    const { wrapper } = createWrapper()
    const { result } = renderHook(usePersonalNotifications, { wrapper })
    await waitFor(() => expect(result.current?.items).toHaveLength(1))
    act(() =>
      useAuthStore.getState().auth.setUser({ id: 2, username: 'bob', role: 1 })
    )
    expect(result.current?.items).toEqual([])
    await waitFor(() => expect(result.current?.loading).toBe(false))
    expect(result.current?.unreadCount).toBe(0)
    act(() => useAuthStore.getState().auth.reset())
    expect(result.current).toBeUndefined()
  })

  it('keeps a message unread when saving read status fails', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
    vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: {
          items: [
            {
              id: 10,
              type: 'low_balance',
              quota: 19,
              threshold: 20,
              created_at: 100,
              read_at: 0,
            },
          ],
          unread_count: 1,
        },
      },
    })
    vi.spyOn(api, 'post').mockRejectedValue(new Error('offline'))
    const error = vi.spyOn(toast, 'error')
    const { wrapper } = createWrapper()
    const { result } = renderHook(usePersonalNotifications, { wrapper })
    await waitFor(() => expect(result.current?.items).toHaveLength(1))
    act(() => result.current?.markRead(10))
    await waitFor(() =>
      expect(error).toHaveBeenCalledWith('Failed to mark message as read')
    )
    expect(result.current?.unreadCount).toBe(1)
    expect(result.current?.items[0]?.read_at).toBe(0)
  })

  it('refreshes the unread count only after the server saves read status', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
    const message = {
      id: 10,
      type: 'low_balance',
      quota: 19,
      threshold: 20,
      created_at: 100,
      read_at: 0,
    }
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce({
        data: { success: true, data: { items: [message], unread_count: 1 } },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: { items: [{ ...message, read_at: 200 }], unread_count: 0 },
        },
      })
    vi.spyOn(api, 'post').mockResolvedValue({ data: { success: true } })
    const { wrapper } = createWrapper()
    const { result } = renderHook(usePersonalNotifications, { wrapper })
    await waitFor(() => expect(result.current?.unreadCount).toBe(1))
    act(() => result.current?.markRead(10))
    await waitFor(() => expect(result.current?.unreadCount).toBe(0))
    expect(result.current?.items[0]?.read_at).toBe(200)
  })

  it('exposes a retryable error rather than treating failed requests as an empty inbox', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
    vi.spyOn(api, 'get').mockRejectedValue(new Error('offline'))
    const { wrapper } = createWrapper()
    const { result } = renderHook(usePersonalNotifications, { wrapper })
    await waitFor(() => expect(result.current?.error).toBe(true))
    expect(result.current?.items).toEqual([])
  })
})
