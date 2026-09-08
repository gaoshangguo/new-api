import { createFileRoute } from '@tanstack/react-router'
import Topup from '@/content/docs/zh/guide/topup.mdx'

export const Route = createFileRoute('/docs/guide/topup')({
  component: () => <Topup />,
})