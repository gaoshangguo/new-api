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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { NotificationTab } from '@/features/profile/components/tabs/notification-tab'
import type { UserProfile } from '@/features/profile/types'
import { api } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

const profile: UserProfile = {
  id: 1,
  username: 'alice',
  display_name: 'Alice',
  role: 1,
  group: 'default',
  quota: 19,
  used_quota: 0,
  request_count: 0,
  status: 1,
  aff_code: '',
  aff_count: 0,
  aff_quota: 0,
  aff_history_quota: 0,
  created_time: 100,
  setting: JSON.stringify({
    quota_warning_threshold: 500000,
    notify_type: 'webhook',
    webhook_url: 'https://example.com/hook',
    low_balance_in_app_enabled: true,
    low_balance_email_enabled: true,
    notification_email: 'notify@example.com',
  }),
}

afterEach(() => {
  useSystemConfigStore
    .getState()
    .setConfig({ currency: { ...DEFAULT_CURRENCY_CONFIG } })
})

describe('personal low balance settings', () => {
  it('converts a displayed points threshold and saves independently selected channels', async () => {
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CUSTOM',
        customCurrencySymbol: '积分',
        customCurrencyExchangeRate: 10,
      },
    })
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true } })
    const onUpdate = vi.fn()
    render(<NotificationTab profile={profile} onUpdate={onUpdate} />)
    expect(screen.getByLabelText('Quota Warning Threshold')).toHaveValue(10)
    const input = screen.getByLabelText('Quota Warning Threshold')
    await userEvent.clear(input)
    await userEvent.type(input, '20')
    await userEvent.click(
      screen.getByRole('switch', { name: 'Low balance in-app messages' })
    )
    expect(
      screen.getByRole('switch', { name: 'Low balance email notifications' })
    ).toBeChecked()
    expect(screen.getByLabelText('Notification Email')).toHaveValue(
      'notify@example.com'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save Settings' }))
    await waitFor(() => expect(onUpdate).toHaveBeenCalled())
    expect(put).toHaveBeenCalledWith(
      '/api/user/setting',
      expect.objectContaining({
        quota_warning_threshold: 1000000,
        notify_type: 'webhook',
        low_balance_in_app_enabled: false,
        low_balance_email_enabled: true,
        notification_email: 'notify@example.com',
      })
    )
  })

  it('preserves legacy webhook users email opt-out when independent preferences are absent', () => {
    render(
      <NotificationTab
        profile={{
          ...profile,
          setting: JSON.stringify({
            notify_type: 'webhook',
            quota_warning_threshold: 500000,
          }),
        }}
        onUpdate={vi.fn()}
      />
    )
    expect(
      screen.getByRole('switch', { name: 'Low balance in-app messages' })
    ).toBeChecked()
    expect(
      screen.getByRole('switch', { name: 'Low balance email notifications' })
    ).not.toBeChecked()
    expect(
      screen.queryByLabelText('Notification Email')
    ).not.toBeInTheDocument()
  })
})
