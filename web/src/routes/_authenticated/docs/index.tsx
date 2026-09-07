import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/docs/')({
  beforeLoad: () => {
    throw redirect({ to: '/docs/quick-start' })
  },
})