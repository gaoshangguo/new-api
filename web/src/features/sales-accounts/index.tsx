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
import { useNavigate } from '@tanstack/react-router'
import { Plus, Users } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { Empty, EmptyTitle } from '@/components/ui/empty'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { formatTimestamp } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  SALES_ACCOUNTS_QUERY_KEY,
  createSalesAccount,
  getSalesAccounts,
  setSalesAccountStatus,
  updateSalesAccountProfile,
} from './api'
import type { SalesAccount } from './types'

function StatusCell({ status }: { status: number }) {
  const { t } = useTranslation()
  const enabled = status === 1
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 text-sm',
        enabled ? 'text-emerald-600' : 'text-muted-foreground'
      )}
    >
      <span
        className={cn(
          'size-1.5 rounded-full',
          enabled ? 'bg-emerald-500' : 'bg-muted-foreground'
        )}
      />
      {t(enabled ? 'Enabled' : 'Disabled')}
    </span>
  )
}

export function SalesAccountsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [createOpen, setCreateOpen] = useState(false)
  const [editing, setEditing] = useState<SalesAccount | null>(null)

  const { data: accounts = [], isLoading } = useQuery({
    queryKey: SALES_ACCOUNTS_QUERY_KEY,
    queryFn: getSalesAccounts,
  })

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: SALES_ACCOUNTS_QUERY_KEY })

  const toggleStatus = async (account: SalesAccount) => {
    const next = account.status === 1 ? 2 : 1
    try {
      await setSalesAccountStatus(account.user_id, next)
      toast.success(t('Saved successfully'))
      refresh()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Save failed'))
    }
  }

  const columns: StaticDataTableColumn<SalesAccount>[] = [
    {
      id: 'account',
      header: t('Sales account'),
      cell: (row) => (
        <div className='min-w-0'>
          <div className='font-medium'>{row.display_name || row.username}</div>
          <div className='text-muted-foreground text-xs'>@{row.username}</div>
        </div>
      ),
    },
    {
      id: 'department',
      header: t('Department'),
      cell: (row) => row.department || '-',
    },
    {
      id: 'region',
      header: t('Region'),
      cell: (row) => row.region || '-',
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (row) => <StatusCell status={row.status} />,
    },
    {
      id: 'customers',
      header: t('Customers'),
      cell: (row) => (
        <Button
          variant='ghost'
          size='sm'
          onClick={() =>
            void navigate({
              to: '/business/customer-assignments',
              search: {
                sales: String(row.user_id),
                account: row.display_name || row.username,
              },
            })
          }
        >
          <Users className='size-3.5' />
          {row.customer_count}
        </Button>
      ),
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
        <div className='flex justify-end gap-1'>
          <Button variant='outline' size='sm' onClick={() => setEditing(row)}>
            {t('Edit')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void toggleStatus(row)}
          >
            {t(row.status === 1 ? 'Disable' : 'Enable')}
          </Button>
        </div>
      ),
    },
  ]

  let content: ReactNode
  if (isLoading) {
    content = (
      <div className='flex justify-center py-16'>
        <Spinner />
      </div>
    )
  } else if (accounts.length === 0) {
    content = (
      <Empty>
        <EmptyTitle>{t('No sales accounts yet')}</EmptyTitle>
      </Empty>
    )
  } else {
    content = (
      <StaticDataTable
        data={accounts}
        columns={columns}
        getRowKey={(row) => row.user_id}
      />
    )
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Sales accounts')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button size='sm' onClick={() => setCreateOpen(true)}>
            <Plus className='size-4' />
            {t('New sales account')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
      </SectionPageLayout>

      <SalesAccountCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onSuccess={refresh}
      />
      <SalesAccountEditDialog
        account={editing}
        onClose={() => setEditing(null)}
        onSuccess={refresh}
      />
    </>
  )
}

function SalesAccountCreateDialog({
  open,
  onOpenChange,
  onSuccess,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const [form, setForm] = useState({
    username: '',
    password: '',
    display_name: '',
    email: '',
    department: '',
    region: '',
    note: '',
  })

  useEffect(() => {
    if (!open) return
    setForm({
      username: '',
      password: '',
      display_name: '',
      email: '',
      department: '',
      region: '',
      note: '',
    })
  }, [open])

  const setField = (key: keyof typeof form, value: string) =>
    setForm((current) => ({ ...current, [key]: value }))

  const handleSubmit = async () => {
    if (!form.username.trim() || !form.password) {
      toast.error(t('Username and password are required'))
      return
    }
    if (form.password.length < 8 || form.password.length > 20) {
      toast.error(t('Password must be between 8 and 20 characters'))
      return
    }
    setSubmitting(true)
    try {
      await createSalesAccount(form)
      toast.success(t('Sales account created successfully'))
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
      title={t('New sales account')}
      description={t('Create a sales supervisor account for the sales team.')}
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? t('Processing...') : t('Create')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label>{t('Username')} *</Label>
        <Input
          value={form.username}
          onChange={(event) => setField('username', event.target.value)}
          maxLength={20}
        />
      </div>
      <div className='space-y-2'>
        <Label>{t('Display Name')}</Label>
        <Input
          value={form.display_name}
          onChange={(event) => setField('display_name', event.target.value)}
          maxLength={20}
        />
      </div>
      <div className='space-y-2'>
        <Label>{t('Password')} *</Label>
        <Input
          type='password'
          value={form.password}
          onChange={(event) => setField('password', event.target.value)}
          maxLength={20}
        />
        <p className='text-muted-foreground text-xs'>
          {t('Password must be between 8 and 20 characters')}
        </p>
      </div>
      <div className='space-y-2'>
        <Label>{t('Email')}</Label>
        <Input
          type='email'
          value={form.email}
          onChange={(event) => setField('email', event.target.value)}
          maxLength={50}
        />
      </div>
      <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label>{t('Department')}</Label>
          <Input
            value={form.department}
            onChange={(event) => setField('department', event.target.value)}
            maxLength={128}
          />
        </div>
        <div className='space-y-2'>
          <Label>{t('Region')}</Label>
          <Input
            value={form.region}
            onChange={(event) => setField('region', event.target.value)}
            maxLength={128}
          />
        </div>
      </div>
      <div className='space-y-2'>
        <Label>{t('Note')}</Label>
        <Textarea
          value={form.note}
          onChange={(event) => setField('note', event.target.value)}
        />
      </div>
    </Dialog>
  )
}

function SalesAccountEditDialog({
  account,
  onClose,
  onSuccess,
}: {
  account: SalesAccount | null
  onClose: () => void
  onSuccess: () => void
}) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const [form, setForm] = useState({ department: '', region: '', note: '' })

  useEffect(() => {
    if (!account) return
    setForm({
      department: account.department ?? '',
      region: account.region ?? '',
      note: account.note ?? '',
    })
  }, [account])

  const setField = (key: keyof typeof form, value: string) =>
    setForm((current) => ({ ...current, [key]: value }))

  const handleSubmit = async () => {
    if (!account) return
    setSubmitting(true)
    try {
      await updateSalesAccountProfile(account.user_id, form)
      toast.success(t('Saved successfully'))
      onClose()
      onSuccess()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Save failed'))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog
      open={!!account}
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
      title={`${t('Edit')} · ${account?.display_name || account?.username || ''}`}
      description={t('Update the sales account commercial profile.')}
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={onClose}>
            {t('Cancel')}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? t('Processing...') : t('Save')}
          </Button>
        </>
      }
    >
      <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
        <div className='space-y-2'>
          <Label>{t('Department')}</Label>
          <Input
            value={form.department}
            onChange={(event) => setField('department', event.target.value)}
            maxLength={128}
          />
        </div>
        <div className='space-y-2'>
          <Label>{t('Region')}</Label>
          <Input
            value={form.region}
            onChange={(event) => setField('region', event.target.value)}
            maxLength={128}
          />
        </div>
      </div>
      <div className='space-y-2'>
        <Label>{t('Note')}</Label>
        <Textarea
          value={form.note}
          onChange={(event) => setField('note', event.target.value)}
        />
      </div>
    </Dialog>
  )
}
