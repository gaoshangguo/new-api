import { createFileRoute } from '@tanstack/react-router'
import UsageLogs from '@/content/docs/zh/guide/usage-logs.mdx'

export const Route = createFileRoute('/_authenticated/docs/guide/usage-logs')({
  component: () => <UsageLogs />,
})