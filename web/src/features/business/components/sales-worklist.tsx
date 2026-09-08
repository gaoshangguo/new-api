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
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { LedgerExportDialog } from '@/features/finance/components/ledger-export-dialog'
import { Input } from '@/components/ui/input'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import {
  getSalesCustomerSummary,
  getSalesCustomers,
  getSalesInvitationCode,
  getSalesCustomerLogs,
} from '../api'

import { SalesFollowUps } from './sales-follow-ups'
import { SalesReminders } from './sales-reminders'

export function SalesWorklist() {
  const [exportReport, setExportReport] = useState<'ledger' | 'consumption' | null>(null)
  const { t } = useTranslation()
  const [selectedCustomerId, setSelectedCustomerId] = useState<number | null>(
    null
  )
  const [logStartDate, setLogStartDate] = useState('')
  const [logEndDate, setLogEndDate] = useState('')
  const customersQuery = useQuery({
    queryKey: ['business', 'sales', 'customers'],
    queryFn: getSalesCustomers,
    staleTime: 60 * 1000,
  })
  const customerData = customersQuery.data
  const customerItems = customerData ?? []
  const invitationQuery = useQuery({
    queryKey: ['business', 'sales', 'invitation'],
    queryFn: getSalesInvitationCode,
    staleTime: 60 * 60 * 1000,
  })

  useEffect(() => {
    if (
      customerData &&
      customerData.length > 0 &&
      !customerData.some((customer) => customer.id === selectedCustomerId)
    ) {
      setSelectedCustomerId(customerData[0].id)
    }
  }, [customerData, selectedCustomerId])

  const summaryQuery = useQuery({
    queryKey: ['business', 'sales', 'customers', selectedCustomerId, 'summary'],
    queryFn: () => getSalesCustomerSummary(selectedCustomerId ?? 0),
    enabled: selectedCustomerId !== null,
    staleTime: 60 * 1000,
  })
  const logsQuery = useQuery({
    queryKey: ['business', 'sales', 'customers', selectedCustomerId, 'logs', logStartDate, logEndDate],
    queryFn: () => getSalesCustomerLogs(
      selectedCustomerId ?? 0,
      logStartDate ? Math.floor(new Date(`${logStartDate}T00:00:00`).getTime() / 1000) : undefined,
      logEndDate ? Math.floor(new Date(`${logEndDate}T23:59:59`).getTime() / 1000) : undefined
    ),
    enabled: selectedCustomerId !== null,
    staleTime: 60 * 1000,
  })

  if (customersQuery.isLoading) return <SalesWorklistSkeleton />

  if (customersQuery.isError) {
    return (
      <ErrorState
        className='min-h-56'
        title={t('Unable to load business data')}
        description={t('Try refreshing this panel.')}
        onRetry={() => void customersQuery.refetch()}
      />
    )
  }

  if (customerItems.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('Sales worklist')}</CardTitle>
        </CardHeader>
        <CardContent>
          <Empty className='min-h-48 border-0'>
            <EmptyHeader>
              <EmptyTitle>{t('No assigned customers')}</EmptyTitle>
              <EmptyDescription>
                {t('Customers assigned to you will appear here.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        </CardContent>
      </Card>
    )
  }

  const summary = summaryQuery.data

  const invitationLink = invitationQuery.data
    ? `${window.location.origin}/sign-up?aff=${encodeURIComponent(invitationQuery.data)}`
    : ''

  const copyInvitationLink = async () => {
    try {
      await navigator.clipboard.writeText(invitationLink)
      toast.success(t('Invitation link copied.'))
    } catch {
      toast.error(t('Unable to copy invitation link.'))
    }
  }

  return (
    <>
      <div className='space-y-4'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Customer invitation')}</CardTitle>
          <CardDescription>
            {t('Share this registration link to associate a new customer with your sales account.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-2 sm:flex-row'>
          <Input readOnly value={invitationLink} aria-label={t('Customer invitation')} />
          <Button
            type='button'
            disabled={!invitationLink}
            onClick={() => void copyInvitationLink()}
          >
            {t('Copy link')}
          </Button>
        </CardContent>
      </Card>
      <SalesReminders />
      <Card>
      <CardHeader>
        <CardTitle>{t('Sales worklist')}</CardTitle>
        <CardDescription>
          {t('Review assigned customers, balances, and project thresholds.')}
        </CardDescription>
        <div className='flex items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => setExportReport('ledger')}
          >
            {t('Export ledger')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => setExportReport('consumption')}
          >
            {t('Export consumption')}
          </Button>
        </div>
      </CardHeader>
      <CardContent className='grid gap-4 lg:grid-cols-[14rem_minmax(0,1fr)]'>
        <div className='flex max-h-64 flex-row gap-2 overflow-x-auto pb-1 lg:max-h-none lg:flex-col lg:overflow-y-auto'>
          {customerItems.map((customer) => (
            <Button
              key={customer.id}
              type='button'
              variant={
                customer.id === selectedCustomerId ? 'secondary' : 'ghost'
              }
              className='min-w-40 justify-start lg:min-w-0'
              onClick={() => setSelectedCustomerId(customer.id)}
            >
              <span className='truncate'>{customer.name}</span>
            </Button>
          ))}
        </div>
        {summaryQuery.isLoading && (
          <div className='space-y-3'>
            <Skeleton className='h-8 w-48' />
            <Skeleton className='h-28 w-full' />
          </div>
        )}
        {!summaryQuery.isLoading && summaryQuery.isError && (
          <ErrorState
            className='min-h-48 border-0'
            title={t('Unable to load business data')}
            description={t('Try refreshing this panel.')}
            onRetry={() => void summaryQuery.refetch()}
          />
        )}
        {!summaryQuery.isLoading && !summaryQuery.isError && summary && (
          <div className='min-w-0 space-y-4'>
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <div>
                <p className='font-medium'>{summary.customer.name}</p>
                <p className='text-muted-foreground text-sm'>
                  {t('Customer balance')}
                </p>
              </div>
              <p className='text-lg font-semibold'>
                {summary.balance_available && summary.balance !== null ? formatQuota(summary.balance) : '—'}
              </p>
            </div>
            <div className='grid gap-2 sm:grid-cols-6'>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('Last 7 days')}</p>
                <p className='mt-1 font-medium'>{formatQuota(summary.usage.last_7_days_quota)}</p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('Last 30 days')}</p>
                <p className='mt-1 font-medium'>{formatQuota(summary.usage.last_30_days_quota)}</p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('Calls')}</p>
                <p className='mt-1 font-medium'>{summary.usage.last_30_days_calls}</p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('Credits last 30 days')}</p>
                <p className='mt-1 font-medium'>{formatQuota(summary.usage.last_30_days_credit)}</p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('Low balance projects')}</p>
                <p className='mt-1 font-medium'>{summary.alerts.low_balance_project_count}</p>
              </div>
              <div className='rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('New projects last 30 days')}</p>
                <p className='mt-1 font-medium'>{summary.usage.new_projects_30_days}</p>
              </div>
            </div>
            <div>
              <p className='mb-2 text-sm font-medium'>{t('Top models')}</p>
              {summary.top_models.length === 0 ? (
                <p className='text-muted-foreground text-sm'>{t('No usage yet.')}</p>
              ) : (
                <div className='space-y-2'>
                  {summary.top_models.map((model) => (
                    <div key={model.model_name} className='flex items-center justify-between rounded-lg border px-3 py-2 text-sm'>
                      <span className='truncate'>{model.model_name}</span>
                      <span className='text-muted-foreground'>{formatQuota(model.quota)} · {model.calls}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div>
              <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
                <p className='text-sm font-medium'>{t('Recent usage logs')}</p>
                <div className='flex gap-2'>
                  <Input aria-label={t('Start date')} className='h-8 w-36' type='date' value={logStartDate} onChange={(event) => setLogStartDate(event.target.value)} />
                  <Input aria-label={t('End date')} className='h-8 w-36' type='date' value={logEndDate} onChange={(event) => setLogEndDate(event.target.value)} />
                </div>
              </div>
              {logsQuery.isLoading && <Skeleton className='h-20 w-full' />}
              {!logsQuery.isLoading && logsQuery.data?.length === 0 && (
                <p className='text-muted-foreground text-sm'>{t('No usage logs yet.')}</p>
              )}
              {!logsQuery.isLoading && logsQuery.data && logsQuery.data.length > 0 && (
                <div className='space-y-2'>
                  {logsQuery.data.map((log) => (
                    <div key={log.id} className='grid gap-1 rounded-lg border px-3 py-2 text-sm sm:grid-cols-[1fr_1fr_auto]'>
                      <span className='truncate'>{log.model_name}</span>
                      <span className='truncate text-muted-foreground'>{log.token_identifier}</span>
                      <span className='text-muted-foreground'>{formatQuota(log.quota)}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div>
              <p className='mb-2 text-sm font-medium'>
                {t('Project thresholds')}
              </p>
              {summary.projects.length === 0 ? (
                <p className='text-muted-foreground text-sm'>
                  {t('No project thresholds have been configured.')}
                </p>
              ) : (
                <div className='grid gap-2 sm:grid-cols-2'>
                  {summary.projects.map((project) => (
                    <div key={project.id} className='rounded-lg border p-3'>
                      <div className='flex items-start justify-between gap-2'>
                        <p className='truncate font-medium'>{project.name}</p>
                        <StatusBadge
                          label={
                            project.status === 1 ? t('Active') : t('Disabled')
                          }
                          variant={project.status === 1 ? 'success' : 'neutral'}
                          copyable={false}
                        />
                      </div>
                      <dl className='mt-3 grid grid-cols-2 gap-2 text-xs'>
                        <div>
                          <dt className='text-muted-foreground'>
                            {t('Project budget')}
                          </dt>
                          <dd className='mt-1 text-sm font-medium'>
                            {formatQuota(project.budget_quota)}
                          </dd>
                        </div>
                        <div>
                          <dt className='text-muted-foreground'>
                            {t('Low balance threshold')}
                          </dt>
                          <dd className='mt-1 text-sm font-medium'>
                            {formatQuota(project.low_balance_quota)}
                          </dd>
                        </div>
                      </dl>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <SalesFollowUps companyId={selectedCustomerId ?? 0} />
          </div>
        )}
      </CardContent>
      </Card>
      </div>
      {exportReport ? (
        <LedgerExportDialog
          open={exportReport !== null}
          onOpenChange={(open) => {
            if (!open) setExportReport(null)
          }}
          report={exportReport}
          companyId={selectedCustomerId ?? undefined}
        />
      ) : null}
    </>
  )
}

function SalesWorklistSkeleton() {
  return (
    <Card>
      <CardHeader>
        <Skeleton className='h-5 w-28' />
        <Skeleton className='h-4 w-80' />
      </CardHeader>
      <CardContent className='grid gap-4 lg:grid-cols-[14rem_minmax(0,1fr)]'>
        <Skeleton className='h-32 w-full' />
        <Skeleton className='h-40 w-full' />
      </CardContent>
    </Card>
  )
}
