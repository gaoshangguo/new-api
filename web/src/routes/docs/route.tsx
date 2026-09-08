import { createFileRoute, Outlet, useLocation } from '@tanstack/react-router'
import { Menu } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ApiReferenceSidebar } from '@/components/docs/api-reference-sidebar'
import { DocsSidebar } from '@/components/docs/docs-sidebar'
import { Button } from '@/components/ui/button'
import { PublicLayout } from '@/components/layout'

export const Route = createFileRoute('/docs')({
  component: DocsLayout,
})

function DocsLayout() {
  const { pathname } = useLocation()
  const { t } = useTranslation()
  const [mobileOpen, setMobileOpen] = useState(false)
  const isApiReference = pathname.startsWith('/docs/api-reference')

  return (
    <PublicLayout showMainContainer={false}>
      <div className='bg-background text-foreground relative min-h-svh'>
        {/* Mobile docs nav trigger */}
        <div className='sticky top-16 z-30 border-b bg-background/90 backdrop-blur lg:hidden'>
          <Button
            type='button'
            variant='ghost'
            className='w-full justify-start gap-2 rounded-none px-4'
            onClick={() => setMobileOpen((v) => !v)}
          >
            <Menu className='h-4 w-4' />
            {isApiReference ? t('API Reference') : t('Docs')}
          </Button>
          {mobileOpen && (
            <div className='max-h-[60svh] overflow-y-auto border-t p-3'>
              {isApiReference ? <ApiReferenceSidebar /> : <DocsSidebar />}
            </div>
          )}
        </div>

        <div className='mx-auto flex w-full max-w-[1600px] items-start gap-0 px-4 pt-6 pb-16 md:px-6 lg:px-8 lg:pt-14'>
          <aside className='sticky top-28 hidden max-h-[calc(100svh-8rem)] w-72 shrink-0 overflow-y-auto pr-4 lg:block'>
            {isApiReference ? <ApiReferenceSidebar /> : <DocsSidebar />}
          </aside>
          <main className='min-w-0 flex-1'>
            <article
              className={
                isApiReference
                  ? 'px-2 md:px-8'
                  : 'prose prose-slate dark:prose-invert max-w-none px-2 md:px-8'
              }
            >
              <Outlet />
            </article>
          </main>
        </div>
      </div>
    </PublicLayout>
  )
}