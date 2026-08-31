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
import { Code2, Eye, Plus, Trash2 } from 'lucide-react'
import { memo, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table'
import { JsonCodeEditor } from '@/components/json-code-editor'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { ComboboxInput } from '@/components/ui/combobox-input'
import { Field, FieldError } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { TableCell, TableRow } from '@/components/ui/table'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { searchUsers } from '@/features/users/api'

import { useUpdateOption } from '../hooks/use-update-option'
import { SettingsSection } from '../components/settings-section'
import {
  configToRows,
  formatUserModelRatioConfig,
  parseUserModelRatioConfig,
  rowsToConfig,
  type UserModelRatioRow,
} from './user-model-ratio-config'

const OPTION_KEY = 'UserModelRatio'

function useDebouncedValue(value: string, delay = 300): string {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}

type RowEditorProps = {
  row: UserModelRatioRow
  onRowChange: (row: UserModelRatioRow) => void
  onDelete: (rowId: number) => void
}

const UserModelRatioRowEditor = memo(function UserModelRatioRowEditor({
  row,
  onRowChange,
  onDelete,
}: RowEditorProps) {
  const { t } = useTranslation()
  const { models, vendors } = usePricingData()

  const vendorOptions = useMemo(
    () => vendors.map((vendor) => ({ value: String(vendor.id), label: vendor.name })),
    [vendors]
  )
  const modelVendorId = useMemo(() => {
    const model = models.find((m) => m.model_name === row.model)
    return model?.vendor_id != null ? String(model.vendor_id) : ''
  }, [models, row.model])
  const [vendorFilter, setVendorFilter] = useState(modelVendorId)

  const modelOptions = useMemo(() => {
    const candidates = vendorFilter
      ? models.filter((m) => String(m.vendor_id) === vendorFilter)
      : models
    return candidates.map((model) => ({
      value: model.model_name,
      label: model.model_name,
    }))
  }, [models, vendorFilter])

  // 用户搜索：初始以已保存的用户 ID 为关键词，命中后可直接显示用户名。
  const [userKeyword, setUserKeyword] = useState(row.userId)
  const debouncedUserKeyword = useDebouncedValue(userKeyword)
  const userSearch = useQuery({
    queryKey: ['user-model-ratio-user-search', debouncedUserKeyword],
    queryFn: () => searchUsers({ keyword: debouncedUserKeyword, page_size: 10 }),
    enabled: debouncedUserKeyword.trim().length > 0,
  })
  const userOptions = useMemo(
    () =>
      (userSearch.data?.data?.items ?? []).map((user) => ({
        value: String(user.id),
        label: `${user.username} (#${user.id})`,
      })),
    [userSearch.data]
  )

  const ratioInvalid = useMemo(() => {
    const text = row.ratio.trim()
    if (text === '') return false
    const ratio = Number(text)
    return !Number.isFinite(ratio) || ratio < 0
  }, [row.ratio])

  const handleModelChange = useCallback(
    (model: string) => {
      onRowChange({ ...row, model })
      const modelEntry = models.find((m) => m.model_name === model)
      if (modelEntry?.vendor_id != null) {
        setVendorFilter(String(modelEntry.vendor_id))
      }
    },
    [models, onRowChange, row]
  )

  return (
    <TableRow>
      <TableCell className='min-w-[200px]'>
        <ComboboxInput
          options={userOptions}
          value={row.userId}
          allowCustomValue
          placeholder={t('Select user')}
          emptyText={t('No users found')}
          openOnFocus={false}
          onValueChange={(value) => {
            setUserKeyword(value)
            onRowChange({ ...row, userId: value })
          }}
        />
      </TableCell>
      <TableCell className='w-[150px]'>
        <Select
          items={[{ value: '', label: t('All Vendors') }, ...vendorOptions]}
          value={vendorFilter}
          onValueChange={(value) => setVendorFilter(value ?? '')}
        >
          <SelectTrigger className='h-9'>
            <SelectValue placeholder={t('Select a vendor')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              <SelectItem value=''>{t('All Vendors')}</SelectItem>
              {vendorOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </TableCell>
      <TableCell className='min-w-[220px]'>
        <ComboboxInput
          options={modelOptions}
          value={row.model}
          allowCustomValue
          placeholder={t('Select a model')}
          openOnFocus={false}
          onValueChange={handleModelChange}
        />
      </TableCell>
      <TableCell className='w-[140px]'>
        <Field data-invalid={ratioInvalid}>
          <Input
            type='number'
            min={0}
            step={0.1}
            value={row.ratio}
            aria-invalid={ratioInvalid}
            aria-label={`${t('Ratio')}: ${row.model || t('Select a model')}`}
            onChange={(e) => onRowChange({ ...row, ratio: e.target.value })}
          />
          {ratioInvalid ? (
            <FieldError>{t('Ratio must be a non-negative number')}</FieldError>
          ) : null}
        </Field>
      </TableCell>
      <TableCell className='text-right'>
        <Button
          variant='ghost'
          size='icon'
          aria-label={t('Delete')}
          onClick={() => onDelete(row.id)}
        >
          <Trash2 className='text-destructive h-4 w-4' />
        </Button>
      </TableCell>
    </TableRow>
  )
})

type UserModelRatioSectionProps = {
  defaultValue: string
}

export const UserModelRatioSection = memo(function UserModelRatioSection({
  defaultValue,
}: UserModelRatioSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [editMode, setEditMode] = useState<'visual' | 'json'>('visual')
  const [rows, setRows] = useState<UserModelRatioRow[]>([])
  const [jsonText, setJsonText] = useState('')
  const [jsonError, setJsonError] = useState('')
  const nextRowIdRef = useRef(1)

  useEffect(() => {
    const result = parseUserModelRatioConfig(defaultValue)
    const initialRows = result.ok ? configToRows(result.config) : []
    setRows(initialRows)
    setJsonText(
      result.ok
        ? formatUserModelRatioConfig(result.config)
        : defaultValue.trim() || '{}'
    )
    setJsonError(result.ok ? '' : result.error)
    nextRowIdRef.current = initialRows.length + 1
  }, [defaultValue])

  const syncFromRows = useCallback((nextRows: UserModelRatioRow[]) => {
    setRows(nextRows)
    const result = rowsToConfig(nextRows)
    setJsonText(result.ok ? formatUserModelRatioConfig(result.config) : '')
    setJsonError(result.ok ? '' : result.error)
    nextRowIdRef.current =
      nextRows.reduce((maxId, row) => Math.max(maxId, row.id), 0) + 1
  }, [])

  const handleJsonChange = useCallback((text: string) => {
    setJsonText(text)
    const result = parseUserModelRatioConfig(text)
    if (result.ok) {
      setJsonError('')
      const nextRows = configToRows(result.config)
      setRows(nextRows)
      nextRowIdRef.current = nextRows.length + 1
    } else {
      setJsonError(result.error)
    }
  }, [])

  const addRow = useCallback(() => {
    const newRow: UserModelRatioRow = {
      id: nextRowIdRef.current,
      userId: '',
      model: '',
      ratio: '1',
    }
    nextRowIdRef.current += 1
    syncFromRows([...rows, newRow])
  }, [rows, syncFromRows])

  const updateRow = useCallback(
    (updatedRow: UserModelRatioRow) => {
      syncFromRows(
        rows.map((row) => (row.id === updatedRow.id ? updatedRow : row))
      )
    },
    [rows, syncFromRows]
  )

  const removeRow = useCallback(
    (rowId: number) => {
      syncFromRows(rows.filter((row) => row.id !== rowId))
    },
    [rows, syncFromRows]
  )

  const handleSave = useCallback(async () => {
    if (editMode === 'json') {
      const result = parseUserModelRatioConfig(jsonText)
      if (!result.ok) {
        toast.error(result.error)
        return
      }
      await updateOption.mutateAsync({
        key: OPTION_KEY,
        value: JSON.stringify(result.config),
      })
      return
    }
    const result = rowsToConfig(rows)
    if (!result.ok) {
      toast.error(result.error)
      return
    }
    await updateOption.mutateAsync({
      key: OPTION_KEY,
      value: JSON.stringify(result.config),
    })
  }, [editMode, jsonText, rows, updateOption])

  const toggleEditMode = useCallback(() => {
    setEditMode((prev) => (prev === 'visual' ? 'json' : 'visual'))
  }, [])

  return (
    <SettingsSection title={t('User Model Discounts')}>
      <Alert>
        <AlertDescription className='space-y-1 text-sm'>
          <div>
            {t(
              'Give a specific user a per-model ratio override. The override fully replaces the group ratio for that user and model (it is not applied on top of it). A ratio of 0 makes the model free for that user.'
            )}
          </div>
          <div>
            <span className='font-medium'>{t('Format')}:</span>{' '}
            <code className='bg-muted rounded px-1 py-0.5 text-xs'>
              {'{"userId": {"modelName": 0.9}}'}
            </code>
          </div>
        </AlertDescription>
      </Alert>

      <div className='flex flex-wrap items-center justify-between gap-2'>
        {editMode === 'visual' ? (
          <Button variant='outline' size='sm' onClick={addRow}>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add')}
          </Button>
        ) : (
          <span />
        )}
        <Button variant='outline' size='sm' onClick={toggleEditMode}>
          {editMode === 'visual' ? (
            <>
              <Code2 className='mr-2 h-4 w-4' />
              {t('Switch to JSON')}
            </>
          ) : (
            <>
              <Eye className='mr-2 h-4 w-4' />
              {t('Switch to Visual')}
            </>
          )}
        </Button>
      </div>

      {editMode === 'visual' ? (
        <StaticDataTable
          data={rows}
          getRowKey={(row) => row.id}
          emptyClassName='text-muted-foreground py-8'
          emptyContent={t('No user model discounts configured')}
          columns={[
            { id: 'user', header: t('User') },
            { id: 'vendor', header: t('Vendor') },
            { id: 'model', header: t('Model') },
            { id: 'ratio', header: t('Ratio') },
            { id: 'actions', header: t('Actions'), className: 'text-right' },
          ]}
          renderRow={(row) => (
            <UserModelRatioRowEditor
              row={row}
              onRowChange={updateRow}
              onDelete={removeRow}
            />
          )}
        />
      ) : (
        <div className='space-y-2'>
          <JsonCodeEditor
            value={jsonText}
            onChange={handleJsonChange}
            heightClassName='h-72 min-h-72 max-h-72'
            aria-invalid={Boolean(jsonError)}
          />
          {jsonError && <p className='text-destructive text-sm'>{jsonError}</p>}
        </div>
      )}

      <div className='flex justify-end'>
        <Button
          onClick={handleSave}
          disabled={
            updateOption.isPending ||
            (editMode === 'json' && Boolean(jsonError))
          }
        >
          {t('Save user model ratios')}
        </Button>
      </div>
    </SettingsSection>
  )
})