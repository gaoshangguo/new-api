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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuota } from '@/lib/format'
import { formatDateTimeObject } from '@/lib/time'

import {
  approveEscalatedManualCreditRequest,
  approveManualCreditRequest,
  getManualCreditRequests,
  rejectManualCreditRequest,
} from '../api'
import {
  FINANCE_MANUAL_CREDIT_QUERY_KEY,
  getManualCreditItems,
  type ManualAdjustmentEntryType,
  type ManualCreditRequest,
} from '../types'

type ManualCreditQueueProps = {
  canApprove: boolean
  canEscalate: boolean
}

export function ManualCreditQueue(props: ManualCreditQueueProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [approvalCandidate, setApprovalCandidate] =
    useState<ManualCreditRequest | null>(null)
  const [rejectionCandidate, setRejectionCandidate] =
    useState<ManualCreditRequest | null>(null)
  const [rejectionReason, setRejectionReason] = useState('')
  const [escalationCandidate, setEscalationCandidate] =
    useState<ManualCreditRequest | null>(null)
  const [escalationReason, setEscalationReason] = useState('')
  const entryTypeLabels: Record<ManualAdjustmentEntryType, string> = {
    manual_credit: t('Manual credit'),
    manual_debit: t('Manual debit'),
    compensation: t('Compensation'),
    freeze: t('Freeze balance'),
    unfreeze: t('Unfreeze balance'),
  }

  const creditsQuery = useQuery({
    queryKey: FINANCE_MANUAL_CREDIT_QUERY_KEY,
    queryFn: async () => {
      const response = await getManualCreditRequests()
      if (!response.success) {
        throw new Error(
          response.message || t('Adjustment request operation failed')
        )
      }
      return getManualCreditItems(response.data).filter(
        (request) =>
          request.status === 'pending' ||
          request.status === 'pending_escalation'
      )
    },
  })

  const refresh = async () => {
    await queryClient.invalidateQueries({
      queryKey: FINANCE_MANUAL_CREDIT_QUERY_KEY,
    })
  }

  const approveMutation = useMutation({
    mutationFn: async (request: ManualCreditRequest) => {
      const response = await approveManualCreditRequest(request.id)
      if (!response.success) {
        throw new Error(
          response.message || t('Adjustment request operation failed')
        )
      }
    },
    onSuccess: async () => {
      toast.success(t('Adjustment request approved'))
      setApprovalCandidate(null)
      await refresh()
    },
    onError: (error) => {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Adjustment request operation failed')
      )
    },
  })

  const rejectMutation = useMutation({
    mutationFn: async ({ request, reason }: { request: ManualCreditRequest; reason: string }) => {
      const response = await rejectManualCreditRequest(request.id, reason)
      if (!response.success) {
        throw new Error(
          response.message || t('Adjustment request operation failed')
        )
      }
    },
    onSuccess: async () => {
      toast.success(t('Adjustment request rejected'))
      setRejectionCandidate(null)
      setRejectionReason('')
      await refresh()
    },
    onError: (error) => {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Adjustment request operation failed')
      )
    },
  })

  const escalationMutation = useMutation({
    mutationFn: async ({
      request,
      reason,
    }: {
      request: ManualCreditRequest
      reason: string
    }) => {
      const response = await approveEscalatedManualCreditRequest(
        request.id,
        reason
      )
      if (!response.success) {
        throw new Error(
          response.message || t('Adjustment request operation failed')
        )
      }
    },
    onSuccess: async () => {
      toast.success(t('Escalated adjustment approved'))
      setEscalationCandidate(null)
      setEscalationReason('')
      await refresh()
    },
    onError: (error) => {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Adjustment request operation failed')
      )
    },
  })

  const credits = creditsQuery.data ?? []
  const isRejecting = rejectMutation.isPending
  const isEscalating = escalationMutation.isPending

  return (
    <>
      <div className='flex h-full min-h-0 flex-col rounded-xl border bg-card'>
        <div className='flex items-center justify-between gap-3 border-b px-4 py-3'>
          <div>
            <h3 className='text-sm font-semibold'>
              {t('Pending balance adjustments')}
            </h3>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Review pending credits before applying them to customer balances.'
              )}
            </p>
          </div>
          <Button
            type='button'
            size='icon-sm'
            variant='ghost'
            onClick={() => void creditsQuery.refetch()}
            disabled={creditsQuery.isFetching}
            aria-label={t('Refresh')}
          >
            <RefreshCw
              className={creditsQuery.isFetching ? 'animate-spin' : undefined}
            />
          </Button>
        </div>
        <div className='min-h-0 flex-1 overflow-auto'>
          {creditsQuery.isLoading && (
            <div className='space-y-3 p-4'>
              {[0, 1, 2].map((index) => (
                <Skeleton key={index} className='h-10 w-full' />
              ))}
            </div>
          )}
          {!creditsQuery.isLoading && credits.length === 0 && (
            <Empty className='min-h-56 border-0'>
              <EmptyHeader>
                <EmptyTitle>{t('No pending balance adjustments')}</EmptyTitle>
                <EmptyDescription>
                  {t(
                    'Create a credit request after confirming the offline payment.'
                  )}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
          {!creditsQuery.isLoading && credits.length > 0 && (
            <Table aria-label={t('Pending balance adjustments')}>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Request ID')}</TableHead>
                  <TableHead>{t('Adjustment type')}</TableHead>
                  <TableHead>{t('Customer user ID')}</TableHead>
                  <TableHead>{t('Project')}</TableHead>
                  <TableHead>{t('Amount')}</TableHead>
                  <TableHead>{t('External reference')}</TableHead>
                  <TableHead>{t('Reason')}</TableHead>
                  <TableHead>{t('Created At')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {credits.map((credit) => (
                  <TableRow key={credit.id}>
                    <TableCell>{credit.id}</TableCell>
                    <TableCell>
                      <StatusBadge
                        label={entryTypeLabels[credit.entry_type]}
                        variant='neutral'
                        copyable={false}
                      />
                    </TableCell>
                    <TableCell>{credit.user_id}</TableCell>
                    <TableCell>{credit.project_id ?? '-'}</TableCell>
                    <TableCell className='font-medium'>
                      {formatQuota(credit.amount)}
                    </TableCell>
                    <TableCell className='max-w-44 truncate'>
                      {credit.external_reference}
                    </TableCell>
                    <TableCell className='max-w-52 truncate'>
                      {credit.reason}
                    </TableCell>
                    <TableCell>
                      {formatDateTimeObject(new Date(credit.created_at * 1000))}
                    </TableCell>
                    <TableCell>
                      <div className='flex justify-end gap-2'>
                        <StatusBadge
                          label={
                            credit.status === 'pending_escalation'
                              ? t('Pending escalation')
                              : t('Pending')
                          }
                          variant='warning'
                          copyable={false}
                        />
                        {credit.status === 'pending_escalation' &&
                        props.canEscalate ? (
                            <Button
                              type='button'
                              size='sm'
                              onClick={() => setEscalationCandidate(credit)}
                            >
                              {t('Approve escalated adjustment')}
                            </Button>
                        ) : null}
                        {credit.status === 'pending' && props.canApprove ? (
                          <>
                            <Button
                              type='button'
                              size='sm'
                              onClick={() => setApprovalCandidate(credit)}
                            >
                              {t('Approve')}
                            </Button>
                            <Button
                              type='button'
                              size='sm'
                              variant='outline'
                              onClick={() => setRejectionCandidate(credit)}
                            >
                              {t('Reject')}
                            </Button>
                          </>
                        ) : null}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={approvalCandidate !== null}
        onOpenChange={(open) => {
          if (!open) setApprovalCandidate(null)
        }}
        title={t('Approve adjustment request')}
        desc={
          approvalCandidate
            ? t('Approve this request and apply {{amount}} to the customer balance?', {
                amount: formatQuota(approvalCandidate.amount),
              })
            : ''
        }
        confirmText={t('Approve')}
        isLoading={approveMutation.isPending}
        handleConfirm={() => {
          if (!approvalCandidate) return
          approveMutation.mutate(approvalCandidate)
        }}
      />

      <Dialog
        open={rejectionCandidate !== null}
        onOpenChange={(open) => {
          if (!open && !isRejecting) {
            setRejectionCandidate(null)
            setRejectionReason('')
          }
        }}
        title={t('Reject adjustment request')}
        description={t('Enter the reason for rejecting this request.')}
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              disabled={isRejecting}
              onClick={() => {
                setRejectionCandidate(null)
                setRejectionReason('')
              }}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              variant='destructive'
              disabled={isRejecting || !rejectionReason.trim()}
              onClick={() => {
                if (!rejectionCandidate) return
                rejectMutation.mutate({
                  request: rejectionCandidate,
                  reason: rejectionReason.trim(),
                })
              }}
            >
              {isRejecting ? t('Processing...') : t('Reject request')}
            </Button>
          </>
        }
      >
        <Input
          autoFocus
          value={rejectionReason}
          onChange={(event) => setRejectionReason(event.target.value)}
          disabled={isRejecting}
        />
      </Dialog>

      <Dialog
        open={escalationCandidate !== null}
        onOpenChange={(open) => {
          if (!open && !isEscalating) {
            setEscalationCandidate(null)
            setEscalationReason('')
          }
        }}
        title={t('Duplicate adjustment requires escalation')}
        description={t(
          'Enter the exception reason for approving this duplicate adjustment.'
        )}
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              disabled={isEscalating}
              onClick={() => {
                setEscalationCandidate(null)
                setEscalationReason('')
              }}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              disabled={isEscalating || !escalationReason.trim()}
              onClick={() => {
                if (!escalationCandidate) return
                escalationMutation.mutate({
                  request: escalationCandidate,
                  reason: escalationReason.trim(),
                })
              }}
            >
              {isEscalating
                ? t('Processing...')
                : t('Approve escalated adjustment')}
            </Button>
          </>
        }
      >
        <Input
          autoFocus
          value={escalationReason}
          onChange={(event) => setEscalationReason(event.target.value)}
          disabled={isEscalating}
        />
      </Dialog>
    </>
  )
}
