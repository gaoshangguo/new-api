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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

import { getSalesCustomerReminders } from '../api'
import type { SalesProjectReminder } from '../types'

const REMINDER_TYPE_LABELS: Record<string, string> = {
  low_balance: 'Low balance',
  budget_exceeded: 'Budget exceeded',
  high_failure_rate: 'High failure rate',
  inactive_project: 'Inactive project',
}

function reminderTypeLabel(type: string, t: (key: string) => string): string {
  const label = REMINDER_TYPE_LABELS[type]
  return label ? t(label) : type
}

export function SalesReminders() {
  const { t } = useTranslation()
  const remindersQuery = useQuery({
    queryKey: ['business', 'sales', 'reminders'],
    queryFn: getSalesCustomerReminders,
    staleTime: 60 * 1000,
  })

  if (remindersQuery.isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('Sales reminders')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Skeleton className='h-20 w-full' />
        </CardContent>
      </Card>
    )
  }

  const reminders: SalesProjectReminder[] = remindersQuery.data ?? []

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Sales reminders')}</CardTitle>
      </CardHeader>
      <CardContent>
        {reminders.length === 0 ? (
          <p className='text-muted-foreground text-sm'>{t('No active reminders.')}</p>
        ) : (
          <div className='space-y-2'>
            {reminders.map((reminder) => (
              <div
                key={reminder.id}
                className='rounded-lg border px-3 py-2 text-sm'
              >
                <div className='flex items-center justify-between gap-2'>
                  <p className='font-medium'>{reminder.title}</p>
                  <span className='text-muted-foreground shrink-0 text-xs'>
                    {reminderTypeLabel(reminder.reminder_type, t)}
                  </span>
                </div>
                {reminder.content && (
                  <p className='text-muted-foreground mt-1 text-xs line-clamp-2'>
                    {reminder.content}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
