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
import { Link, useLocation } from '@tanstack/react-router'
import { ChevronDown, ChevronRight, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { buildCategoryTree, useApiCatalog } from '@/content/api-reference/catalog'
import type { ApiCategoryNode, ApiEndpoint } from '@/content/api-reference/types'
import { cn } from '@/lib/utils'

const METHOD_STYLES: Record<string, string> = {
  get: 'text-emerald-600 dark:text-emerald-400',
  post: 'text-sky-600 dark:text-sky-400',
  put: 'text-amber-600 dark:text-amber-400',
  delete: 'text-rose-600 dark:text-rose-400',
  patch: 'text-violet-600 dark:text-violet-400',
  head: 'text-slate-500 dark:text-slate-400',
  options: 'text-slate-500 dark:text-slate-400',
}

function EndpointNavLink({
  endpoint,
  active,
}: {
  endpoint: ApiEndpoint
  active: string
}) {
  return (
    <Link
      to='/docs/api-reference/$endpointId'
      params={{ endpointId: endpoint.id }}
      className={cn(
        'flex items-center gap-2 rounded-md px-3 py-1 text-[13px] transition-colors',
        active === endpoint.id
          ? 'bg-accent text-accent-foreground font-medium'
          : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
      )}
    >
      <span
        className={cn(
          'w-10 shrink-0 text-xs font-semibold',
          METHOD_STYLES[endpoint.method]
        )}
      >
        {endpoint.method.toUpperCase()}
      </span>
      <span className='truncate'>{endpoint.summary}</span>
    </Link>
  )
}

function CategoryTreeNode({
  node,
  active,
  collapsed,
  onToggle,
}: {
  node: ApiCategoryNode
  active: string
  collapsed: Record<string, boolean>
  onToggle: (key: string) => void
}) {
  const hasChildren = node.children.length > 0
  const hasContent = node.endpoints.length > 0 || hasChildren
  const isCollapsed = collapsed[node.path] ?? false

  if (!hasContent) return null

  return (
    <div className='flex flex-col'>
      {hasChildren ? (
        <button
          type='button'
          onClick={() => onToggle(node.path)}
          className='flex w-full items-center justify-between gap-1 rounded-md px-3 py-1 text-left text-[13px] font-medium text-foreground/90 transition-colors hover:bg-accent hover:text-accent-foreground'
        >
          <span className='truncate'>{node.title}</span>
          {isCollapsed ? (
            <ChevronRight className='h-3.5 w-3.5 shrink-0 text-muted-foreground' />
          ) : (
            <ChevronDown className='h-3.5 w-3.5 shrink-0 text-muted-foreground' />
          )}
        </button>
      ) : (
        <span className='px-3 py-1 text-xs text-muted-foreground/70'>
          {node.title}
        </span>
      )}
      {!isCollapsed && (
        <div className='ml-2 flex flex-col border-l pl-2'>
          {node.endpoints.map((endpoint) => (
            <EndpointNavLink key={endpoint.id} endpoint={endpoint} active={active} />
          ))}
          {node.children.map((child) => (
            <CategoryTreeNode
              key={child.path}
              node={child}
              active={active}
              collapsed={collapsed}
              onToggle={onToggle}
            />
          ))}
        </div>
      )}
    </div>
  )
}

/** Left rail shown on /docs/api-reference* pages. */
export function ApiReferenceSidebar() {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const catalog = useApiCatalog()
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})

  const toggle = (key: string) =>
    setCollapsed((prev) => ({ ...prev, [key]: !prev[key] }))

  if (catalog.isLoading) {
    return (
      <div className='flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground'>
        <Loader2 className='h-4 w-4 animate-spin' />
        {t('Loading...')}
      </div>
    )
  }

  if (catalog.isError || !catalog.data) {
    return (
      <div className='px-3 py-2 text-sm text-muted-foreground'>
        {t('Unable to load API reference')}
      </div>
    )
  }

  const active = pathname.split('/').pop() ?? ''

  return (
    <nav className='docs-api-nav flex flex-col gap-1'>
      <Link
        to='/docs/api-reference'
        className={cn(
          'flex items-center rounded-md px-3 py-1.5 text-sm transition-colors',
          pathname === '/docs/api-reference' || pathname === '/docs/api-reference/'
            ? 'bg-accent text-accent-foreground font-medium'
            : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
        )}
      >
        {t('API Reference')}
      </Link>
      {catalog.data.groups.map((group) => {
        const isCollapsed = collapsed[group.id] ?? false
        return (
          <div key={group.id} className='mt-1'>
            <button
              type='button'
              onClick={() => toggle(group.id)}
              className='flex w-full items-center justify-between rounded-md px-3 py-1.5 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase hover:text-foreground'
            >
              <span>{group.title}</span>
              {isCollapsed ? (
                <ChevronRight className='h-3.5 w-3.5' />
              ) : (
                <ChevronDown className='h-3.5 w-3.5' />
              )}
            </button>
            {!isCollapsed && (
              <div className='mt-0.5 ml-2 flex flex-col border-l pl-2'>
                {buildCategoryTree(group.categories).map((node) => (
                  <CategoryTreeNode
                    key={node.path}
                    node={node}
                    active={active}
                    collapsed={collapsed}
                    onToggle={toggle}
                  />
                ))}
              </div>
            )}
          </div>
        )
      })}
    </nav>
  )
}
