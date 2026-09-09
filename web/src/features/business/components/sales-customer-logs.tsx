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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import type { StaticDataTableColumn } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota, formatTimestamp, formatUseTime } from '@/lib/format'
import { getLogTypeConfig } from '@/features/usage-logs/lib'
import { cn } from '@/lib/utils'

import { getSalesCustomerLogsPage } from '../api'
import type { SalesUsageLog } from '../types'

const PAGE_SIZE = 20
const LOG_TYPE_DOT: Record<string, string> = {
  green: 'bg-emerald-500',
  red: 'bg-red-500',
  blue: 'bg-sky-500',
  orange: 'bg-orange-500',
  purple: 'bg-purple-500',
  cyan: 'bg-cyan-500',
  teal: 'bg-teal-500',
}

type SalesCustomerLogsDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  companyId: number
  customerName: string
}

export function SalesCustomerLogsDialog({
  open,
  onOpenChange,
  companyId,
  customerName,
}: SalesCustomerLogsDialogProps) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')

  useEffect(() => {
    if (open) {
      setPage(1)
    }
  }, [open, companyId])

  const startAt = startDate
    ? Math.floor(new Date(`${startDate}T00:00:00`).getTime() / 1000)
    : undefined
  const endAt = endDate
    ? Math.floor(new Date(`${endDate}T23:59:59`).getTime() / 1000)
    : undefined

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'business',
      'sales',
      'customers',
      companyId,
      'logs-page',
      page,
      startAt,
      endAt,
    ],
    queryFn: () =>
      getSalesCustomerLogsPage(companyId, {
        p: page,
        page_size: PAGE_SIZE,
        start_at: startAt,
        end_at: endAt,
      }),
    enabled: open && companyId > 0,
  })

  const logs = data?.items ?? []
  const total = data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  const columns: StaticDataTableColumn<SalesUsageLog>[] = [
    {
      id: 'created_at',
      header: t('Time'),
      cell: (row) => (
        <span className='text-muted-foreground whitespace-nowrap text-sm'>
          {row.created_at ? formatTimestamp(row.created_at) : '-'}
        </span>
      ),
    },
    {
      id: 'type',
      header: t('Type'),
      cell: (row) => {
        const config = getLogTypeConfig(row.type)
        return (
          <span className='inline-flex items-center gap-1.5 text-sm'>
            <span
              className={cn(
                'size-1.5 shrink-0 rounded-full',
                LOG_TYPE_DOT[config.color] ?? 'bg-muted-foreground'
              )}
            />
            {t(config.label)}
          </span>
        )
      },
    },
    {
      id: 'model',
      header: t('Model'),
      cell: (row) => row.model_name || '-',
    },
    {
      id: 'token',
      header: t('Token'),
      cell: (row) => (
        <div className='min-w-0'>
          <div className='truncate text-sm'>{row.token_identifier || '-'}</div>
          {row.prompt_tokens + row.completion_tokens > 0 && (
            <div className='text-muted-foreground text-xs'>
              {row.prompt_tokens + row.completion_tokens}
            </div>
          )}
        </div>
      ),
    },
    {
      id: 'quota',
      header: t('Cost'),
      cell: (row) => formatQuota(row.quota),
    },
    {
      id: 'use_time',
      header: t('Latency'),
      cell: (row) => (row.use_time >= 0 ? formatUseTime(row.use_time) : '-'),
    },
  ]

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('View all logs')}
      description={customerName}
      contentHeight='auto'
      bodyClassName='space-y-3'
      footer={
        <>
          <p className='text-muted-foreground mr-auto text-sm'>
            {t('Total')}: {total}
          </p>
          <Button
            variant='outline'
            size='sm'
            disabled={page <= 1 || isFetching}
            onClick={() => setPage((value) => Math.max(1, value - 1))}
          >
            {t('Previous')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            disabled={page >= totalPages || isFetching}
            onClick={() => setPage((value) => value + 1)}
          >
            {t('Next')}
          </Button>
          <Button variant='outline' size='sm' onClick={() => onOpenChange(false)}>
            {t('Close')}
          </Button>
        </>
      }
    >
      <div className='flex flex-wrap items-center gap-2'>
        <Input
          aria-label={t('Start date')}
          className='h-8 w-36'
          type='date'
          value={startDate}
          onChange={(event) => {
            setStartDate(event.target.value)
            setPage(1)
          }}
        />
        <Input
          aria-label={t('End date')}
          className='h-8 w-36'
          type='date'
          value={endDate}
          onChange={(event) => {
            setEndDate(event.target.value)
            setPage(1)
          }}
        />
      </div>
      {isLoading ? (
        <div className='space-y-2 py-4'>
          <Skeleton className='h-8 w-full' />
          <Skeleton className='h-24 w-full' />
        </div>
      ) : logs.length === 0 ? (
        <Empty className='min-h-40 border-0'>
          <EmptyTitle>{t('No usage logs yet.')}</EmptyTitle>
          <EmptyDescription>
            {t('Adjust the date range or try again later.')}
          </EmptyDescription>
        </Empty>
      ) : (
        <StaticDataTable
          data={logs}
          columns={columns}
          getRowKey={(row) => row.id}
        />
      )}
    </Dialog>
  )
}
