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
import { Link } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { useApiCatalog } from '@/content/api-reference/catalog'
import { cn } from '@/lib/utils'

const METHOD_DOT: Record<string, string> = {
  get: 'bg-emerald-500',
  post: 'bg-sky-500',
  put: 'bg-amber-500',
  delete: 'bg-rose-500',
  patch: 'bg-violet-500',
  head: 'bg-slate-400',
  options: 'bg-slate-400',
}

export function ApiReferenceOverview() {
  const { t } = useTranslation()
  const catalog = useApiCatalog()

  if (catalog.isLoading) {
    return (
      <div className='flex items-center gap-2 py-16 text-muted-foreground'>
        <Loader2 className='h-4 w-4 animate-spin' />
        {t('Loading...')}
      </div>
    )
  }
  if (catalog.isError || !catalog.data) {
    return (
      <div className='py-16 text-muted-foreground'>
        {t('Unable to load API reference')}
      </div>
    )
  }

  return (
    <div className='flex flex-col gap-10'>
      <header className='flex flex-col gap-3'>
        <h1 className='text-3xl font-bold tracking-tight'>{t('API Reference')}</h1>
        <p className='text-muted-foreground'>
          {t('Platform API documentation')} · {catalog.data.endpointCount} {t('endpoints total')}
        </p>
      </header>

      {catalog.data.groups.map((group) => (
        <section key={group.id} className='flex flex-col gap-5'>
          <div>
            <h2 className='text-xl font-semibold tracking-tight'>{group.title}</h2>
            {group.description ? (
              <p className='text-muted-foreground'>{group.description}</p>
            ) : null}
          </div>
          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            {group.categories.map((category) => (
              <div
                key={category.id}
                className='flex flex-col gap-2 rounded-xl border p-4'
              >
                <div className='text-sm font-semibold'>{category.title}</div>
                <div className='flex flex-col gap-1'>
                  {category.endpoints.map((endpoint) => (
                    <Link
                      key={endpoint.id}
                      to='/docs/api-reference/$endpointId'
                      params={{ endpointId: endpoint.id }}
                      className='group flex items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors hover:bg-accent'
                    >
                      <span
                        className={cn(
                          'h-1.5 w-1.5 shrink-0 rounded-full',
                          METHOD_DOT[endpoint.method]
                        )}
                      />
                      <span className='text-muted-foreground transition-colors group-hover:text-foreground'>
                        {endpoint.summary}
                      </span>
                    </Link>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
