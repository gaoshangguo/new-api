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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import dayjs from '@/lib/dayjs'

import {
  createSalesCustomerFollowUp,
  getSalesCustomerFollowUps,
  updateSalesCustomerFollowUp,
} from '../api'
import type { BusinessFollowUp } from '../types'

function followUpQueryKey(companyId: number) {
  return ['business', 'sales', 'customers', companyId, 'follow-ups']
}

export function SalesFollowUps({ companyId }: { companyId: number }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const followUpsQuery = useQuery({
    queryKey: followUpQueryKey(companyId),
    queryFn: () => getSalesCustomerFollowUps(companyId),
    enabled: companyId > 0,
  })

  const toggleMutation = useMutation({
    mutationFn: (followUp: BusinessFollowUp) =>
      updateSalesCustomerFollowUp(followUp.id, {
        title: followUp.title,
        note: followUp.note,
        status: followUp.status === 'open' ? 'done' : 'open',
        due_at: followUp.due_at,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: followUpQueryKey(companyId) })
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : t('Save failed'))
    },
  })

  const followUps = followUpsQuery.data ?? []

  return (
    <div>
      <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
        <p className='text-sm font-medium'>{t('Follow-ups')}</p>
        <Button
          type='button'
          size='sm'
          variant='outline'
          onClick={() => setDialogOpen(true)}
        >
          {t('Add follow-up')}
        </Button>
      </div>
      {followUpsQuery.isLoading && (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      )}
      {!followUpsQuery.isLoading && followUps.length === 0 && (
        <p className='text-muted-foreground text-sm'>{t('No follow-ups yet.')}</p>
      )}
      {!followUpsQuery.isLoading && followUps.length > 0 && (
        <div className='space-y-2'>
          {followUps.map((followUp) => (
            <div
              key={followUp.id}
              className='flex items-start justify-between gap-3 rounded-lg border px-3 py-2 text-sm'
            >
              <div className='min-w-0'>
                <div className='flex flex-wrap items-center gap-2'>
                  <p
                    className={
                      followUp.status === 'done'
                        ? 'text-muted-foreground line-through'
                        : 'font-medium'
                    }
                  >
                    {followUp.title}
                  </p>
                  {followUp.due_at > 0 && (
                    <span className='text-muted-foreground text-xs'>
                      {dayjs(followUp.due_at * 1000).format('YYYY-MM-DD')}
                    </span>
                  )}
                </div>
                {followUp.note && (
                  <p className='text-muted-foreground mt-1 text-xs line-clamp-2'>
                    {followUp.note}
                  </p>
                )}
              </div>
              <Button
                type='button'
                size='sm'
                variant='ghost'
                disabled={toggleMutation.isPending}
                onClick={() => toggleMutation.mutate(followUp)}
              >
                {followUp.status === 'open' ? t('Mark done') : t('Reopen')}
              </Button>
            </div>
          ))}
        </div>
      )}
      <AddFollowUpDialog
        companyId={companyId}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onCreated={() => {
          void queryClient.invalidateQueries({
            queryKey: followUpQueryKey(companyId),
          })
        }}
      />
    </div>
  )
}

function AddFollowUpDialog(props: {
  companyId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}) {
  const { t } = useTranslation()
  const [title, setTitle] = useState('')
  const [note, setNote] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    if (!title.trim()) {
      toast.error(t('Title is required'))
      return
    }
    setSaving(true)
    try {
      await createSalesCustomerFollowUp(props.companyId, {
        title: title.trim(),
        note: note.trim() || undefined,
        due_at: dueDate
          ? Math.floor(new Date(`${dueDate}T00:00:00`).getTime() / 1000)
          : undefined,
      })
      toast.success(t('Follow-up saved.'))
      setTitle('')
      setNote('')
      setDueDate('')
      props.onCreated()
      props.onOpenChange(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Save failed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Add follow-up')}
      description={t('Record the next action to keep this customer moving.')}
      footer={
        <div className='flex justify-end gap-2'>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={handleSave} disabled={saving}>
            {saving ? t('Saving...') : t('Save')}
          </Button>
        </div>
      }
    >
      <div className='flex flex-col gap-4'>
        <div className='flex flex-col gap-1'>
          <Label>{t('Title')} *</Label>
          <Input value={title} onChange={(e) => setTitle(e.currentTarget.value)} />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Note')}</Label>
          <Textarea value={note} onChange={(e) => setNote(e.currentTarget.value)} />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Due date')}</Label>
          <Input
            type='date'
            value={dueDate}
            onChange={(e) => setDueDate(e.currentTarget.value)}
          />
        </div>
      </div>
    </Dialog>
  )
}
