import { createFileRoute } from '@tanstack/react-router'
import Installation from '@/content/docs/zh/installation.mdx'

export const Route = createFileRoute('/_authenticated/docs/installation')({
  component: () => <Installation />,
})