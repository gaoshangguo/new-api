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
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import type { PersonalInbox } from '@/hooks/use-personal-notifications'
import { formatQuota } from '@/lib/format'

export function PersonalNotificationsContent({
  inbox,
}: {
  inbox: PersonalInbox
}) {
  const { t } = useTranslation()
  if (inbox.loading) return <p role='status'>{t('Loading...')}</p>
  if (inbox.error) {
    return (
      <div role='alert' className='space-y-2 p-3'>
        <p>{t('Failed to load personal messages')}</p>
        <Button size='sm' variant='outline' onClick={inbox.refetch}>
          {t('Retry')}
        </Button>
      </div>
    )
  }
  if (inbox.items.length === 0) {
    return (
      <p className='text-muted-foreground p-6 text-center text-sm'>
        {t('No personal messages')}
      </p>
    )
  }
  return (
    <ScrollArea className='h-80'>
      <ul className='space-y-2 pr-3'>
        {inbox.items.map((item) => (
          <li key={item.id} className='space-y-2 rounded-lg border p-3'>
            <div className='flex items-start justify-between gap-2'>
              <h3 className='text-sm font-medium'>
                {t('Low balance reminder')}
              </h3>
              {!item.read_at && (
                <span className='text-primary shrink-0 text-xs'>
                  {t('Unread')}
                </span>
              )}
            </div>
            <p className='text-muted-foreground text-sm break-words'>
              {t(
                'Your balance is {{balance}}, below your reminder threshold of {{threshold}}. Please top up.',
                {
                  balance: formatQuota(item.quota),
                  threshold: formatQuota(item.threshold),
                }
              )}
            </p>
            <time
              className='text-muted-foreground block text-xs'
              dateTime={new Date(item.created_at * 1000).toISOString()}
            >
              {new Date(item.created_at * 1000).toLocaleString()}
            </time>
            {!item.read_at && (
              <Button
                size='sm'
                variant='outline'
                disabled={inbox.markingRead}
                onClick={() => inbox.markRead(item.id)}
              >
                {t('Mark as read')}
              </Button>
            )}
          </li>
        ))}
      </ul>
      {inbox.hasMore && (
        <Button
          className='mt-3'
          size='sm'
          variant='outline'
          disabled={inbox.loadingMore}
          onClick={inbox.loadMore}
        >
          {t('Load more')}
        </Button>
      )}
    </ScrollArea>
  )
}
