import { createFileRoute, Outlet } from '@tanstack/react-router'
import { DocsSidebar } from '@/components/docs/docs-sidebar'

export const Route = createFileRoute('/_authenticated/docs')({
  component: DocsLayout,
})

function DocsLayout() {
  return (
    <div className="flex h-full">
      <aside className="w-56 shrink-0 border-r overflow-auto p-3">
        <DocsSidebar />
      </aside>
      <main className="flex-1 overflow-auto">
        <article className="prose prose-slate dark:prose-invert max-w-4xl px-8 py-6">
          <Outlet />
        </article>
      </main>
    </div>
  )
}