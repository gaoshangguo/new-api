import { createFileRoute } from '@tanstack/react-router'
import Faq from '@/content/docs/zh/faq.mdx'

export const Route = createFileRoute('/docs/faq')({
  component: () => <Faq />,
})