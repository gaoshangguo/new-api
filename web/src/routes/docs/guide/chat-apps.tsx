import { createFileRoute } from '@tanstack/react-router'
import ChatApps from '@/content/docs/zh/guide/chat-apps.mdx'

export const Route = createFileRoute('/docs/guide/chat-apps')({
  component: () => <ChatApps />,
})