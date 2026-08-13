import { createFileRoute } from '@tanstack/react-router'

import { CustomerFinancePage } from '@/features/customer-finance'

export const Route = createFileRoute('/_authenticated/wallet/financial-details')({
  component: CustomerFinancePage,
})
