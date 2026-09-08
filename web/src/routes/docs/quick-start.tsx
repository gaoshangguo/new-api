import { createFileRoute } from '@tanstack/react-router'
import QuickStart from '@/content/docs/zh/quick-start.mdx'

export const Route = createFileRoute('/docs/quick-start')({
  component: () => <QuickStart />,
})