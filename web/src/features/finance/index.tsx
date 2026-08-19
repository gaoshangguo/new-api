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
import { useQueryClient } from '@tanstack/react-query'
import { Download, FileDown, Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { hasPermission } from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

import { USER_ROLE } from '../users/constants'

import { LedgerExportDialog } from './components/ledger-export-dialog'
import { ManualCreditQueue } from './components/manual-credit-queue'
import { ManualCreditRequestDialog } from './components/manual-credit-request-dialog'
import {
  FINANCE_PERMISSION_ACTIONS,
  FINANCE_MANUAL_CREDIT_QUERY_KEY,
  FINANCE_PERMISSION_RESOURCE,
} from './types'

export function FinanceManualCreditsPage() {
  const { t } = useTranslation()
  const currentUser = useAuthStore((state) => state.auth.user)
  const queryClient = useQueryClient()
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isLedgerExportOpen, setIsLedgerExportOpen] = useState(false)
  const [isConsumptionExportOpen, setIsConsumptionExportOpen] = useState(false)
  const canCreate = hasPermission(
    currentUser,
    FINANCE_PERMISSION_RESOURCE,
    FINANCE_PERMISSION_ACTIONS.CREATE
  )
  const canApprove = hasPermission(
    currentUser,
    FINANCE_PERMISSION_RESOURCE,
    FINANCE_PERMISSION_ACTIONS.APPROVE
  )
  const canEscalate = currentUser?.role === USER_ROLE.ROOT

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Balance adjustments')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          size='sm'
          variant='outline'
          onClick={() => setIsLedgerExportOpen(true)}
        >
          <Download className='size-4' />
          {t('Export ledger')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          onClick={() => setIsConsumptionExportOpen(true)}
        >
          <FileDown className='size-4' />
          {t('Export consumption')}
        </Button>
        {canCreate ? (
          <Button size='sm' onClick={() => setIsCreateDialogOpen(true)}>
            <Plus className='size-4' />
            {t('Create balance adjustment')}
          </Button>
        ) : null}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <ManualCreditQueue
          canApprove={canApprove}
          canEscalate={canEscalate}
        />
      </SectionPageLayout.Content>
      <ManualCreditRequestDialog
        open={isCreateDialogOpen}
        onOpenChange={setIsCreateDialogOpen}
        onCreated={() => {
          void queryClient.invalidateQueries({
            queryKey: FINANCE_MANUAL_CREDIT_QUERY_KEY,
          })
        }}
      />
      <LedgerExportDialog
        open={isLedgerExportOpen}
        onOpenChange={setIsLedgerExportOpen}
        report='ledger'
      />
      <LedgerExportDialog
        open={isConsumptionExportOpen}
        onOpenChange={setIsConsumptionExportOpen}
        report='consumption'
      />
    </SectionPageLayout>
  )
}
