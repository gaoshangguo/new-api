import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatQuota } from '@/lib/format'
import { formatDateTimeObject } from '@/lib/time'

import {
  getMyBalanceLedgers,
  getMyBusinessConsumptions,
  pageItems,
  type CustomerFinanceFilter,
} from './api'

type FilterForm = {
  projectId: string
  tokenId: string
  modelName: string
  startAt: string
  endAt: string
}

const initialFilter: FilterForm = {
  projectId: '',
  tokenId: '',
  modelName: '',
  startAt: '',
  endAt: '',
}

function toFilter(values: FilterForm): CustomerFinanceFilter {
  const projectId = Number(values.projectId)
  const tokenId = Number(values.tokenId)
  const startAt = values.startAt ? new Date(values.startAt).getTime() : 0
  const endAt = values.endAt ? new Date(values.endAt).getTime() : 0
  return {
    project_id: Number.isSafeInteger(projectId) && projectId > 0 ? projectId : undefined,
    token_id: Number.isSafeInteger(tokenId) && tokenId > 0 ? tokenId : undefined,
    model_name: values.modelName.trim() || undefined,
    start_at: startAt > 0 ? Math.floor(startAt / 1000) : undefined,
    end_at: endAt > 0 ? Math.floor(endAt / 1000) : undefined,
  }
}

export function CustomerFinancePage() {
  const { t } = useTranslation()
  const [form, setForm] = useState(initialFilter)
  const [filter, setFilter] = useState<CustomerFinanceFilter>({})
  const ledgerQuery = useQuery({
    queryKey: ['customer-finance', 'ledger', filter],
    queryFn: () => getMyBalanceLedgers(filter),
  })
  const consumptionQuery = useQuery({
    queryKey: ['customer-finance', 'consumption', filter],
    queryFn: () => getMyBusinessConsumptions(filter),
  })
  const ledgers = pageItems(ledgerQuery.data?.data)
  const consumptions = pageItems(consumptionQuery.data?.data)

  const updateForm = (name: keyof FilterForm, value: string) => {
    setForm((previous) => ({ ...previous, [name]: value }))
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Financial details')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <div className='grid gap-3 rounded-xl border bg-card p-4 sm:grid-cols-2 lg:grid-cols-5'>
            <Input inputMode='numeric' placeholder={t('Project ID')} value={form.projectId} onChange={(event) => updateForm('projectId', event.target.value)} />
            <Input inputMode='numeric' placeholder={t('API key ID')} value={form.tokenId} onChange={(event) => updateForm('tokenId', event.target.value)} />
            <Input placeholder={t('Model name')} value={form.modelName} onChange={(event) => updateForm('modelName', event.target.value)} />
            <Input type='datetime-local' aria-label={t('Start time')} value={form.startAt} onChange={(event) => updateForm('startAt', event.target.value)} />
            <Input type='datetime-local' aria-label={t('End time')} value={form.endAt} onChange={(event) => updateForm('endAt', event.target.value)} />
            <Button className='sm:col-span-2 lg:col-span-5' onClick={() => setFilter(toFilter(form))}>{t('Apply filters')}</Button>
          </div>
          <Tabs defaultValue='ledger'>
            <TabsList>
              <TabsTrigger value='ledger'>{t('Ledger')}</TabsTrigger>
              <TabsTrigger value='consumption'>{t('Consumption')}</TabsTrigger>
            </TabsList>
            <TabsContent value='ledger'>
              <Table>
                <TableHeader><TableRow><TableHead>{t('Time')}</TableHead><TableHead>{t('Project ID')}</TableHead><TableHead>{t('API key ID')}</TableHead><TableHead>{t('Entry type')}</TableHead><TableHead>{t('Amount')}</TableHead><TableHead>{t('Balance')}</TableHead></TableRow></TableHeader>
                <TableBody>{ledgers.map((item) => <TableRow key={item.id}><TableCell>{formatDateTimeObject(new Date(item.created_at * 1000))}</TableCell><TableCell>{item.project_id || '-'}</TableCell><TableCell>{item.token_id || '-'}</TableCell><TableCell>{item.entry_type}</TableCell><TableCell>{formatQuota(item.amount)}</TableCell><TableCell>{formatQuota(item.balance_after)}</TableCell></TableRow>)}</TableBody>
              </Table>
            </TabsContent>
            <TabsContent value='consumption'>
              <Table>
                <TableHeader><TableRow><TableHead>{t('Time')}</TableHead><TableHead>{t('Project ID')}</TableHead><TableHead>{t('API key ID')}</TableHead><TableHead>{t('Model')}</TableHead><TableHead>{t('Consumption')}</TableHead></TableRow></TableHeader>
                <TableBody>{consumptions.map((item) => <TableRow key={item.id}><TableCell>{formatDateTimeObject(new Date(item.created_at * 1000))}</TableCell><TableCell>{item.project_id}</TableCell><TableCell>{item.token_id}</TableCell><TableCell>{item.model_name}</TableCell><TableCell>{formatQuota(item.quota)}</TableCell></TableRow>)}</TableBody>
              </Table>
            </TabsContent>
          </Tabs>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
