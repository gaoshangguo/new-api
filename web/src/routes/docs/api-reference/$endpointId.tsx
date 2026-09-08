/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTIBILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { createFileRoute, Link } from '@tanstack/react-router'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EndpointDocumentation } from '@/components/docs/endpoint-documentation'
import { Button } from '@/components/ui/button'
import { findEndpoint, useApiCatalog } from '@/content/api-reference/catalog'

export const Route = createFileRoute('/docs/api-reference/$endpointId')({
  component: EndpointPage,
  notFoundComponent: EndpointNotFound,
})

function EndpointPage() {
  const { endpointId } = Route.useParams()
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

  const endpoint = findEndpoint(catalog.data, endpointId)
  if (!endpoint) {
    return <EndpointNotFound />
  }

  return (
    <div className='flex flex-col gap-6'>
      <div className='flex items-center gap-1 text-sm text-muted-foreground'>
        <Link to='/docs' className='hover:text-foreground'>
          {t('Docs')}
        </Link>
        <span>/</span>
        <Link to='/docs/api-reference' className='hover:text-foreground'>
          API 参考
        </Link>
      </div>
      <EndpointDocumentation endpoint={endpoint} />
    </div>
  )
}

function EndpointNotFound() {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col items-start gap-4 py-16'>
      <div className='text-lg font-semibold'>{t('Endpoint not found')}</div>
      <Button render={<Link to='/docs/api-reference' />}>
        {t('Back to API reference')}
      </Button>
    </div>
  )
}
