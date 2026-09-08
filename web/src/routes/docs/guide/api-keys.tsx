import { createFileRoute } from '@tanstack/react-router'
import ApiKeys from '@/content/docs/zh/guide/api-keys.mdx'

export const Route = createFileRoute('/docs/guide/api-keys')({
  component: () => <ApiKeys />,
})