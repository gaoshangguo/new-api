import { createFileRoute } from '@tanstack/react-router'
import Tokens from '@/content/docs/zh/guide/tokens.mdx'

export const Route = createFileRoute('/docs/guide/tokens')({
  component: () => <Tokens />,
})