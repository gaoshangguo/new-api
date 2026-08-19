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
*/
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { formatQuota } from '@/lib/format'

import { getSelfDashboardSummary } from '../../api-self'

// P0-04 账户健康度：冻结余额、24 小时错误率与待办提醒。

function statCard(label: string, value: string, hint?: string) {
  return (
    <div className='rounded-xl border bg-background p-4'>
      <p className='text-muted-foreground text-xs'>{label}</p>
      <p className='mt-1 text-xl font-semibold'>{value}</p>
      {hint ? <p className='text-muted-foreground mt-1 text-xs'>{hint}</p> : null}
    </div>
  )
}

export function AccountHealthPanel() {
  const { t } = useTranslation()
  const { data } = useQuery({
    queryKey: ['dashboard', 'account-health'],
    queryFn: getSelfDashboardSummary,
    staleTime: 60 * 1000,
  })

  if (!data) return null

  const errorRatePercent = Math.round(data.error_rate_24h * 1000) / 10
  const pendingCount = (data.pending_items ?? []).length

  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-base'>{t('Account health')}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className='grid grid-cols-2 gap-3 md:grid-cols-4'>
          {statCard(
            t('Available balance'),
            formatQuota(data.available_quota)
          )}
          {statCard(
            t('Frozen balance'),
            formatQuota(data.frozen_quota),
            t('Unfreezing or in review')
          )}
          {statCard(
            t('24h error rate'),
            `${errorRatePercent}%`,
            t('Requests ending with zero settlement')
          )}
          {statCard(
            t('Pending items'),
            String(pendingCount),
            pendingCount > 0 ? t('Low balance or risk reminders') : undefined
          )}
        </div>
        {pendingCount > 0 ? (
          <div className='mt-4 flex flex-col gap-2'>
            {(data.pending_items ?? []).map((item) => (
              <div
                key={item.id}
                className='flex items-start justify-between gap-3 rounded-lg border bg-muted/40 px-3 py-2'
              >
                <div className='min-w-0'>
                  <p className='text-sm font-medium'>{item.title}</p>
                  <p className='text-muted-foreground line-clamp-2 text-xs'>
                    {item.content}
                  </p>
                </div>
              </div>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
