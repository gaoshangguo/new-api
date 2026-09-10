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
import { X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useAnnouncements } from '@/features/dashboard/hooks/use-status-data'

const DISMISS_STORAGE_KEY = 'home-announcement-dismissed'

/**
 * Top-of-page announcement band sourced from the site announcements managed in
 * System Settings → Content. Dismissals are remembered per announcement so a
 * new announcement always reappears.
 */
export function AnnouncementBanner() {
  const { t } = useTranslation()
  const { items } = useAnnouncements()
  const [dismissedKey, setDismissedKey] = useState<string | null>(null)

  useEffect(() => {
    try {
      setDismissedKey(window.localStorage.getItem(DISMISS_STORAGE_KEY))
    } catch {
      setDismissedKey(null)
    }
  }, [])

  const announcement = items[0]
  const announcementKey = announcement
    ? String(announcement.id ?? announcement.content)
    : ''

  if (!announcement || !announcement.content || dismissedKey === announcementKey) {
    return null
  }

  const handleDismiss = () => {
    setDismissedKey(announcementKey)
    try {
      window.localStorage.setItem(DISMISS_STORAGE_KEY, announcementKey)
    } catch {
      // Ignore storage failures (private mode); the banner just reappears.
    }
  }

  return (
    <div className='bg-primary/5 border-primary/15 relative z-20 border-b'>
      <div className='mx-auto flex max-w-6xl items-center justify-center gap-3 px-6 py-2.5'>
        <p className='text-foreground/80 line-clamp-2 text-center text-sm'>
          {announcement.content}
        </p>
        <button
          type='button'
          onClick={handleDismiss}
          aria-label={t('Close')}
          className='text-muted-foreground hover:text-foreground shrink-0 rounded-md p-1 transition-colors'
        >
          <X className='size-3.5' />
        </button>
      </div>
    </div>
  )
}
