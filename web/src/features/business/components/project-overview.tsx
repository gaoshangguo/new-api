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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'
import type { BusinessProject } from '../types'

import {
  getBusinessCompanies,
  getCompanyProjects,
} from '../api'
import {
  BindTokenDialog,
  CreateProjectDialog,
  EditProjectDialog,
  ProjectReportDialog,
} from './project-management-dialogs'

export function ProjectOverview() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const [selectedCompanyId, setSelectedCompanyId] = useState<number | null>(
    null
  )
  const companiesQuery = useQuery({
    queryKey: ['business', 'companies'],
    queryFn: getBusinessCompanies,
    staleTime: 60 * 1000,
  })
  const companyData = companiesQuery.data
  const companyItems = companyData ?? []

  useEffect(() => {
    if (
      companyData &&
      companyData.length > 0 &&
      !companyData.some((company) => company.id === selectedCompanyId)
    ) {
      setSelectedCompanyId(companyData[0].id)
    }
  }, [companyData, selectedCompanyId])

  const [createOpen, setCreateOpen] = useState(false)
  const [editingProject, setEditingProject] = useState<BusinessProject | null>(null)
  const [bindProject, setBindProject] = useState<BusinessProject | null>(null)
  const [reportProject, setReportProject] = useState<BusinessProject | null>(null)
  const invalidateProjects = () => {
    queryClient.invalidateQueries({
      queryKey: ['business', 'projects', selectedCompanyId],
    })
  }

  const projectsQuery = useQuery({
    queryKey: ['business', 'companies', selectedCompanyId, 'projects'],
    queryFn: () => getCompanyProjects(selectedCompanyId ?? 0),
    enabled: selectedCompanyId !== null,
    staleTime: 60 * 1000,
  })

  if (companiesQuery.isLoading) {
    return <ProjectOverviewSkeleton />
  }

  if (companiesQuery.isError) {
    return (
      <ErrorState
        className='min-h-56'
        title={t('Unable to load business data')}
        description={t('Try refreshing this panel.')}
        onRetry={() => void companiesQuery.refetch()}
      />
    )
  }

  if (companyItems.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className='flex items-center justify-between gap-2'>
          {t('Project overview')}
          {selectedCompanyId ? (
            <Button type='button' size='sm' onClick={() => setCreateOpen(true)}>
              {t('Create project')}
            </Button>
          ) : null}
        </CardTitle>
        </CardHeader>
        <CardContent>
          <Empty className='min-h-48 border-0'>
            <EmptyHeader>
              <EmptyTitle>{t('No enterprise customers available')}</EmptyTitle>
              <EmptyDescription>
                {t('Create an enterprise customer before adding projects.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        </CardContent>
      </Card>
    )
  }

  const selectedCompany = companyItems.find(
    (company) => company.id === selectedCompanyId
  )
  const projects = projectsQuery.data ?? []

  return (
    <>
      <Card>
      <CardHeader className='gap-3 sm:flex sm:flex-row sm:items-start sm:justify-between'>
        <div>
          <CardTitle>{t('Project overview')}</CardTitle>
          <CardDescription>
            {t('Review project budgets and safeguard thresholds by customer.')}
          </CardDescription>
        </div>
        <Select
          items={companyItems.map((company) => ({
            value: String(company.id),
            label: company.name,
          }))}
          value={selectedCompanyId === null ? null : String(selectedCompanyId)}
          onValueChange={(value) => {
            if (value) setSelectedCompanyId(Number(value))
          }}
        >
          <SelectTrigger className='w-full sm:w-64'>
            <SelectValue placeholder={t('Select an enterprise customer')} />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {companyItems.map((company) => (
                <SelectItem key={company.id} value={String(company.id)}>
                  {company.name}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </CardHeader>
      <CardContent>
        {projectsQuery.isLoading && (
          <div className='grid gap-3 md:grid-cols-2'>
            <Skeleton className='h-36 w-full' />
            <Skeleton className='h-36 w-full' />
          </div>
        )}
        {!projectsQuery.isLoading && projectsQuery.isError && (
          <ErrorState
            className='min-h-48 border-0'
            title={t('Unable to load business data')}
            description={t('Try refreshing this panel.')}
            onRetry={() => void projectsQuery.refetch()}
          />
        )}
        {!projectsQuery.isLoading && !projectsQuery.isError && projects.length === 0 && (
          <Empty className='min-h-48 border-0'>
            <EmptyHeader>
              <EmptyTitle>{t('No projects found')}</EmptyTitle>
              <EmptyDescription>
                {t('No projects are configured for {{customer}}.', {
                  customer: selectedCompany?.name ?? '',
                })}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
        {!projectsQuery.isLoading && !projectsQuery.isError && projects.length > 0 && (
          <div className='grid gap-3 md:grid-cols-2'>
            {projects.map((project) => {
              const controlsConfigured = Boolean(
                project.model_limits || project.channel_limits
              )

              return (
                <div key={project.id} className='rounded-lg border p-3'>
                  <div className='flex items-start justify-between gap-3'>
                    <div className='min-w-0'>
                      <p className='truncate font-medium'>{project.name}</p>
                      <p className='text-muted-foreground text-xs'>
                        {t('Project ID')}: {project.id}
                      </p>
                    </div>
                    <div className='flex items-center gap-1'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => setEditingProject(project)}
                      >
                        {t('Edit')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => setBindProject(project)}
                      >
                        {t('Bind key')}
                      </Button>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() => setReportProject(project)}
                      >
                        {t('Report')}
                      </Button>
                    </div>
                    <StatusBadge
                      label={project.status === 1 ? t('Active') : t('Disabled')}
                      variant={project.status === 1 ? 'success' : 'neutral'}
                      copyable={false}
                    />
                  </div>
                  <dl className='mt-4 grid grid-cols-2 gap-3 text-sm'>
                    <div>
                      <dt className='text-muted-foreground'>
                        {t('Project budget')}
                      </dt>
                      <dd className='mt-1 font-medium'>
                        {formatQuota(project.budget_quota)}
                      </dd>
                    </div>
                    <div>
                      <dt className='text-muted-foreground'>
                        {t('Low balance threshold')}
                      </dt>
                      <dd className='mt-1 font-medium'>
                        {formatQuota(project.low_balance_quota)}
                      </dd>
                    </div>
                    <div className='col-span-2'>
                      <dt className='text-muted-foreground'>
                        {t('Model and channel controls')}
                      </dt>
                      <dd className='mt-1 font-medium'>
                        {controlsConfigured ? t('Configured') : t('Not configured')}
                      </dd>
                    </div>
                  </dl>
                </div>
              )
            })}
          </div>
        )}
      </CardContent>
    </Card>

      {selectedCompanyId ? (
        <CreateProjectDialog
          open={createOpen}
          onOpenChange={setCreateOpen}
          companyId={selectedCompanyId}
          onSaved={invalidateProjects}
        />
      ) : null}
      {editingProject ? (
        <EditProjectDialog
          open={editingProject !== null}
          onOpenChange={(open) => { if (!open) setEditingProject(null) }}
          project={editingProject}
          onSaved={invalidateProjects}
        />
      ) : null}
      {bindProject ? (
        <BindTokenDialog
          open={bindProject !== null}
          onOpenChange={(open) => { if (!open) setBindProject(null) }}
          projectId={bindProject.id}
          onSaved={invalidateProjects}
        />
      ) : null}
      {reportProject ? (
        <ProjectReportDialog
          open={reportProject !== null}
          onOpenChange={(open) => { if (!open) setReportProject(null) }}
          projectId={reportProject.id}
        />
      ) : null}
    </>
  )
}

function ProjectOverviewSkeleton() {
  return (
    <Card>
      <CardHeader>
        <Skeleton className='h-5 w-36' />
        <Skeleton className='h-4 w-72' />
      </CardHeader>
      <CardContent className='grid gap-3 md:grid-cols-2'>
        <Skeleton className='h-36 w-full' />
        <Skeleton className='h-36 w-full' />
      </CardContent>
    </Card>
  )
}
