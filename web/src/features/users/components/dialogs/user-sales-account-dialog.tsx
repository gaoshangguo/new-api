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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { promoteUserToSalesAccount } from '@/features/sales-accounts/api'

interface UserSalesAccountDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  username: string
  onSuccess?: () => void
}

export function UserSalesAccountDialog({
  open,
  onOpenChange,
  userId,
  username,
  onSuccess,
}: UserSalesAccountDialogProps) {
  const { t } = useTranslation()
  const [submitting, setSubmitting] = useState(false)
  const [form, setForm] = useState({ department: '', region: '', note: '' })

  useEffect(() => {
    if (!open) return
    setForm({ department: '', region: '', note: '' })
  }, [open])

  const setField = (key: keyof typeof form, value: string) =>
    setForm((current) => ({ ...current, [key]: value }))

  const handleSubmit = async () => {
    setSubmitting(true)
    try {
      await promoteUserToSalesAccount(userId, form)
      toast.success(t('Sales account set successfully'))
      onOpenChange(false)
      onSuccess?.()
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
      title={t('Set as sales account')}
      description={t(
        'Grant the sales supervisor role to {{username}} and set up the sales profile.',
        { username }
      )}
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={submitting}>
            {submitting ? t('Processing...') : t('Confirm')}
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
