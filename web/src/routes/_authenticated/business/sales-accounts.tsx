import { createFileRoute, redirect } from '@tanstack/react-router'

import { SalesAccountsPage } from '@/features/sales-accounts'
import { USER_ROLE } from '@/features/users/constants'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/business/sales-accounts')({
  beforeLoad: () => {
    if (useAuthStore.getState().auth.user?.role !== USER_ROLE.ROOT) {
      throw redirect({ to: '/403' })
    }
  },
  component: SalesAccountsPage,
})
