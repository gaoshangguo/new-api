import { createFileRoute } from '@tanstack/react-router'
import Tasks from '@/content/docs/zh/guide/tasks.mdx'

export const Route = createFileRoute('/docs/guide/tasks')({
  component: () => <Tasks />,
})