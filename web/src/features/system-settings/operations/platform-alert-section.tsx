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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { api } from '@/lib/api'
import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { SettingsSection } from '../components/settings-section'

// P0-29 平台级告警规则：阈值规则 CRUD + 评估事件列表。

export type PlatformAlertRule = {
  id: number
  name: string
  metric: string
  operator: string
  threshold: number
  window_minutes: number
  enabled: boolean
  notify_enabled: boolean
  reason: string
  created_at: number
}

export type PlatformAlertEvent = {
  id: number
  rule_id: number
  rule_name: string
  metric: string
  value: number
  threshold: number
  operator: string
  status: 'active' | 'resolved'
  message: string
  created_at: number
  resolved_at: number
}

type RuleListResponse = { success: boolean; data: PlatformAlertRule[] }
type EventListResponse = { success: boolean; data: PlatformAlertEvent[]; total: number }

const METRICS = [
  { value: 'error_rate', labelKey: 'Error rate' },
  { value: 'disabled_channels', labelKey: 'Disabled channels' },
  { value: 'auto_disabled_channels', labelKey: 'Auto-disabled channels' },
  { value: 'request_count_1h', labelKey: 'Requests (1h)' },
]

export function PlatformAlertSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<PlatformAlertRule | null>(null)

  const rulesQuery = useQuery({
    queryKey: ['platform-alert-rules'],
    queryFn: async () => (await api.get<RuleListResponse>('/api/platform_alert/rules')).data,
  })
  const eventsQuery = useQuery({
    queryKey: ['platform-alert-events'],
    queryFn: async () =>
      (await api.get<EventListResponse>('/api/platform_alert/events', { params: { page: 0, page_size: 50 } })).data,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['platform-alert-rules'] })
    queryClient.invalidateQueries({ queryKey: ['platform-alert-events'] })
  }

  const evalMutation = useMutation({
    mutationFn: () => api.post('/api/platform_alert/eval'),
    onSuccess: () => {
      toast.success(t('Alert evaluation completed'))
      invalidate()
    },
    onError: (error: Error) => toast.error(error.message || t('Alert evaluation failed')),
  })

  const deleteMutation = useMutation({
    mutationFn: (rule: PlatformAlertRule) => api.delete(`/api/platform_alert/rules/${rule.id}`),
    onSuccess: () => {
      toast.success(t('Alert rule deleted'))
      invalidate()
    },
    onError: (error: Error) => toast.error(error.message || t('Failed to delete alert rule')),
  })

  const rules = rulesQuery.data?.data ?? []
  const events = eventsQuery.data?.data ?? []
  const fmtTime = (ts: number) => new Date(ts * 1000).toLocaleString()

  return (
    <SettingsSection title={t('Platform Alerts')}>
      <div className='flex items-center justify-between gap-2'>
        <p className='text-muted-foreground text-sm'>
          {t('Threshold rules evaluated by the scheduled platform alert task.')}
        </p>
        <div className='flex items-center gap-2'>
          <Button type='button' variant='outline' onClick={() => evalMutation.mutate()} disabled={evalMutation.isPending}>
            {t('Evaluate now')}
          </Button>
          <Button type='button' onClick={() => { setEditing(null); setDialogOpen(true) }}>
            {t('Create alert rule')}
          </Button>
        </div>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Name')}</TableHead>
            <TableHead>{t('Metric')}</TableHead>
            <TableHead>{t('Condition')}</TableHead>
            <TableHead>{t('Window (min)')}</TableHead>
            <TableHead>{t('Notify')}</TableHead>
            <TableHead>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rules.map((rule) => (
            <TableRow key={rule.id}>
              <TableCell>
                <span className='flex items-center gap-2'>
                  {rule.name}
                  {rule.enabled ? <Badge variant='default'>{t('Enabled')}</Badge> : <Badge variant='secondary'>{t('Disabled')}</Badge>}
                </span>
              </TableCell>
              <TableCell className='font-mono text-xs'>{rule.metric}</TableCell>
              <TableCell className='font-mono text-xs'>
                {rule.operator} {rule.threshold}
              </TableCell>
              <TableCell>{rule.window_minutes}</TableCell>
              <TableCell>{rule.notify_enabled ? t('Yes') : t('No')}</TableCell>
              <TableCell>
                <div className='flex items-center gap-2'>
                  <Button type='button' variant='outline' size='sm' onClick={() => { setEditing(rule); setDialogOpen(true) }}>
                    {t('Edit')}
                  </Button>
                  <Button type='button' variant='outline' size='sm' onClick={() => deleteMutation.mutate(rule)}>
                    {t('Delete')}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
          {rules.length === 0 ? (
            <TableRow>
              <TableCell colSpan={6} className='text-center text-muted-foreground'>
                {t('No alert rules yet')}
              </TableCell>
            </TableRow>
          ) : null}
        </TableBody>
      </Table>

      <div className='mt-6'>
        <h4 className='mb-2 text-sm font-medium'>{t('Alert events')}</h4>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Rule')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Value / threshold')}</TableHead>
              <TableHead>{t('Time')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {events.slice(0, 20).map((event) => (
              <TableRow key={event.id}>
                <TableCell>{event.rule_name}</TableCell>
                <TableCell>
                  {event.status === 'active' ? (
                    <Badge variant='destructive'>{t('Active')}</Badge>
                  ) : (
                    <Badge variant='secondary'>{t('Resolved')}</Badge>
                  )}
                </TableCell>
                <TableCell className='font-mono text-xs'>
                  {event.value.toFixed(4)} {event.operator} {event.threshold}
                </TableCell>
                <TableCell>{fmtTime(event.created_at)}</TableCell>
              </TableRow>
            ))}
            {events.length === 0 ? (
              <TableRow>
                <TableCell colSpan={4} className='text-center text-muted-foreground'>
                  {t('No alert events yet')}
                </TableCell>
              </TableRow>
            ) : null}
          </TableBody>
        </Table>
      </div>

      <AlertRuleDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        editing={editing}
        onSaved={invalidate}
      />
    </SettingsSection>
  )
}

function AlertRuleDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  editing: PlatformAlertRule | null
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [name, setName] = useState(props.editing?.name ?? '')
  const [metric, setMetric] = useState(props.editing?.metric ?? 'error_rate')
  const [operator, setOperator] = useState(props.editing?.operator ?? '>')
  const [threshold, setThreshold] = useState(String(props.editing?.threshold ?? 0.5))
  const [window, setWindow] = useState(String(props.editing?.window_minutes ?? 10))
  const [enabled, setEnabled] = useState(props.editing?.enabled ?? true)
  const [notifyEnabled, setNotifyEnabled] = useState(props.editing?.notify_enabled ?? false)
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error(t('Rule name is required'))
      return
    }
    const payload = {
      name: name.trim(),
      metric,
      operator,
      threshold: Number(threshold),
      window_minutes: Number(window) || 10,
      enabled,
      notify_enabled: notifyEnabled,
    }
    setSaving(true)
    try {
      if (props.editing) {
        await api.put(`/api/platform_alert/rules/${props.editing.id}`, payload)
        toast.success(t('Alert rule updated'))
      } else {
        await api.post('/api/platform_alert/rules', payload)
        toast.success(t('Alert rule created'))
      }
      props.onSaved()
      props.onOpenChange(false)
    } catch (error) {
      toast.error(error instanceof Error && error.message ? error.message : t('Failed to save alert rule'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.editing ? t('Edit alert rule') : t('Create alert rule')}
      footer={
        <div className='flex justify-end gap-2'>
          <Button type='button' variant='outline' onClick={() => props.onOpenChange(false)}>
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
          <Label>{t('Name')}</Label>
          <Input value={name} onChange={(e) => setName(e.currentTarget.value)} />
        </div>
        <div className='grid grid-cols-2 gap-3'>
          <div className='flex flex-col gap-1'>
            <Label>{t('Metric')}</Label>
            <select
              className='h-9 w-full rounded-md border border-input bg-background px-3 text-sm'
              value={metric}
              onChange={(e) => setMetric(e.currentTarget.value)}
            >
              {METRICS.map((item) => (
                <option key={item.value} value={item.value}>
                  {t(item.labelKey)}
                </option>
              ))}
            </select>
          </div>
          <div className='flex flex-col gap-1'>
            <Label>{t('Operator')}</Label>
            <select
              className='h-9 w-full rounded-md border border-input bg-background px-3 text-sm'
              value={operator}
              onChange={(e) => setOperator(e.currentTarget.value)}
            >
              {['>', '>=', '<', '<='].map((op) => (
                <option key={op} value={op}>
                  {op}
                </option>
              ))}
            </select>
          </div>
          <div className='flex flex-col gap-1'>
            <Label>{t('Threshold')}</Label>
            <Input
              type='number'
              step='any'
              value={threshold}
              onChange={(e) => setThreshold(e.currentTarget.value)}
            />
          </div>
          <div className='flex flex-col gap-1'>
            <Label>{t('Window (minutes)')}</Label>
            <Input type='number' value={window} onChange={(e) => setWindow(e.currentTarget.value)} />
          </div>
        </div>
        <div className='flex flex-col gap-2'>
          <div className='flex items-center justify-between'>
            <Label>{t('Enabled')}</Label>
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </div>
          <div className='flex items-center justify-between'>
            <Label>{t('Notify on trigger')}</Label>
            <Switch checked={notifyEnabled} onCheckedChange={setNotifyEnabled} />
          </div>
        </div>
      </div>
    </Dialog>
  )
}
