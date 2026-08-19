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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import {
  createSelfCompany,
  getMyCompany,
  type CompanyProfile,
  updateSelfCompany,
} from '../../api-self'

// P0-02 企业开户与资料维护：注册后自助填写/更新企业主体信息。

export function EnterpriseProfilePanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const formRef = useRef<HTMLFormElement>(null)
  const [saving, setSaving] = useState(false)

  const { data: company } = useQuery({
    queryKey: ['self-company'],
    queryFn: getMyCompany,
    staleTime: 60 * 1000,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['self-company'] })
  }

  const saveMutation = useMutation({
    mutationFn: (profile: Partial<CompanyProfile>) =>
      company
        ? updateSelfCompany(company.id, profile)
        : createSelfCompany(profile),
    onSuccess: () => {
      toast.success(company ? t('Company profile updated') : t('Company created'))
      invalidate()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save company profile'))
    },
  })

  const handleSave = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!formRef.current) return
    const data = new FormData(formRef.current)
    const name = String(data.get('name') ?? '').trim()
    if (!name) {
      toast.error(t('Company name is required'))
      return
    }
    setSaving(true)
    try {
      await saveMutation.mutateAsync({
        name,
        credit_code: String(data.get('credit_code') ?? '').trim(),
        contact_name: String(data.get('contact_name') ?? '').trim(),
        contact_email: String(data.get('contact_email') ?? '').trim(),
        contact_phone: String(data.get('contact_phone') ?? '').trim(),
        invoice_remark: String(data.get('invoice_remark') ?? '').trim(),
      })
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-base'>{t('Enterprise profile')}</CardTitle>
        <CardDescription>
          {company
            ? t('Keep your billing and contact information up to date.')
            : t('Register your enterprise to access invoicing and dedicated support.')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form ref={formRef} onSubmit={handleSave} className='flex flex-col gap-4'>
          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <div className='flex flex-col gap-1'>
              <Label htmlFor='company-name'>{t('Company name')} *</Label>
              <Input
                id='company-name'
                name='name'
                defaultValue={company?.name ?? ''}
                placeholder={t('Legal entity name')}
              />
            </div>
            <div className='flex flex-col gap-1'>
              <Label htmlFor='credit-code'>{t('Unified social credit code')}</Label>
              <Input
                id='credit-code'
                name='credit_code'
                defaultValue={company?.credit_code ?? ''}
                placeholder={t('e.g. 91310000XXXXXXXX')}
              />
            </div>
            <div className='flex flex-col gap-1'>
              <Label htmlFor='contact-name'>{t('Contact name')}</Label>
              <Input
                id='contact-name'
                name='contact_name'
                defaultValue={company?.contact_name ?? ''}
              />
            </div>
            <div className='flex flex-col gap-1'>
              <Label htmlFor='contact-email'>{t('Contact email')}</Label>
              <Input
                id='contact-email'
                type='email'
                name='contact_email'
                defaultValue={company?.contact_email ?? ''}
              />
            </div>
            <div className='flex flex-col gap-1'>
              <Label htmlFor='contact-phone'>{t('Contact phone')}</Label>
              <Input
                id='contact-phone'
                name='contact_phone'
                defaultValue={company?.contact_phone ?? ''}
              />
            </div>
            <div className='flex flex-col gap-1 md:col-span-2'>
              <Label htmlFor='invoice-remark'>{t('Invoice remark')}</Label>
              <Textarea
                id='invoice-remark'
                name='invoice_remark'
                rows={2}
                defaultValue={company?.invoice_remark ?? ''}
              />
            </div>
          </div>
          <div className='flex justify-end'>
            <Button type='submit' disabled={saving}>
              {saving ? t('Saving...') : t('Save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  )
}
