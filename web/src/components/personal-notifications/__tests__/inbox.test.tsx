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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { NotificationPopover } from '@/components/notification-popover'
import { PersonalNotificationsContent } from '@/components/personal-notifications-content'
import type { PersonalInbox } from '@/hooks/use-personal-notifications'

const originalGetAnimations = Object.getOwnPropertyDescriptor(
  HTMLElement.prototype,
  'getAnimations'
)
beforeEach(() => {
  Object.defineProperty(HTMLElement.prototype, 'getAnimations', {
    configurable: true,
    value: () => [],
  })
})
afterEach(() => {
  if (originalGetAnimations) {
    Object.defineProperty(
      HTMLElement.prototype,
      'getAnimations',
      originalGetAnimations
    )
  } else {
    Reflect.deleteProperty(HTMLElement.prototype, 'getAnimations')
  }
})

function inbox(overrides: Partial<PersonalInbox> = {}): PersonalInbox {
  return {
    items: [],
    unreadCount: 0,
    loading: false,
    error: false,
    refetch: vi.fn(),
    markRead: vi.fn(),
    markingRead: false,
    hasMore: false,
    loadingMore: false,
    loadMore: vi.fn(),
    ...overrides,
  }
}

describe('personal inbox interactions', () => {
  it('shows an empty inbox without read controls', () => {
    render(<PersonalNotificationsContent inbox={inbox()} />)
    expect(screen.getByText('No personal messages')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Mark as read' })
    ).not.toBeInTheDocument()
  })

  it('shows a loading state before messages arrive', () => {
    render(<PersonalNotificationsContent inbox={inbox({ loading: true })} />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading...')
  })

  it('retries a failed inbox request from its error state', async () => {
    const personal = inbox({ error: true })
    render(<PersonalNotificationsContent inbox={personal} />)
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Failed to load personal messages'
    )
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(personal.refetch).toHaveBeenCalled()
  })

  it('marks only an unread message and disables duplicate submissions while saving', async () => {
    const personal = inbox({
      items: [
        {
          id: 1,
          type: 'low_balance',
          quota: 19,
          threshold: 20,
          created_at: 100,
          read_at: 0,
        },
        {
          id: 2,
          type: 'low_balance',
          quota: 18,
          threshold: 20,
          created_at: 100,
          read_at: 200,
        },
      ],
    })
    const { rerender } = render(
      <PersonalNotificationsContent inbox={personal} />
    )
    const read = screen.getByRole('button', { name: 'Mark as read' })
    await userEvent.click(read)
    expect(personal.markRead).toHaveBeenCalledWith(1)
    rerender(
      <PersonalNotificationsContent
        inbox={{ ...personal, markingRead: true }}
      />
    )
    expect(screen.getByRole('button', { name: 'Mark as read' })).toBeDisabled()
  })

  it('allows older messages to be loaded without automatically marking them read', async () => {
    const personal = inbox({
      items: [
        {
          id: 1,
          type: 'low_balance',
          quota: 19,
          threshold: 20,
          created_at: 100,
          read_at: 0,
        },
      ],
      hasMore: true,
    })
    render(<PersonalNotificationsContent inbox={personal} />)
    await userEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(personal.loadMore).toHaveBeenCalled()
    expect(personal.markRead).not.toHaveBeenCalled()
  })

  it('shows personal messages in a keyboard-accessible tab only for signed-in users', async () => {
    const props = {
      open: true,
      onOpenChange: vi.fn(),
      unreadCount: 1,
      activeTab: 'messages' as const,
      onTabChange: vi.fn(),
      notice: '',
      announcements: [],
      loading: false,
    }
    const { rerender } = render(
      <NotificationPopover {...props} personal={inbox({ unreadCount: 1 })} />
    )
    const messages = screen.getByRole('tab', { name: /Messages/ })
    expect(messages).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText('No personal messages')).toBeInTheDocument()
    await userEvent.click(messages)
    await userEvent.keyboard('{ArrowRight}{Enter}')
    expect(props.onTabChange).toHaveBeenCalledWith('notice')
    rerender(
      <NotificationPopover {...props} activeTab='notice' personal={undefined} />
    )
    expect(
      screen.queryByRole('tab', { name: /Messages/ })
    ).not.toBeInTheDocument()
  })
})
