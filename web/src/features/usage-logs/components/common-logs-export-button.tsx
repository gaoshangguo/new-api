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
import { Download04Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { handleServerError } from '@/lib/handle-server-error'

import { exportUsageLogs } from '../api'
import type { GetLogsParams } from '../types'

interface CommonLogsExportButtonProps {
  params: GetLogsParams
  isAdmin: boolean
}

export function CommonLogsExportButton(props: CommonLogsExportButtonProps) {
  const { t } = useTranslation()
  const mutation = useMutation({
    mutationFn: () => exportUsageLogs(props.params, props.isAdmin),
    onSuccess: (blob) => {
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = 'usage-logs.xlsx'
      try {
        document.body.append(anchor)
        anchor.click()
      } finally {
        anchor.remove()
        URL.revokeObjectURL(url)
      }
    },
    onError: handleServerError,
  })

  return (
    <Button
      type='button'
      variant='outline'
      disabled={mutation.isPending}
      aria-busy={mutation.isPending}
      onClick={() => mutation.mutate()}
    >
      {mutation.isPending ? (
        <Spinner data-icon='inline-start' aria-hidden='true' />
      ) : (
        <HugeiconsIcon
          icon={Download04Icon}
          data-icon='inline-start'
          aria-hidden='true'
        />
      )}
      {mutation.isPending ? t('Processing...') : t('Export')}
    </Button>
  )
}
