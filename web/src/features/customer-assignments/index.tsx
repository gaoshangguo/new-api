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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getRouteApi,
  useNavigate,
} from '@tanstack/react-router'
import { History, Link2, X } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table'
import type { StaticDataTableColumn } from '@/components/data-table'
import { ComboboxInput } from '@/components/ui/combobox-input'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyTitle } from '@/components/ui/empty'
import { Label } from '@/components/ui/label'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { formatTimestamp } from '@/lib/format'
import { api } from '@/lib/api'

import { getSalesAccounts } from '../sales-accounts/api'

const route = getRouteApi('/_authenticated/business/customer-assignments')

type ApiEnvelope<T> = { success: boolean; message?: string; data?: T }
type PageEnvelope<T> = { items?: T[]; total?: number }

type AssignedCompany = {
  id: number
  name: string
  owner_user_id: number
  created_at: number
}

type CompanyOption = {
  id: number
  name: string
}

type AssignmentHistoryEntry = {
  id: number
  company_id: number
  customer_user_id: number
  sales_user_id: number
  assigned_by: number
  active: boolean
  reason: string
  created_at: number
}

async function getAssignedCompanies(
  salesUserId: string
): Promise<AssignedCompany[]> {
  if (!salesUserId) return []
  const response = await api.get<ApiEnvelope<PageEnvelope<AssignedCompany>>>(
    `/api/business/sales/accounts/${salesUserId}/customers`,
    { params: { p: 1, page_size: 100 } }
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to load customers')
  }
  return response.data.data?.items ?? []
}

async function getCompanyOptions(): Promise<CompanyOption[]> {
  const response = await api.get<ApiEnvelope<PageEnvelope<CompanyOption>>>(
    '/api/business/companies',
    { params: { p: 1, page_size: 100 } }
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to load enterprises')
  }
  return response.data.data?.items ?? []
}

async function getAssignmentHistory(
  companyId: number
): Promise<AssignmentHistoryEntry[]> {
  const response = await api.get<ApiEnvelope<PageEnvelope<AssignmentHistoryEntry>>>(
    '/api/business/customer-assignments',
    { params: { company_id: companyId, p: 1, page_size: 50 } }
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Failed to load assignment history')
  }
  return response.data.data?.items ?? []
}

export function CustomerAssignmentsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const search = route.useSearch()
  const [salesId, setSalesId] = useState(search.sales ?? '')
  const [assignOpen, setAssignOpen] = useState(false)
  const [historyCompany, setHistoryCompany] =
    useState<AssignedCompany | null>(null)

  const { data: salesAccounts = [] } = useQuery({
    queryKey: ['business', 'sales-accounts'],
    queryFn: getSalesAccounts,
  })

  const salesOptions = useMemo(
    () =>
      salesAccounts.map((account) => ({
        value: String(account.user_id),
        label: account.display_name || account.username,
      })),
    [salesAccounts]
  )

  const selectedAccount = useMemo(
    () => salesAccounts.find((account) => String(account.user_id) === salesId),
    [salesAccounts, salesId]
  )

  const { data: companies = [], isLoading } = useQuery({
    queryKey: ['business', 'sales-account-customers', salesId],
    queryFn: () => getAssignedCompanies(salesId),
    enabled: !!salesId,
  })

  const queryClient = useQueryClient()
  const refresh = () => {
    queryClient.invalidateQueries({
      queryKey: ['business', 'sales-account-customers', salesId],
    })
    queryClient.invalidateQueries({ queryKey: ['business', 'sales-accounts'] })
  }

  const handleSalesChange = (value: string) => {
    setSalesId(value)
    const account = salesAccounts.find(
      (item) => String(item.user_id) === value
    )
    void navigate({
      to: '/business/customer-assignments',
      search: {
        sales: value,
        account: account ? account.display_name || account.username : '',
      },
    })
  }

  const columns: StaticDataTableColumn<AssignedCompany>[] = [
    {
      id: 'id',
      header: 'ID',
      cell: (row) => row.id,
    },
    {
      id: 'name',
      header: t('Customer'),
      cell: (row) => row.name,
    },
    {
      id: 'owner',
      header: t('Owner user ID'),
      cell: (row) => row.owner_user_id,
    },
    {
      id: 'created_at',
      header: t('Created at'),
      cell: (row) => (row.created_at ? formatTimestamp(row.created_at) : '-'),
    },
    {
      id: 'actions',
      header: t('Actions'),
      cell: (row) => (
        <div className='flex justify-end'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setHistoryCompany(row)}
          >
            <History className='size-3.5' />
            {t('History')}
          </Button>
        </div>
      ),
    },
  ]

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Customer assignment')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' disabled={!salesId} onClick={() => setAssignOpen(true)}>
            <Link2 className='size-4' />
            {t('Assign customers')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <div className='grid max-w-xl grid-cols-1 gap-4 sm:grid-cols-[1fr_auto] sm:items-end'>
              <div className='space-y-1.5'>
                <Label>{t('Sales account')}</Label>
                <ComboboxInput
                  options={salesOptions}
                  value={salesId}
                  onValueChange={handleSalesChange}
                  placeholder={t('Select a sales account')}
                  emptyText={t('No results')}
                />
              </div>
            </div>

            {!salesId ? (
              <Empty>
                <EmptyTitle>{t('Select a sales account first')}</EmptyTitle>
                <EmptyDescription>
                  {t(
                    'Pick a sales account to view and manage its assigned customers.'
                  )}
                </EmptyDescription>
              </Empty>
            ) : isLoading ? (
              <div className='flex justify-center py-16'>
                <Spinner />
              </div>
            ) : (
              <div className='space-y-2'>
                <div className='text-muted-foreground flex items-center gap-2 text-sm'>
                  <span className='font-medium text-foreground'>
                    {selectedAccount?.display_name ||
                      selectedAccount?.username ||
                      ''}
                  </span>
                  <span>
                    {t('has {{count}} assigned customers', {
                      count: companies.length,
                    })}
                  </span>
                </div>
                {companies.length === 0 ? (
                  <Empty>
                    <EmptyTitle>{t('No assigned customers')}</EmptyTitle>
                    <EmptyDescription>
                      {t('Use Assign customers to add enterprise customers.')}
                    </EmptyDescription>
                  </Empty>
                ) : (
                  <StaticDataTable
                    data={companies}
                    columns={columns}
                    getRowKey={(row) => row.id}
                  />
                )}
              </div>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {salesId && (
        <AssignCustomersDialog
          open={assignOpen}
          onOpenChange={setAssignOpen}
          salesId={salesId}
          salesLabel={
            selectedAccount?.display_name || selectedAccount?.username || ''
          }
          assignedCompanyIds={new Set(companies.map((company) => company.id))}
          onSuccess={refresh}
        />
      )}
      <AssignmentHistoryDialog
        company={historyCompany}
        onClose={() => setHistoryCompany(null)}
      />
    </>
  )
}

function AssignCustomersDialog({
  open,
  onOpenChange,
  salesId,
  salesLabel,
  assignedCompanyIds,
  onSuccess,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  salesId: string
  salesLabel: string
  assignedCompanyIds: Set<number>
  onSuccess: () => void
}) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [pickValue, setPickValue] = useState('')
  const [reason, setReason] = useState('')

  const { data: allCompanies = [], isLoading } = useQuery({
    queryKey: ['business', 'companies', 'options'],
    queryFn: getCompanyOptions,
    staleTime: 30 * 1000,
  })

  useEffect(() => {
    if (!open) {
      setSelectedIds([])
      setPickValue('')
      setReason('')
    }
  }, [open])

  const companyOptions = useMemo(
    () =>
      allCompanies
        .filter(
          (company) =>
            !selectedIds.includes(company.id) &&
            !assignedCompanyIds.has(company.id)
        )
        .map((company) => ({
          value: String(company.id),
          label: `${company.name} (#${company.id})`,
        })),
    [allCompanies, selectedIds, assignedCompanyIds]
  )

  const handleAdd = (value: string) => {
    const id = Number(value)
    if (!Number.isSafeInteger(id) || id <= 0) return
    setSelectedIds((current) =>
      current.includes(id) ? current : [...current, id]
    )
    setPickValue('')
  }

  const handleSubmit = async () => {
    if (selectedIds.length === 0) {
      toast.error(t('Select at least one customer'))
      return
    }
    if (!reason.trim()) {
      toast.error(t('Transfer reason is required'))
      return
    }
    setSubmitting(true)
    try {
      const response = await api.post<ApiEnvelope<unknown>>(
        '/api/business/companies/assignments/batch',
        {
          company_ids: selectedIds,
          sales_user_id: Number(salesId),
          reason: reason.trim(),
        }
      )
      if (!response.data.success) {
        throw new Error(response.data.message || t('Customer assignment failed.'))
      }
      toast.success(t('Customer assignments saved.'))
      onOpenChange(false)
      onSuccess()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Save failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Assign customers')}
      description={t('Assign or transfer enterprise customers to {{sales}}', {
        sales: salesLabel,
      })}
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? t('Processing...') : t('Assign')}
          </Button>
        </>
      }
    >
      <div className='space-y-1.5'>
        <Label>{t('Customer')}</Label>
        {isLoading ? (
          <Spinner />
        ) : (
          <ComboboxInput
            options={companyOptions}
            value={pickValue}
            onValueChange={handleAdd}
            placeholder={t('Search an enterprise customer')}
            emptyText={t('No results')}
            allowCustomValue
          />
        )}
        <p className='text-muted-foreground text-xs'>
          {t(
            'Assigning a customer already owned by another sales account transfers ownership.'
          )}
        </p>
      </div>

      {selectedIds.length > 0 && (
        <div className='flex flex-wrap gap-2'>
          {selectedIds.map((id) => {
            const company = allCompanies.find((item) => item.id === id)
            return (
              <span
                key={id}
                className='bg-secondary text-secondary-foreground inline-flex items-center gap-1 rounded-full px-3 py-1 text-sm'
              >
                {company?.name ?? `#${id}`}
                <button
                  type='button'
                  aria-label={t('Remove')}
                  onClick={() =>
                    setSelectedIds((current) =>
                      current.filter((item) => item !== id)
                    )
                  }
                >
                  <X className='size-3.5' />
                </button>
              </span>
            )
          })}
        </div>
      )}

      <div className='space-y-1.5'>
        <Label>{t('Transfer reason')} *</Label>
        <Textarea
          value={reason}
          onChange={(event) => setReason(event.target.value)}
          placeholder={t('Enter the reason for this assignment')}
        />
      </div>

      {assignedCompanyIds.size > 0 && (
        <p className='text-muted-foreground text-xs'>
          {t(
            '{{count}} customers already assigned to this account are not listed.',
            { count: assignedCompanyIds.size }
          )}
        </p>
      )}
    </Dialog>
  )
}

function AssignmentHistoryDialog({
  company,
  onClose,
}: {
  company: AssignedCompany | null
  onClose: () => void
}) {
  const { t } = useTranslation()
  const { data: history = [], isLoading } = useQuery({
    queryKey: ['business', 'customer-assignment-history', company?.id],
    queryFn: () => getAssignmentHistory(company!.id),
    enabled: !!company,
  })

  return (
    <Dialog
      open={!!company}
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
      title={t('Assignment history')}
      description={company ? `${company.name} (#${company.id})` : undefined}
      bodyClassName='space-y-3'
      footer={
        <Button variant='outline' onClick={onClose}>
          {t('Close')}
        </Button>
      }
    >
      {isLoading ? (
        <div className='flex justify-center py-8'>
          <Spinner />
        </div>
      ) : history.length === 0 ? (
        <Empty>
          <EmptyTitle>{t('No assignment history')}</EmptyTitle>
        </Empty>
      ) : (
        <div className='divide-y rounded-xl border'>
          {history.map((entry) => (
            <div key={entry.id} className='flex items-start gap-3 p-3'>
              <div className='min-w-0 flex-1 space-y-1'>
                <div className='flex items-center gap-2'>
                  {entry.active ? (
                    <Badge variant='default'>{t('Current')}</Badge>
                  ) : (
                    <Badge variant='outline'>{t('Previous')}</Badge>
                  )}
                  <span className='text-sm font-medium'>
                    {t('Sales user')} #{entry.sales_user_id}
                  </span>
                </div>
                {entry.reason && (
                  <p className='text-muted-foreground text-sm'>
                    {entry.reason}
                  </p>
                )}
                <p className='text-muted-foreground text-xs'>
                  {entry.created_at ? formatTimestamp(entry.created_at) : '-'}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </Dialog>
  )
}
