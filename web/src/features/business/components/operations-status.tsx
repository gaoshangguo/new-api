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

import { ErrorState } from '@/components/error-state'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatNumber, formatQuota } from '@/lib/format'

import {
  getBusinessAnnouncements,
  getBusinessChannelHealth,
  getBusinessOperationsOverview,
  getPlatformReadOnlyUsers,
  getPlatformReadOnlyLedger,
  getPlatformReadOnlyConsumptions,
  getPlatformReadOnlyAdjustments,
  getBusinessAuditEvents,
} from '../api'

type OperationsStatusProps = {
  canReadOperations: boolean
  canReadReports: boolean
}

export function OperationsStatus(props: OperationsStatusProps) {
  const { t } = useTranslation()
  const overviewQuery = useQuery({
    queryKey: ['business', 'operations', 'overview'],
    queryFn: getBusinessOperationsOverview,
    enabled: props.canReadReports,
    staleTime: 60 * 1000,
  })
  const healthQuery = useQuery({
    queryKey: ['business', 'operations', 'channel-health'],
    queryFn: getBusinessChannelHealth,
    enabled: props.canReadOperations,
    staleTime: 60 * 1000,
  })
  const announcementsQuery = useQuery({
    queryKey: ['business', 'operations', 'announcements'],
    queryFn: getBusinessAnnouncements,
    enabled: props.canReadOperations,
    staleTime: 60 * 1000,
  })
  const usersQuery = useQuery({
    queryKey: ['business', 'operations', 'users'],
    queryFn: getPlatformReadOnlyUsers,
    enabled: props.canReadOperations,
    staleTime: 60 * 1000,
  })
  const ledgerQuery = useQuery({
    queryKey: ['business', 'operations', 'ledger'],
    queryFn: getPlatformReadOnlyLedger,
    enabled: props.canReadOperations,
    staleTime: 60 * 1000,
  })
  const consumptionQuery = useQuery({
    queryKey: ['business', 'operations', 'consumption'],
    queryFn: getPlatformReadOnlyConsumptions,
    enabled: props.canReadOperations,
    staleTime: 60 * 1000,
  })
  const adjustmentsQuery = useQuery({
    queryKey: ['business', 'operations', 'adjustments'],
    queryFn: getPlatformReadOnlyAdjustments,
    enabled: props.canReadOperations,
    staleTime: 60 * 1000,
  })
  const auditQuery = useQuery({
    queryKey: ['business', 'operations', 'audit'],
    queryFn: getBusinessAuditEvents,
    enabled: props.canReadOperations,
    staleTime: 60 * 1000,
  })

  const healthyChannels = (healthQuery.data ?? []).filter(
    (channel) => channel.status === 1
  ).length
  const degradedChannels = (healthQuery.data ?? []).length - healthyChannels

  return (
    <div className='grid gap-4 xl:grid-cols-2'>
      {props.canReadReports && (
        <Card>
          <CardHeader>
            <CardTitle>{t('Operations overview')}</CardTitle>
            <CardDescription>{t('Last 24 hours')}</CardDescription>
          </CardHeader>
          <CardContent>
            {overviewQuery.isLoading && <OverviewSkeleton />}
            {!overviewQuery.isLoading && overviewQuery.isError && (
              <ErrorState
                className='min-h-48 border-0'
                title={t('Unable to load business data')}
                description={t('Try refreshing this panel.')}
                onRetry={() => void overviewQuery.refetch()}
              />
            )}
            {!overviewQuery.isLoading &&
              !overviewQuery.isError &&
              overviewQuery.data && (
                <div className='grid grid-cols-2 gap-3 sm:grid-cols-3'>
                  <Metric
                    label={t('Enterprise customers')}
                    value={formatNumber(overviewQuery.data.company_count)}
                  />
                  <Metric
                    label={t('Projects')}
                    value={formatNumber(overviewQuery.data.project_count)}
                  />
                  <Metric
                    label={t('Pending adjustments')}
                    value={formatNumber(overviewQuery.data.pending_adjustment_count)}
                  />
                  <Metric
                    label={t('Requests')}
                    value={formatNumber(overviewQuery.data.request_count)}
                  />
                  <Metric
                    label={t('Errors')}
                    value={formatNumber(overviewQuery.data.error_count)}
                  />
                  <Metric
                    label={t('Consumed quota')}
                    value={formatQuota(overviewQuery.data.consumed_quota)}
                  />
                  <Metric
                    label={t('Revenue')}
                    value={formatQuota(overviewQuery.data.revenue_quota)}
                  />
                  <Metric
                    label={t('Cost (ratio {{ratio}}%)', { ratio: Math.round(overviewQuery.data.cost_ratio * 100) })}
                    value={formatQuota(overviewQuery.data.cost_quota)}
                  />
                  <Metric
                    label={t('Gross margin')}
                    value={formatQuota(overviewQuery.data.gross_margin_quota)}
                  />
                </div>
              )}
          {overviewQuery.data ? (
          <div className='mt-4 grid gap-4 md:grid-cols-2'>
            <div>
              <h4 className='mb-2 text-sm font-medium'>{t('Top models')}</h4>
              {overviewQuery.data.top_models && overviewQuery.data.top_models.length > 0 ? (
                <ul className='flex flex-col gap-1 text-sm'>
                  {overviewQuery.data.top_models.map((item) => (
                    <li key={item.model_name} className='flex items-center justify-between gap-2'>
                      <span className='truncate'>{item.model_name}</span>
                      <span className='text-muted-foreground shrink-0'>
                        {formatQuota(item.quota)}
                      </span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className='text-muted-foreground text-xs'>{t('No usage yet')}</p>
              )}
            </div>
            <div>
              <h4 className='mb-2 text-sm font-medium'>{t('Top companies')}</h4>
              {overviewQuery.data.top_companies && overviewQuery.data.top_companies.length > 0 ? (
                <ul className='flex flex-col gap-1 text-sm'>
                  {overviewQuery.data.top_companies.map((item) => (
                    <li key={item.model_name} className='flex items-center justify-between gap-2'>
                      <span className='truncate'>#{item.model_name}</span>
                      <span className='text-muted-foreground shrink-0'>
                        {formatQuota(item.quota)}
                      </span>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className='text-muted-foreground text-xs'>{t('No usage yet')}</p>
              )}
            </div>
          </div>
          ) : null}
          </CardContent>
        </Card>
      )}

      {props.canReadOperations && (
        <Card>
          <CardHeader>
            <CardTitle>{t('Channel status')}</CardTitle>
            <CardDescription>
              {t('Health indicators do not expose upstream credentials.')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {healthQuery.isLoading && <OverviewSkeleton />}
            {!healthQuery.isLoading && healthQuery.isError && (
              <ErrorState
                className='min-h-48 border-0'
                title={t('Unable to load business data')}
                description={t('Try refreshing this panel.')}
                onRetry={() => void healthQuery.refetch()}
              />
            )}
            {!healthQuery.isLoading && !healthQuery.isError && (
              <div className='grid grid-cols-3 gap-3'>
                <Metric
                  label={t('Channels')}
                  value={formatNumber(healthQuery.data?.length ?? 0)}
                />
                <Metric label={t('Healthy')} value={formatNumber(healthyChannels)} />
                <Metric
                  label={t('Degraded')}
                  value={formatNumber(degradedChannels)}
                />
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {props.canReadOperations && (
        <Card className='xl:col-span-2'>
          <CardHeader>
            <CardTitle>{t('Recent audit events')}</CardTitle>
            <CardDescription>{t('Immutable records of sensitive reads and business changes.')}</CardDescription>
          </CardHeader>
          <CardContent>
            {auditQuery.isLoading && <OverviewSkeleton />}
            {!auditQuery.isLoading && auditQuery.data?.length === 0 && <p className='text-muted-foreground text-sm'>{t('No audit events available.')}</p>}
            {!auditQuery.isLoading && auditQuery.data && auditQuery.data.length > 0 && <div className='space-y-2'>
              {auditQuery.data.map((event) => <div key={event.id} className='grid gap-1 rounded-lg border px-3 py-2 text-sm sm:grid-cols-[1fr_auto]'><span className='truncate'>{event.action}</span><span className='text-muted-foreground'>{event.actor_username}</span></div>)}
            </div>}
          </CardContent>
        </Card>
      )}

      {props.canReadOperations && (
        <Card className='xl:col-span-2'>
          <CardHeader>
            <CardTitle>{t('Recent adjustment orders')}</CardTitle>
            <CardDescription>{t('Read-only adjustment order statuses without external references or review notes.')}</CardDescription>
          </CardHeader>
          <CardContent>
            {adjustmentsQuery.isLoading && <OverviewSkeleton />}
            {!adjustmentsQuery.isLoading && adjustmentsQuery.data?.length === 0 && (
              <p className='text-muted-foreground text-sm'>{t('No adjustment orders available.')}</p>
            )}
            {!adjustmentsQuery.isLoading && adjustmentsQuery.data && adjustmentsQuery.data.length > 0 && (
              <div className='space-y-2'>
                {adjustmentsQuery.data.map((entry) => (
                  <div key={entry.id} className='grid gap-1 rounded-lg border px-3 py-2 text-sm sm:grid-cols-[1fr_auto_auto]'>
                    <span className='truncate'>{entry.entry_type}</span>
                    <span className='text-muted-foreground'>{entry.status}</span>
                    <span className='text-muted-foreground'>{formatQuota(entry.amount)}</span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {props.canReadOperations && (
        <Card className='xl:col-span-2'>
          <CardHeader>
            <CardTitle>{t('Recent platform usage')}</CardTitle>
            <CardDescription>{t('Read-only usage records without request content or client identifiers.')}</CardDescription>
          </CardHeader>
          <CardContent>
            {consumptionQuery.isLoading && <OverviewSkeleton />}
            {!consumptionQuery.isLoading && consumptionQuery.data?.length === 0 && (
              <p className='text-muted-foreground text-sm'>{t('No usage records available.')}</p>
            )}
            {!consumptionQuery.isLoading && consumptionQuery.data && consumptionQuery.data.length > 0 && (
              <div className='space-y-2'>
                {consumptionQuery.data.map((entry) => (
                  <div key={entry.id} className='grid gap-1 rounded-lg border px-3 py-2 text-sm sm:grid-cols-[1fr_auto]'>
                    <span className='truncate'>{entry.model_name}</span>
                    <span className='text-muted-foreground'>{formatQuota(entry.quota)}</span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {props.canReadOperations && (
        <Card className='xl:col-span-2'>
          <CardHeader>
            <CardTitle>{t('Recent balance ledger')}</CardTitle>
            <CardDescription>{t('Read-only balance movements without external references or notes.')}</CardDescription>
          </CardHeader>
          <CardContent>
            {ledgerQuery.isLoading && <OverviewSkeleton />}
            {!ledgerQuery.isLoading && ledgerQuery.data?.length === 0 && (
              <p className='text-muted-foreground text-sm'>{t('No balance movements available.')}</p>
            )}
            {!ledgerQuery.isLoading && ledgerQuery.data && ledgerQuery.data.length > 0 && (
              <div className='space-y-2'>
                {ledgerQuery.data.map((entry) => (
                  <div key={entry.id} className='grid gap-1 rounded-lg border px-3 py-2 text-sm sm:grid-cols-[1fr_auto]'>
                    <span className='truncate'>{entry.entry_type}</span>
                    <span className='text-muted-foreground'>{formatQuota(entry.amount)}</span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {props.canReadOperations && (
        <Card className='xl:col-span-2'>
          <CardHeader>
            <CardTitle>{t('Platform users')}</CardTitle>
            <CardDescription>{t('Read-only user usage overview without contact details or credentials.')}</CardDescription>
          </CardHeader>
          <CardContent>
            {usersQuery.isLoading && <OverviewSkeleton />}
            {!usersQuery.isLoading && usersQuery.data?.length === 0 && (
              <p className='text-muted-foreground text-sm'>{t('No users available.')}</p>
            )}
            {!usersQuery.isLoading && usersQuery.data && usersQuery.data.length > 0 && (
              <div className='space-y-2'>
                {usersQuery.data.map((user) => (
                  <div key={user.id} className='grid gap-1 rounded-lg border px-3 py-2 text-sm sm:grid-cols-[1fr_auto_auto]'>
                    <span className='truncate'>{user.display_name || user.username}</span>
                    <span className='text-muted-foreground'>{formatQuota(user.quota)}</span>
                    <span className='text-muted-foreground'>{formatNumber(user.request_count)}</span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {props.canReadOperations && (
        <Card className='xl:col-span-2'>
          <CardHeader>
            <CardTitle>{t('Maintenance announcements')}</CardTitle>
            <CardDescription>
              {t('Current maintenance and known-service notices.')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {announcementsQuery.isLoading && (
              <div className='space-y-2'>
                <Skeleton className='h-14 w-full' />
                <Skeleton className='h-14 w-full' />
              </div>
            )}
            {!announcementsQuery.isLoading && announcementsQuery.isError && (
              <ErrorState
                className='min-h-48 border-0'
                title={t('Unable to load business data')}
                description={t('Try refreshing this panel.')}
                onRetry={() => void announcementsQuery.refetch()}
              />
            )}
            {!announcementsQuery.isLoading &&
              !announcementsQuery.isError &&
              (announcementsQuery.data?.length ?? 0) === 0 && (
              <p className='text-muted-foreground text-sm'>
                {t('No active announcements')}
              </p>
            )}
            {!announcementsQuery.isLoading &&
              !announcementsQuery.isError &&
              (announcementsQuery.data?.length ?? 0) > 0 && (
              <div className='space-y-2'>
                {announcementsQuery.data?.slice(0, 3).map((announcement) => (
                  <div key={announcement.id} className='rounded-lg border p-3'>
                    <p className='font-medium'>{announcement.title}</p>
                    <p className='text-muted-foreground mt-1 text-sm whitespace-pre-wrap'>
                      {announcement.content}
                    </p>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className='rounded-lg border p-3'>
      <p className='text-muted-foreground text-xs'>{props.label}</p>
      <p className='mt-1 truncate text-lg font-semibold'>{props.value}</p>
    </div>
  )
}

function OverviewSkeleton() {
  return (
    <div className='grid grid-cols-3 gap-3'>
      {[0, 1, 2].map((index) => (
        <Skeleton key={index} className='h-18 w-full' />
      ))}
    </div>
  )
}
