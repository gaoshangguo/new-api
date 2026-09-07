import { createFileRoute } from '@tanstack/react-router'
import PersonalSettings from '@/content/docs/zh/guide/personal-settings.mdx'

export const Route = createFileRoute('/_authenticated/docs/guide/personal-settings')({
  component: () => <PersonalSettings />,
})