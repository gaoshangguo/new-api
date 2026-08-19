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
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import {
  assignBusinessProjectToken,
  createBusinessProject,
  getBusinessProjectSummary,
  getBusinessProjectTokens,
  updateBusinessProject,
  type BusinessProjectMutation,
} from '../api'
import type { BusinessProject } from '../types'

// P0-06 项目管理的创建/编辑/令牌绑定/报表弹窗。

export function CreateProjectDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  companyId: number
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [budget, setBudget] = useState('0')
  const [lowBalance, setLowBalance] = useState('0')
  const [modelLimits, setModelLimits] = useState('[]')
  const [channelLimits, setChannelLimits] = useState('[]')
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error(t('Project name is required'))
      return
    }
    const payload: BusinessProjectMutation = {
      name: name.trim(),
      budget_quota: Number(budget) || 0,
      low_balance_quota: Number(lowBalance) || 0,
      model_limits: modelLimits.trim() || '[]',
      channel_limits: channelLimits.trim() || '[]',
    }
    setSaving(true)
    try {
      await createBusinessProject(props.companyId, payload)
      toast.success(t('Project created'))
      props.onSaved()
      props.onOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Failed to create project')
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Create project')}
      description={t('A project groups API keys with a shared budget and model controls.')}
      footer={
        <div className='flex justify-end gap-2'>
          <Button type='button' variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={handleSave} disabled={saving}>
            {saving ? t('Saving...') : t('Create')}
          </Button>
        </div>
      }
    >
      <div className='flex flex-col gap-4'>
        <div className='flex flex-col gap-1'>
          <Label>{t('Project name')} *</Label>
          <Input value={name} onChange={(e) => setName(e.currentTarget.value)} />
        </div>
        <div className='grid grid-cols-2 gap-3'>
          <div className='flex flex-col gap-1'>
            <Label>{t('Project budget')}</Label>
            <Input
              type='number'
              value={budget}
              onChange={(e) => setBudget(e.currentTarget.value)}
            />
          </div>
          <div className='flex flex-col gap-1'>
            <Label>{t('Low balance threshold')}</Label>
            <Input
              type='number'
              value={lowBalance}
              onChange={(e) => setLowBalance(e.currentTarget.value)}
            />
          </div>
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Model limits')}</Label>
          <Textarea
            value={modelLimits}
            onChange={(e) => setModelLimits(e.currentTarget.value)}
            rows={2}
            className='font-mono text-xs'
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Channel limits')}</Label>
          <Textarea
            value={channelLimits}
            onChange={(e) => setChannelLimits(e.currentTarget.value)}
            rows={2}
            className='font-mono text-xs'
          />
        </div>
      </div>
    </Dialog>
  )
}

export function EditProjectDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  project: BusinessProject
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [name, setName] = useState(props.project.name)
  const [budget, setBudget] = useState(String(props.project.budget_quota))
  const [lowBalance, setLowBalance] = useState(String(props.project.low_balance_quota))
  const [modelLimits, setModelLimits] = useState(props.project.model_limits || '[]')
  const [channelLimits, setChannelLimits] = useState(props.project.channel_limits || '[]')
  const [reason, setReason] = useState('')
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error(t('Project name is required'))
      return
    }
    if (!reason.trim()) {
      toast.error(t('Reason is required'))
      return
    }
    setSaving(true)
    try {
      await updateBusinessProject(props.project.id, {
        name: name.trim(),
        budget_quota: Number(budget) || 0,
        low_balance_quota: Number(lowBalance) || 0,
        model_limits: modelLimits.trim() || '[]',
        channel_limits: channelLimits.trim() || '[]',
        reason: reason.trim(),
      })
      toast.success(t('Project updated'))
      props.onSaved()
      props.onOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Failed to update project')
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Edit project')}
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
          <Label>{t('Project name')} *</Label>
          <Input value={name} onChange={(e) => setName(e.currentTarget.value)} />
        </div>
        <div className='grid grid-cols-2 gap-3'>
          <div className='flex flex-col gap-1'>
            <Label>{t('Project budget')}</Label>
            <Input
              type='number'
              value={budget}
              onChange={(e) => setBudget(e.currentTarget.value)}
            />
          </div>
          <div className='flex flex-col gap-1'>
            <Label>{t('Low balance threshold')}</Label>
            <Input
              type='number'
              value={lowBalance}
              onChange={(e) => setLowBalance(e.currentTarget.value)}
            />
          </div>
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Model limits')}</Label>
          <Textarea
            value={modelLimits}
            onChange={(e) => setModelLimits(e.currentTarget.value)}
            rows={2}
            className='font-mono text-xs'
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Channel limits')}</Label>
          <Textarea
            value={channelLimits}
            onChange={(e) => setChannelLimits(e.currentTarget.value)}
            rows={2}
            className='font-mono text-xs'
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Reason')} *</Label>
          <Textarea
            value={reason}
            onChange={(e) => setReason(e.currentTarget.value)}
            rows={2}
            placeholder={t('Reason for change')}
          />
        </div>
      </div>
    </Dialog>
  )
}

export function BindTokenDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectId: number
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [tokenId, setTokenId] = useState('')
  const [reason, setReason] = useState('')
  const [saving, setSaving] = useState(false)
  const { data: tokens } = useQuery({
    queryKey: ['project-tokens', props.projectId],
    queryFn: () => getBusinessProjectTokens(props.projectId),
    enabled: props.open && props.projectId > 0,
  })

  const handleBind = async () => {
    const id = Number(tokenId)
    if (!Number.isInteger(id) || id <= 0) {
      toast.error(t('A valid token ID is required'))
      return
    }
    if (!reason.trim()) {
      toast.error(t('Reason is required'))
      return
    }
    setSaving(true)
    try {
      await assignBusinessProjectToken(props.projectId, id, reason.trim())
      toast.success(t('Token bound to project'))
      props.onSaved()
      props.onOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Failed to bind token')
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Bind API key to project')}
      description={t('Binding is permanent for the key: it becomes project-scoped.')}
      footer={
        <div className='flex justify-end gap-2'>
          <Button type='button' variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={handleBind} disabled={saving}>
            {saving ? t('Saving...') : t('Bind')}
          </Button>
        </div>
      }
    >
      <div className='flex flex-col gap-4'>
        <div className='flex flex-col gap-1'>
          <Label>{t('API key ID')}</Label>
          <Input
            type='number'
            value={tokenId}
            onChange={(e) => setTokenId(e.currentTarget.value)}
            placeholder={t('Numeric API key ID')}
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Reason')} *</Label>
          <Textarea
            value={reason}
            onChange={(e) => setReason(e.currentTarget.value)}
            rows={2}
            placeholder={t('Reason for binding')}
          />
        </div>
        {tokens && tokens.length > 0 ? (
          <div className='flex flex-col gap-1'>
            <Label>{t('Bound keys')}</Label>
            <ul className='flex flex-col gap-1 text-sm text-muted-foreground'>
              {tokens.map((token) => (
                <li key={token.id}>
                  #{token.id} · {token.name}
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>
    </Dialog>
  )
}

export function ProjectReportDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectId: number
}) {
  const { t } = useTranslation()
  const { data } = useQuery({
    queryKey: ['project-summary', props.projectId],
    queryFn: () => getBusinessProjectSummary(props.projectId),
    enabled: props.open && props.projectId > 0,
  })

  const items = data
    ? [
        { label: t('Balance'), value: String(data.balance) },
        { label: t('API keys'), value: String(data.token_count) },
        { label: t('Consumed'), value: String(data.consumed_quota) },
        { label: t('Requests'), value: String(data.request_count) },
        { label: t('Errors'), value: String(data.error_count) },
      ]
    : []

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Project report')}
      description={t('Usage and balance summary for this project.')}
    >
      <div className='flex flex-col gap-3'>
        {data ? (
          <div className='grid grid-cols-2 gap-3'>
            {items.map((item) => (
              <div key={item.label} className='rounded-lg border bg-muted/40 p-3'>
                <p className='text-muted-foreground text-xs'>{item.label}</p>
                <p className='mt-1 font-semibold'>{item.value}</p>
              </div>
            ))}
          </div>
        ) : null}
        {data?.alerts && data.alerts.length > 0 ? (
          <div className='flex flex-col gap-2'>
            <Label>{t('Alerts')}</Label>
            {data.alerts.map((alert) => (
              <p key={alert} className='text-sm text-destructive'>
                {alert}
              </p>
            ))}
          </div>
        ) : null}
      </div>
    </Dialog>
  )
}
