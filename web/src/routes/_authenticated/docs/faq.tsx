import { createFileRoute } from '@tanstack/react-router'
import Faq from '@/content/docs/zh/faq.mdx'

export const Route = createFileRoute('/_authenticated/docs/faq')({
  component: () => <Faq />,
})