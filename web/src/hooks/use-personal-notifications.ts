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
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

export interface PersonalNotification {
  id: number
  type: string
  quota: number
  threshold: number
  created_at: number
  read_at: number
}

export function usePersonalNotifications() {
  const userId = useAuthStore((state) => state.auth.user?.id)
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const queryKey = ['user-notifications', userId]
  const inbox = useInfiniteQuery({
    queryKey,
    enabled: !!userId,
    initialPageParam: 0,
    queryFn: async ({ pageParam, signal }) => {
      const response = await api.get<{
        success: boolean
        data: { items: PersonalNotification[]; unread_count: number }
      }>('/api/user/notifications', {
        params: pageParam ? { before_id: pageParam } : undefined,
        signal,
        skipErrorHandler: true,
        skipBusinessError: true,
      })
      if (!response.data.success) {
        throw new Error('Failed to load notifications')
      }
      return response.data.data
    },
    getNextPageParam: (page) =>
      page.items.length === 50 ? page.items.at(-1)?.id : undefined,
    refetchInterval: 60_000,
    staleTime: 30_000,
    retry: false,
  })
  const read = useMutation({
    mutationFn: async (id: number) => {
      const response = await api.post(
        `/api/user/notifications/${id}/read`,
        undefined,
        { skipErrorHandler: true, skipBusinessError: true }
      )
      if (!response.data.success) throw new Error('Failed to mark notification')
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey }),
    onError: () => toast.error(t('Failed to mark message as read')),
  })

  if (!userId) return undefined
  return {
    items: inbox.data?.pages.flatMap((page) => page.items) ?? [],
    unreadCount: inbox.data?.pages[0]?.unread_count ?? 0,
    loading: inbox.isLoading,
    error: inbox.isError,
    refetch: () => void inbox.refetch(),
    markRead: (id: number) => read.mutate(id),
    markingRead: read.isPending,
    hasMore: inbox.hasNextPage,
    loadingMore: inbox.isFetchingNextPage,
    loadMore: () => void inbox.fetchNextPage(),
  }
}

export type PersonalInbox = NonNullable<
  ReturnType<typeof usePersonalNotifications>
>
