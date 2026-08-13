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
import dayjs from 'dayjs'
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
import { cn } from '@/lib/utils'

import {
  applyPriceVersionNow,
  createPriceVersion,
  getPriceVersion,
  listPriceVersions,
} from '../api'
import { SettingsSection } from '../components/settings-section'
import type { BillingSettings, PriceVersionDetail } from '../types'

const PRICE_SNAPSHOT_KEYS = [
  { optionKey: 'ModelRatio', jsonField: 'model_ratio' },
  { optionKey: 'ModelPrice', jsonField: 'model_price' },
  { optionKey: 'CompletionRatio', jsonField: 'completion_ratio' },
  { optionKey: 'CacheRatio', jsonField: 'cache_ratio' },
  { optionKey: 'CreateCacheRatio', jsonField: 'create_cache_ratio' },
  { optionKey: 'ImageRatio', jsonField: 'image_ratio' },
  { optionKey: 'AudioRatio', jsonField: 'audio_ratio' },
  { optionKey: 'AudioCompletionRatio', jsonField: 'audio_completion_ratio' },
  { optionKey: 'GroupRatio', jsonField: 'group_ratio' },
  { optionKey: 'GroupGroupRatio', jsonField: 'group_group_ratio' },
] as const

type PriceSnapshotKey = (typeof PRICE_SNAPSHOT_KEYS)[number]['optionKey']

function formatTime(timestamp: number): string {
  if (!timestamp) return '\u2014'
  return dayjs(timestamp * 1000).format('YYYY-MM-DD HH:mm:ss')
}

function toLocalDateTimeInput(timestamp: number): string {
  return dayjs(timestamp * 1000).format('YYYY-MM-DDTHH:mm')
}

function localDateTimeInputToTimestamp(value: string): number | undefined {
  if (!value) return undefined
  const timestamp = new Date(value).getTime()
  return Number.isFinite(timestamp) ? Math.floor(timestamp / 1000) : undefined
}

function prettyJson(value: string): string {
  if (!value) return '{}'
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

function snapshotPreview(version: PriceVersionDetail, maxEntries = 8): string[] {
  const entries: string[] = []
  for (const { optionKey, jsonField } of PRICE_SNAPSHOT_KEYS) {
    const value = version[jsonField]
    if (!value || value === '{}' || value === 'null') continue
    let parsed: Record<string, unknown> = {}
    try {
      parsed = JSON.parse(value) as Record<string, unknown>
    } catch {
      continue
    }
    const names = Object.keys(parsed)
    if (names.length === 0) continue
    if (names.length <= maxEntries) {
      entries.push(`${optionKey}: ${names.join(', ')}`)
    } else {
      entries.push(`${optionKey}: ${names.slice(0, maxEntries).join(', ')} (+${names.length - maxEntries})`)
    }
  }
  return entries
}

type PriceVersionSectionProps = {
  settings: BillingSettings
}

export function PriceVersionSection(props: PriceVersionSectionProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [detail, setDetail] = useState<PriceVersionDetail | null>(null)

  const versionsQuery = useQuery({
    queryKey: ['price-versions'],
    queryFn: listPriceVersions,
  })

  const invalidateVersions = () => {
    queryClient.invalidateQueries({ queryKey: ['price-versions'] })
  }

  const applyMutation = useMutation({
    mutationFn: (versionId: number) => applyPriceVersionNow(versionId),
    onSuccess: () => {
      toast.success(t('Price version applied'))
      invalidateVersions()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to apply price version'))
    },
  })

  const versions = versionsQuery.data?.data ?? []
  // The list is newest first, so the first active row is the current version.
  const currentVersion = versions.find((version) => version.status === 'active')

  return (
    <SettingsSection
      title={t('Price Versions')}
      titleProps={{
        className: 'flex items-center justify-between gap-2',
      }}
    >
      <div className='flex items-center justify-between gap-2'>
        <p className='text-sm text-muted-foreground'>
          {currentVersion
            ? t('Current active price version: #{{versionId}} ({{effectiveTime}})', {
                versionId: currentVersion.id,
                effectiveTime: formatTime(currentVersion.effective_at),
              })
            : t('No price version has been created yet')}
        </p>
        <Button
          type='button'
          onClick={() => setCreateOpen(true)}
          disabled={versionsQuery.isLoading}
        >
          {t('Create price version')}
        </Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('ID')}</TableHead>
            <TableHead>{t('Name')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead>{t('Effective time')}</TableHead>
            <TableHead>{t('Applied time')}</TableHead>
            <TableHead>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {versions.map((version) => (
            <TableRow key={version.id}>
              <TableCell>#{version.id}</TableCell>
              <TableCell>{version.name || `#${version.id}`}</TableCell>
              <TableCell>
                {version.status === 'active' ? (
                  <Badge variant='default'>{t('Active')}</Badge>
                ) : (
                  <Badge variant='secondary'>{t('Pending')}</Badge>
                )}
              </TableCell>
              <TableCell>{formatTime(version.effective_at)}</TableCell>
              <TableCell>{formatTime(version.applied_at)}</TableCell>
              <TableCell>
                <div className='flex items-center gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => {
                      getPriceVersion(version.id)
                        .then((response) => {
                          setDetail(response.data)
                        })
                        .catch((error: Error) => {
                          toast.error(error.message || t('Failed to load price version'))
                        })
                    }}
                  >
                    {t('View snapshot')}
                  </Button>
                  {version.status === 'pending' ? (
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      disabled={applyMutation.isPending}
                      onClick={() => applyMutation.mutate(version.id)}
                    >
                      {t('Apply now')}
                    </Button>
                  ) : null}
                </div>
              </TableCell>
            </TableRow>
          ))}
          {versions.length === 0 ? (
            <TableRow>
              <TableCell colSpan={6} className='text-center text-muted-foreground'>
                {t('No price versions yet')}
              </TableCell>
            </TableRow>
          ) : null}
        </TableBody>
      </Table>

      <CreatePriceVersionDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        settings={props.settings}
        onCreated={invalidateVersions}
      />

      <Dialog
        open={detail !== null}
        onOpenChange={(open) => {
          if (!open) setDetail(null)
        }}
        title={detail ? detail.name || `#${detail.id}` : ''}
        description={
          detail
            ? `${t('Status')}: ${detail.status} 路 ${t('Effective time')}: ${formatTime(detail.effective_at)}`
            : undefined
        }
        contentHeight='70vh'
      >
        {detail ? (
          <div className='flex flex-col gap-4 overflow-auto'>
            <div className='flex flex-col gap-1'>
              {snapshotPreview(detail).map((entry) => (
                <p key={entry} className='text-sm text-muted-foreground'>
                  {entry}
                </p>
              ))}
            </div>
            <div className='flex flex-col gap-3'>
              {PRICE_SNAPSHOT_KEYS.map(({ optionKey, jsonField }) => (
                <div key={optionKey} className='flex flex-col gap-1'>
                  <Label>{optionKey}</Label>
                  <pre className='max-h-48 overflow-auto whitespace-pre-wrap rounded-md bg-muted p-3 text-xs'>
                    {prettyJson(detail[jsonField])}
                  </pre>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </Dialog>
    </SettingsSection>
  )
}

type CreatePriceVersionDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  settings: BillingSettings
  onCreated: () => void
}

function CreatePriceVersionDialog(props: CreatePriceVersionDialogProps) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [effectiveAt, setEffectiveAt] = useState(() =>
    toLocalDateTimeInput(Math.floor(Date.now() / 1000))
  )
  const [reason, setReason] = useState('')
  const [overlays, setOverlays] = useState<Partial<Record<PriceSnapshotKey, string>>>({})
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [errors, setErrors] = useState<Partial<Record<PriceSnapshotKey, string>>>({})
  const [creating, setCreating] = useState(false)

  const handleOpenChange = (open: boolean) => {
    props.onOpenChange(open)
    if (!open) {
      setName('')
      setEffectiveAt(toLocalDateTimeInput(Math.floor(Date.now() / 1000)))
      setReason('')
      setOverlays({})
      setErrors({})
    }
  }

  const updateOverlay = (key: PriceSnapshotKey, value: string) => {
    setOverlays((previous) => ({ ...previous, [key]: value }))
    setErrors((previous) => {
      if (!previous[key]) return previous
      const next = { ...previous }
      delete next[key]
      return next
    })
  }

  const validateOverlays = (): boolean => {
    const nextErrors: Partial<Record<PriceSnapshotKey, string>> = {}
    for (const { optionKey } of PRICE_SNAPSHOT_KEYS) {
      const value = overlays[optionKey]
      if (!value) continue
      try {
        const parsed = JSON.parse(value) as unknown
        if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
          nextErrors[optionKey] = t('Value must be a JSON object')
          continue
        }
        for (const ratio of Object.values(parsed as Record<string, unknown>)) {
          if (typeof ratio !== 'number' || ratio < 0) {
            nextErrors[optionKey] = t('Values must be non-negative numbers')
            break
          }
        }
      } catch {
        nextErrors[optionKey] = t('Invalid JSON')
      }
    }
    setErrors(nextErrors)
    return Object.keys(nextErrors).length === 0
  }

  const handleCreate = async () => {
    if (!validateOverlays()) return
    const effectiveTimestamp = localDateTimeInputToTimestamp(effectiveAt)
    if (!effectiveTimestamp) {
      toast.error(t('A valid effective time is required'))
      return
    }
    const overlay: Record<string, string> = {}
    for (const { optionKey } of PRICE_SNAPSHOT_KEYS) {
      const value = overlays[optionKey]
      if (value && value.trim()) overlay[optionKey] = value.trim()
    }
    setCreating(true)
    try {
      await createPriceVersion({
        name: name.trim() || undefined,
        effective_at: effectiveTimestamp,
        reason: reason.trim() || undefined,
        overlay: Object.keys(overlay).length > 0 ? overlay : undefined,
      })
      toast.success(t('Price version created'))
      props.onCreated()
      handleOpenChange(false)
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('Failed to create price version')
      )
    } finally {
      setCreating(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Create price version')}
      description={t(
        'Snapshot all model prices and group ratios. Empty advanced fields keep the current live values.'
      )}
      footer={
        <div className='flex justify-end gap-2'>
          <Button type='button' variant='outline' onClick={() => handleOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={handleCreate} disabled={creating}>
            {creating ? t('Creating...') : t('Create')}
          </Button>
        </div>
      }
    >
      <div className='flex flex-col gap-4'>
        <div className='flex flex-col gap-1'>
          <Label>{t('Name')}</Label>
          <Input
            value={name}
            onChange={(event) => setName(event.currentTarget.value)}
            placeholder={t('e.g. Summer price adjustment')}
          />
        </div>
        <div className='flex flex-col gap-1'>
          <Label>{t('Effective time')}</Label>
          <Input
            type='datetime-local'
            value={effectiveAt}
            onChange={(event) => setEffectiveAt(event.currentTarget.value)}
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
        <Button
          type='button'
          variant='ghost'
          className='justify-start'
          onClick={() => setShowAdvanced((previous) => !previous)}
        >
          {showAdvanced ? t('Hide advanced settings') : t('Show advanced settings')}
        </Button>
        {showAdvanced ? (
          <div className='flex flex-col gap-3'>
            {PRICE_SNAPSHOT_KEYS.map(({ optionKey }) => (
              <div key={optionKey} className='flex flex-col gap-1'>
                <Label>{optionKey}</Label>
                <Textarea
                  value={overlays[optionKey] ?? prettyJson(props.settings[optionKey] ?? '')}
                  onChange={(event) => updateOverlay(optionKey, event.currentTarget.value)}
                  rows={4}
                  className={cn('font-mono text-xs', errors[optionKey] && 'border-destructive')}
                />
                {errors[optionKey] ? (
                  <p className='text-xs text-destructive'>{errors[optionKey]}</p>
                ) : null}
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </Dialog>
  )
}
