/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { createFileRoute, redirect } from '@tanstack/react-router'

import { BusinessWorkbench } from '@/features/business'
import {
  BUSINESS_PERMISSION_ACTIONS,
  BUSINESS_WORKBENCH_READ_RESOURCES,
} from '@/features/business/types'
import { hasPermission } from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/business/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    const canAccessWorkbench = BUSINESS_WORKBENCH_READ_RESOURCES.some(
      (resource) =>
        hasPermission(auth.user, resource, BUSINESS_PERMISSION_ACTIONS.READ)
    )
    if (!canAccessWorkbench) throw redirect({ to: '/403' })
  },
  component: BusinessWorkbench,
})
