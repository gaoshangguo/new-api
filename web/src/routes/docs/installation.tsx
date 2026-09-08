import { createFileRoute } from '@tanstack/react-router'
import Installation from '@/content/docs/zh/installation.mdx'

export const Route = createFileRoute('/docs/installation')({
  component: () => <Installation />,
})