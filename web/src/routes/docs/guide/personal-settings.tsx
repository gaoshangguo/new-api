import { createFileRoute } from '@tanstack/react-router'
import PersonalSettings from '@/content/docs/zh/guide/personal-settings.mdx'

export const Route = createFileRoute('/docs/guide/personal-settings')({
  component: () => <PersonalSettings />,
})