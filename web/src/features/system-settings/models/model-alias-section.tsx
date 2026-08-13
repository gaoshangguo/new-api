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

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'

import {
  createModelAlias,
  deleteModelAlias,
  listModelAliases,
  updateModelAlias,
} from '../api'
import { SettingsSection } from '../components/settings-section'
import type { ModelAlias, ModelAliasRequest } from '../types'

export function ModelAliasSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<ModelAlias | null>(null)

  const aliasesQuery = useQuery({
    queryKey: ['model-aliases'],
    queryFn: listModelAliases,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['model-aliases'] })
  }

  const deleteMutation = useMutation({
    mutationFn: (alias: ModelAlias) => deleteModelAlias(alias.id),
    onSuccess: () => {
      toast.success(t('Model alias deleted'))
      invalidate()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to delete model alias'))
    },
  })

  const aliases = aliasesQuery.data?.data ?? []

  return (
    <SettingsSection
      title={t('Model Aliases')}
      titleProps={{
        className: 'flex items-center justify-between gap-2',
      }}
    >
      <div className='flex justify-end'>
        <Button
          type='button'
          onClick={() => {
            setEditing(null)
            setDialogOpen(true)
          }}
          disabled={aliasesQuery.isLoading}
        >
          {t('Create model alias')}
        </Button>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Alias name')}</TableHead>
            <TableHead>{t('Target model')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead>{t('Version')}</TableHead>
            <TableHead>{t('Channel IDs')}</TableHead>
            <TableHead>{t('Note')}</TableHead>
            <TableHead>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {aliases.map((alias) => (
            <TableRow key={alias.id}>
              <TableCell className='font-mono text-xs'>{alias.alias_name}</TableCell>
              <TableCell className='font-mono text-xs'>{alias.model_name}</TableCell>
              <TableCell>
                {alias.status === 'active' ? (
                  <Badge variant='default'>{t('Active')}</Badge>
                ) : (
                  <Badge variant='destructive'>{t('Deprecated')}</Badge>
                )}
              </TableCell>
              <TableCell>v{alias.version}</TableCell>
              <TableCell className='font-mono text-xs'>
                {alias.channel_ids || '\u2014'}
              </TableCell>
              <TableCell className='text-xs text-muted-foreground'>
                {alias.note || '\u2014'}
              </TableCell>
              <TableCell>
                <div className='flex items-center gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => {
                      setEditing(alias)
                      setDialogOpen(true)
                    }}
                  >
                    {t('Edit')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={deleteMutation.isPending}
                    onClick={() => {
                      if (window.confirm(t('Delete this model alias?'))) {
                        deleteMutation.mutate(alias)
                      }
                    }}
                  >
                    {t('Delete')}
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          ))}
          {aliases.length === 0 ? (
            <TableRow>
              <TableCell colSpan={7} className='text-center text-muted-foreground'>
                {t('No model aliases yet')}
              </TableCell>
            </TableRow>
          ) : null}
        </TableBody>
      </Table>

      <ModelAliasDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        editing={editing}
        onSaved={invalidate}
      />
    </SettingsSection>
  )
}

type ModelAliasDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  editing: ModelAlias | null
  onSaved: () => void
}

function ModelAliasDialog(props: ModelAliasDialogProps) {
  const { t } = useTranslation()
  const [aliasName, setAliasName] = useState('')
  const [modelName, setModelName] = useState('')
  const [channelIds, setChannelIds] = useState('')
  const [status, setStatus] = useState<'active' | 'deprecated'>('active')
  const [replacement, setReplacement] = useState('')
  const [note, setNote] = useState('')
  const [reason, setReason] = useState('')
  const [saving, setSaving] = useState(false)

  const reset = () => {
    setAliasName(props.editing?.alias_name ?? '')
    setModelName(props.editing?.model_name ?? '')
    setChannelIds(props.editing?.channel_ids ?? '')
    setStatus(props.editing?.status ?? 'active')
    setReplacement(props.editing?.replacement ?? '')
    setNote(props.editing?.note ?? '')
    setReason('')
  }

  const handleOpenChange = (open: boolean) => {
    props.onOpenChange(open)
    if (open) reset()
  }

  const handleSave = async () => {
    if (!aliasName.trim() || !modelName.trim()) {
      toast.error(t('Alias name and target model are required'))
      return
    }
    if (status === 'deprecated' && !replacement.trim()) {
      toast.error(t('A deprecated alias requires a replacement model'))
      return
    }
    if (channelIds.trim()) {
      try {
        const parsed = JSON.parse(channelIds) as unknown
        if (!Array.isArray(parsed) || parsed.some((v) => typeof v !== 'number')) {
          toast.error(t('Channel IDs must be a JSON array of numbers'))
          return
        }
      } catch {
        toast.error(t('Channel IDs must be a JSON array of numbers'))
        return
      }
    }
    const request: ModelAliasRequest = {
      alias_name: aliasName.trim(),
      model_name: modelName.trim(),
      channel_ids: channelIds.trim() || '',
      status,
      replacement: replacement.trim(),
      note: note.trim(),
      reason: reason.trim() || undefined,
    }
    setSaving(true)
    try {
      if (props.editing) {
        await updateModelAlias(props.editing.id, request)
        toast.success(t('Model alias updated'))
      } else {
        await createModelAlias(request)
        toast.success(t('Model alias created'))
      }
      props.onSaved()
      props.onOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Failed to save model alias')
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={props.editing ? t('Edit model alias') : t('Create model alias')}
      description={t(
        'An alias exposes an external model name that resolves to an internal model at request time.'
      )}
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
          <Label>{t('Alias name')}</Label>
          <Input
            value={aliasName}
            onChange={(event) => setAliasName(event.currentTarget.value)}
            placeholder={t('e.g. gpt-4o-fast')}
            disabled={props.editing !== null}
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Target model')}</Label>
          <Input
            value={modelName}
            onChange={(event) => setModelName(event.currentTarget.value)}
            placeholder={t('e.g. gpt-4o-2024-11-20')}
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Status')}</Label>
          <select
            className='h-9 w-full rounded-md border border-input bg-background px-3 text-sm'
            value={status}
            onChange={(event) =>
              setStatus(event.currentTarget.value === 'deprecated' ? 'deprecated' : 'active')
            }
          >
            <option value='active'>{t('Active')}</option>
            <option value='deprecated'>{t('Deprecated')}</option>
          </select>
        </div>
        {status === 'deprecated' ? (
          <div className='flex flex-col gap-1'>
            <Label>{t('Replacement model')}</Label>
            <Input
              value={replacement}
              onChange={(event) => setReplacement(event.currentTarget.value)}
              placeholder={t('e.g. gpt-5')}
            />
          </div>
        ) : null}
        <div className='flex flex-col gap-1'>
          <Label>{t('Channel IDs')}</Label>
          <Input
            value={channelIds}
            onChange={(event) => setChannelIds(event.currentTarget.value)}
            placeholder={t('e.g. [3, 7] (empty = any channel)')}
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Note')}</Label>
          <Textarea
            value={note}
            onChange={(event) => setNote(event.currentTarget.value)}
            rows={2}
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Reason')}</Label>
          <Input
            value={reason}
            onChange={(event) => setReason(event.currentTarget.value)}
            placeholder={t('Optional, recorded in the audit log')}
          />
        </div>
      </div>
    </Dialog>
  )
}
