import { createFileRoute } from '@tanstack/react-router'
import Topup from '@/content/docs/zh/guide/topup.mdx'

export const Route = createFileRoute('/_authenticated/docs/guide/topup')({
  component: () => <Topup />,
})