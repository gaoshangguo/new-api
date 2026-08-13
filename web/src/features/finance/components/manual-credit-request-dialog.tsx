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
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { createManualCreditRequest } from '../api'
import {
  getManualCreditFormSchema,
  MANUAL_CREDIT_FORM_DEFAULT_VALUES,
  type ManualCreditFormValues,
} from '../lib/manual-credit-form'
import type { ManualAdjustmentEntryType } from '../types'

type ManualCreditRequestDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}

export function ManualCreditRequestDialog(
  props: ManualCreditRequestDialogProps
) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const form = useForm<ManualCreditFormValues>({
    resolver: zodResolver(getManualCreditFormSchema(t)),
    defaultValues: MANUAL_CREDIT_FORM_DEFAULT_VALUES,
  })
  const entryType = form.watch('entry_type')
  const adjustmentTypeOptions: Array<{
    value: ManualAdjustmentEntryType
    label: string
    description: string
  }> = [
    {
      value: 'manual_credit',
      label: t('Manual credit'),
      description: t("Add the amount to the customer's available balance."),
    },
    {
      value: 'manual_debit',
      label: t('Manual debit'),
      description: t("Deduct the amount from the customer's available balance."),
    },
    {
      value: 'compensation',
      label: t('Compensation'),
      description: t('Add the amount as a customer compensation.'),
    },
    {
      value: 'freeze',
      label: t('Freeze balance'),
      description: t(
        'Move the amount from the available balance to the frozen balance.'
      ),
    },
    {
      value: 'unfreeze',
      label: t('Unfreeze balance'),
      description: t(
        'Move the amount from the frozen balance to the available balance.'
      ),
    },
  ]
  const selectedAdjustmentType =
    adjustmentTypeOptions.find((option) => option.value === entryType) ??
    adjustmentTypeOptions[0]

  const handleOpenChange = (open: boolean) => {
    props.onOpenChange(open)
    if (!open) form.reset(MANUAL_CREDIT_FORM_DEFAULT_VALUES)
  }

  const handleSubmit = async (values: ManualCreditFormValues) => {
    setIsSubmitting(true)
    try {
      const result = await createManualCreditRequest({
        entry_type: values.entry_type,
        user_id: values.user_id,
        project_id: values.project_id,
        amount: values.amount,
        external_reference: values.external_reference,
        reason: values.reason,
        note: values.note?.trim() || undefined,
        invoice_number: values.invoice_number?.trim() || undefined,
        external_finance_reference:
          values.external_finance_reference?.trim() || undefined,
        reconciliation_conclusion:
          values.reconciliation_conclusion?.trim() || undefined,
      })
      if (!result.success) {
        throw new Error(result.message || t('Adjustment request operation failed'))
      }

      toast.success(t('Adjustment request created'))
      handleOpenChange(false)
      props.onCreated()
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Adjustment request operation failed')
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Create balance adjustment')}
      description={t(
        'Create a pending adjustment request after confirming the supporting record.'
      )}
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={isSubmitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            form='manual-credit-request-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Processing...') : t('Create')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id='manual-credit-request-form'
          className='grid gap-4 sm:grid-cols-2'
          onSubmit={(event) => void form.handleSubmit(handleSubmit)(event)}
        >
          <FormField
            control={form.control}
            name='entry_type'
            render={({ field }) => (
              <FormItem className='sm:col-span-2'>
                <FormLabel>{t('Adjustment type')}</FormLabel>
                <Select
                  items={adjustmentTypeOptions}
                  value={field.value}
                  onValueChange={(value) => {
                    if (value) field.onChange(value as ManualAdjustmentEntryType)
                  }}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue placeholder={t('Select an adjustment type')} />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {adjustmentTypeOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {selectedAdjustmentType.description}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='user_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Customer user ID')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='1'
                    inputMode='numeric'
                    value={field.value ?? ''}
                    onChange={(event) =>
                      field.onChange(
                        event.target.value === ''
                          ? undefined
                          : Number(event.target.value)
                      )
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='project_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Project ID (optional)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='1'
                    inputMode='numeric'
                    value={field.value ?? ''}
                    onChange={(event) =>
                      field.onChange(
                        event.target.value === ''
                          ? undefined
                          : Number(event.target.value)
                      )
                    }
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='amount'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Amount')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='1'
                    step='1'
                    inputMode='numeric'
                    value={field.value ?? ''}
                    onChange={(event) =>
                      field.onChange(
                        event.target.value === ''
                          ? undefined
                          : Number(event.target.value)
                      )
                    }
                  />
                </FormControl>
                <FormDescription>
                  {t('Enter a positive integer quota amount.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='external_reference'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('External reference')}</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormDescription>
                  {t('Use an external order number or bank transaction reference.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='reason'
            render={({ field }) => (
              <FormItem className='sm:col-span-2'>
                <FormLabel>{t('Reason')}</FormLabel>
                <FormControl>
                  <Input {...field} />
                </FormControl>
                <FormDescription>
                  {t('Explain why this adjustment is required.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='note'
            render={({ field }) => (
              <FormItem className='sm:col-span-2'>
                <FormLabel>{t('Note')}</FormLabel>
                <FormControl>
                  <Textarea {...field} value={field.value ?? ''} rows={3} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='invoice_number'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Invoice number')}</FormLabel>
                <FormControl>
                  <Input {...field} value={field.value ?? ''} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='external_finance_reference'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('External finance reference')}</FormLabel>
                <FormControl>
                  <Input {...field} value={field.value ?? ''} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='reconciliation_conclusion'
            render={({ field }) => (
              <FormItem className='sm:col-span-2'>
                <FormLabel>{t('Reconciliation conclusion')}</FormLabel>
                <FormControl>
                  <Textarea {...field} value={field.value ?? ''} rows={2} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}
