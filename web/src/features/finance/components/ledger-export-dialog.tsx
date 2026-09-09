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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { ComboboxInput } from '@/components/ui/combobox-input'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  exportBusinessConsumptions,
  exportBalanceLedgers,
  exportSalesCustomerConsumptions,
  exportSalesCustomerLedgers,
  getSalesCustomerExportModelOptions,
  type BalanceLedgerExportFilters,
} from '../api'

type LedgerExportDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  report: 'ledger' | 'consumption'
  // P0-20：销售视角导出时传入归属客户公司 ID，走销售限定端点；
  // 不传则为平台级财务导出（需 BusinessLedgerRead 权限）。
  companyId?: number
}

type LedgerExportForm = {
  userId: string
  projectId: string
  tokenId: string
  entryType: string
  modelName: string
  startAt: string
  endAt: string
}

const DEFAULT_FILTERS: LedgerExportForm = {
  userId: '',
  projectId: '',
  tokenId: '',
  entryType: '',
  modelName: '',
  startAt: '',
  endAt: '',
}

const ENTRY_TYPE_ALL = '__all__'

// Balance ledger entry types are a fixed business vocabulary (see
// model/business.go LedgerEntry* constants), so they render as a selectable
// list instead of free text.
const LEDGER_ENTRY_TYPES = [
  { value: 'manual_credit', labelKey: 'Manual credit' },
  { value: 'manual_debit', labelKey: 'Manual debit' },
  { value: 'compensation', labelKey: 'Compensation' },
  { value: 'freeze', labelKey: 'Freeze' },
  { value: 'unfreeze', labelKey: 'Unfreeze' },
  { value: 'opening_balance', labelKey: 'Opening balance' },
  { value: 'consumption', labelKey: 'Consumption' },
  { value: 'consumption_refund', labelKey: 'Consumption refund' },
]

function optionalPositiveInt(value: string): number | undefined {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined
}

function optionalUnixTime(value: string): number | undefined {
  if (!value) return undefined
  const timestamp = new Date(value).getTime()
  return Number.isFinite(timestamp) ? Math.floor(timestamp / 1000) : undefined
}

export function LedgerExportDialog(props: LedgerExportDialogProps) {
  const { t } = useTranslation()
  const [filters, setFilters] = useState(DEFAULT_FILTERS)
  const [isExporting, setIsExporting] = useState(false)
  const isConsumption = props.report === 'consumption'
  const exportLabel = isConsumption ? t('Export consumption') : t('Export ledger')
  const isCompanyScoped = props.companyId !== undefined && props.companyId > 0

  // Sales exports can restrict by the model names that actually exist for the
  // assigned customer; platform exports keep free-text model filtering.
  const { data: customerModels = [] } = useQuery({
    queryKey: [
      'business',
      'sales',
      'customers',
      props.companyId,
      'export-model-options',
    ],
    queryFn: () => getSalesCustomerExportModelOptions(props.companyId as number),
    enabled: isCompanyScoped,
  })
  const modelOptions = [
    { value: '', label: t('All') },
    ...customerModels.map((name) => ({ value: name, label: name })),
  ]

  const handleOpenChange = (open: boolean) => {
    props.onOpenChange(open)
    if (!open) setFilters(DEFAULT_FILTERS)
  }

  const updateFilter = (name: keyof LedgerExportForm, value: string) => {
    setFilters((previous) => ({ ...previous, [name]: value }))
  }

  const handleExport = async () => {
    const startAt = optionalUnixTime(filters.startAt)
    const endAt = optionalUnixTime(filters.endAt)
    if (startAt && endAt && startAt > endAt) {
      toast.error(t('Start time must not be after end time'))
      return
    }
    const payload: BalanceLedgerExportFilters = {
      user_id: optionalPositiveInt(filters.userId),
      project_id: optionalPositiveInt(filters.projectId),
      token_id: optionalPositiveInt(filters.tokenId),
      entry_type: filters.entryType.trim() || undefined,
      model_name: filters.modelName.trim() || undefined,
      start_at: startAt,
      end_at: endAt,
    }
    setIsExporting(true)
    try {
      let blob: Blob
      if (isCompanyScoped) {
        const companyId = props.companyId
        if (companyId === undefined) {
          toast.error(t('Company is required'))
          return
        }
        blob = isConsumption
          ? await exportSalesCustomerConsumptions(companyId, payload)
          : await exportSalesCustomerLedgers(companyId, payload)
      } else {
        blob = isConsumption
          ? await exportBusinessConsumptions(payload)
          : await exportBalanceLedgers(payload)
      }
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = isConsumption
        ? 'business-consumption.csv'
        : 'balance-ledger.csv'
      document.body.append(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
      handleOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Ledger export failed')
      )
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={exportLabel}
      description={
        isConsumption
          ? t('Use optional filters to export up to 10,000 matching consumption records.')
          : t('Use optional filters to export up to 10,000 matching ledger entries.')
      }
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={isExporting}
            onClick={() => handleOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' disabled={isExporting} onClick={() => void handleExport()}>
            {isExporting ? t('Processing...') : exportLabel}
          </Button>
        </>
      }
    >
      <div className='grid gap-4 sm:grid-cols-2'>
        {!isCompanyScoped ? (
          <>
            <div className='grid gap-2'>
              <Label htmlFor='ledger-export-user-id'>{t('User ID')}</Label>
              <Input id='ledger-export-user-id' inputMode='numeric' value={filters.userId} onChange={(event) => updateFilter('userId', event.target.value)} />
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='ledger-export-project-id'>{t('Project ID')}</Label>
              <Input id='ledger-export-project-id' inputMode='numeric' value={filters.projectId} onChange={(event) => updateFilter('projectId', event.target.value)} />
            </div>
            <div className='grid gap-2'>
              <Label htmlFor='ledger-export-token-id'>{t('API key ID')}</Label>
              <Input id='ledger-export-token-id' inputMode='numeric' value={filters.tokenId} onChange={(event) => updateFilter('tokenId', event.target.value)} />
            </div>
          </>
        ) : null}
        <div className='grid gap-2'>
          <Label htmlFor='ledger-export-model'>{t('Model name')}</Label>
          <ComboboxInput
            id='ledger-export-model'
            options={modelOptions}
            value={filters.modelName}
            onValueChange={(value) => updateFilter('modelName', value)}
            placeholder={t('All')}
            emptyText={t('No results')}
            allowCustomValue
          />
        </div>
        {!isConsumption ? (
          <div className='grid gap-2'>
            <Label htmlFor='ledger-export-entry-type'>{t('Entry type')}</Label>
            <Select
              items={[
                { value: ENTRY_TYPE_ALL, label: t('All') },
                ...LEDGER_ENTRY_TYPES.map((entry) => ({
                  value: entry.value,
                  label: t(entry.labelKey),
                })),
              ]}
              value={filters.entryType || ENTRY_TYPE_ALL}
              onValueChange={(value) =>
                updateFilter(
                  'entryType',
                  value === ENTRY_TYPE_ALL ? '' : (value ?? '')
                )
              }
            >
              <SelectTrigger id='ledger-export-entry-type'>
                <SelectValue placeholder={t('All')} />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectItem value={ENTRY_TYPE_ALL}>{t('All')}</SelectItem>
                  {LEDGER_ENTRY_TYPES.map((entry) => (
                    <SelectItem key={entry.value} value={entry.value}>
                      {t(entry.labelKey)}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        ) : null}
        <div className='grid gap-2'>
          <Label htmlFor='ledger-export-start'>{t('Start time')}</Label>
          <Input id='ledger-export-start' type='datetime-local' value={filters.startAt} onChange={(event) => updateFilter('startAt', event.target.value)} />
        </div>
        <div className='grid gap-2'>
          <Label htmlFor='ledger-export-end'>{t('End time')}</Label>
          <Input id='ledger-export-end' type='datetime-local' value={filters.endAt} onChange={(event) => updateFilter('endAt', event.target.value)} />
        </div>
      </div>
    </Dialog>
  )
}
