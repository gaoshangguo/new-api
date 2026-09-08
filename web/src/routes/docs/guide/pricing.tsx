import { createFileRoute } from '@tanstack/react-router'
import Pricing from '@/content/docs/zh/guide/pricing.mdx'

export const Route = createFileRoute('/docs/guide/pricing')({
  component: () => <Pricing />,
})