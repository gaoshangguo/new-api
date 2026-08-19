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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'

type AssignmentForm = {
  companyIds: string
  salesUserId: string
  reason: string
}

function parseCompanyIds(value: string): number[] {
  return value
    .split(/[\s,]+/)
    .filter(Boolean)
    .map(Number)
}

export function CustomerAssignmentsPage() {
  const { t } = useTranslation()
  const formSchema = z.object({
    companyIds: z
      .string()
      .min(1, t('Enterprise IDs are required.'))
      .refine((value) => {
        const ids = parseCompanyIds(value)
        return (
          ids.length > 0 &&
          ids.length <= 100 &&
          ids.every((id) => Number.isSafeInteger(id) && id > 0) &&
          new Set(ids).size === ids.length
        )
      }, t('Enterprise IDs must be positive integers without duplicates.')),
    salesUserId: z
      .string()
      .refine(
        (value) => Number.isSafeInteger(Number(value)) && Number(value) > 0,
        t('Sales user ID must be a positive integer.')
      ),
    reason: z.string().trim().min(1, t('Transfer reason is required.')),
  })
  const form = useForm<AssignmentForm>({
    resolver: zodResolver(formSchema),
    defaultValues: { companyIds: '', salesUserId: '', reason: '' },
  })
  const mutation = useMutation({
    mutationFn: async (values: AssignmentForm) => {
      const response = await api.post('/api/business/companies/assignments/batch', {
        company_ids: parseCompanyIds(values.companyIds),
        sales_user_id: Number(values.salesUserId),
        reason: values.reason.trim(),
      })
      if (!response.data.success) throw new Error(response.data.message)
    },
    onSuccess: () => {
      toast.success(t('Customer assignments saved.'))
      form.reset()
    },
    onError: (error) => {
      toast.error(
        error instanceof Error ? error.message : t('Customer assignment failed.')
      )
    },
  })

  const onSubmit = (values: AssignmentForm) => mutation.mutate(values)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Customer assignment management')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <form
          className='max-w-xl space-y-4 rounded-xl border bg-card p-4'
          onSubmit={form.handleSubmit(onSubmit)}
        >
          <div className='space-y-1.5'>
            <label className='text-sm font-medium' htmlFor='company-ids'>
              {t('Enterprise IDs')}
            </label>
            <Textarea id='company-ids' {...form.register('companyIds')} />
            <p className='text-sm text-muted-foreground'>
              {t('Enter one to 100 enterprise IDs, separated by commas.')}
            </p>
            {form.formState.errors.companyIds && (
              <p className='text-sm text-destructive'>
                {form.formState.errors.companyIds.message}
              </p>
            )}
          </div>
          <div className='space-y-1.5'>
            <label className='text-sm font-medium' htmlFor='sales-user-id'>
              {t('Sales user ID')}
            </label>
            <Input id='sales-user-id' inputMode='numeric' {...form.register('salesUserId')} />
            {form.formState.errors.salesUserId && (
              <p className='text-sm text-destructive'>
                {form.formState.errors.salesUserId.message}
              </p>
            )}
          </div>
          <div className='space-y-1.5'>
            <label className='text-sm font-medium' htmlFor='transfer-reason'>
              {t('Transfer reason')}
            </label>
            <Textarea id='transfer-reason' {...form.register('reason')} />
            {form.formState.errors.reason && (
              <p className='text-sm text-destructive'>
                {form.formState.errors.reason.message}
              </p>
            )}
          </div>
          <Button disabled={mutation.isPending} type='submit'>
            {t('Assign customers')}
          </Button>
        </form>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
