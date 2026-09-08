import { createFileRoute } from '@tanstack/react-router'
import Subscription from '@/content/docs/zh/guide/subscription.mdx'

export const Route = createFileRoute('/docs/guide/subscription')({
  component: () => <Subscription />,
})