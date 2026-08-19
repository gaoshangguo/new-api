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
import { ArrowRight, BriefcaseBusiness, ClipboardCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { hasPermission } from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

import { OperationsStatus } from './components/operations-status'
import { ProjectOverview } from './components/project-overview'
import { SalesWorklist } from './components/sales-worklist'
import {
  BUSINESS_PERMISSION_ACTIONS,
  BUSINESS_PERMISSION_RESOURCES,
} from './types'

export function BusinessWorkbench() {
  const { t } = useTranslation()
  const currentUser = useAuthStore((state) => state.auth.user)
  const canReadCompanies = hasPermission(
    currentUser,
    BUSINESS_PERMISSION_RESOURCES.COMPANY,
    BUSINESS_PERMISSION_ACTIONS.READ
  )
  const canReadProjects = hasPermission(
    currentUser,
    BUSINESS_PERMISSION_RESOURCES.PROJECT,
    BUSINESS_PERMISSION_ACTIONS.READ
  )
  const canReadSales = hasPermission(
    currentUser,
    BUSINESS_PERMISSION_RESOURCES.SALES,
    BUSINESS_PERMISSION_ACTIONS.READ
  )
  const canReadReports = hasPermission(
    currentUser,
    BUSINESS_PERMISSION_RESOURCES.REPORT,
    BUSINESS_PERMISSION_ACTIONS.READ
  )
  const canReadOperations = hasPermission(
    currentUser,
    BUSINESS_PERMISSION_RESOURCES.OPERATIONS,
    BUSINESS_PERMISSION_ACTIONS.READ
  )
  const canReadFinance = hasPermission(
    currentUser,
    BUSINESS_PERMISSION_RESOURCES.FINANCE,
    BUSINESS_PERMISSION_ACTIONS.READ
  )
  const canViewProjects = canReadCompanies && canReadProjects

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Business workspace')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <Card>
            <CardHeader className='gap-3 sm:flex sm:flex-row sm:items-center sm:justify-between'>
              <div className='flex min-w-0 items-start gap-3'>
                <div className='bg-primary/10 text-primary flex size-10 shrink-0 items-center justify-center rounded-lg'>
                  <BriefcaseBusiness className='size-5' />
                </div>
                <div>
                  <CardTitle>{t('Business workbench')}</CardTitle>
                  <CardDescription>
                    {t(
                      'Your available workspace sections are determined by role permissions.'
                    )}
                  </CardDescription>
                </div>
              </div>
              {canReadFinance && (
                <Button
                  size='sm'
                  render={<a href='/finance/manual-credits' />}
                >
                  <ClipboardCheck className='size-4' />
                  {t('Open finance queue')}
                </Button>
              )}
            </CardHeader>
          </Card>

          {canViewProjects && <ProjectOverview />}
          {canReadSales && <SalesWorklist />}
          {(canReadReports || canReadOperations) && (
            <OperationsStatus
              canReadReports={canReadReports}
              canReadOperations={canReadOperations}
            />
          )}
          {canReadFinance && (
            <Card>
              <CardHeader className='gap-3 sm:flex sm:flex-row sm:items-center sm:justify-between'>
                <div>
                  <CardTitle>{t('Balance adjustments')}</CardTitle>
                  <CardDescription>
                    {t('Create and review maker-checker balance adjustments.')}
                  </CardDescription>
                </div>
                <Button
                  size='sm'
                  variant='outline'
                  render={<a href='/finance/manual-credits' />}
                >
                  {t('Open finance queue')}
                  <ArrowRight className='size-4' />
                </Button>
              </CardHeader>
              <CardContent className='text-muted-foreground text-sm'>
                {t(
                  'Adjustment requests remain pending until an authorized reviewer approves or rejects them.'
                )}
              </CardContent>
            </Card>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
