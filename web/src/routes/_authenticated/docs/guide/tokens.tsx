import { createFileRoute } from '@tanstack/react-router'
import Tokens from '@/content/docs/zh/guide/tokens.mdx'

export const Route = createFileRoute('/_authenticated/docs/guide/tokens')({
  component: () => <Tokens />,
})