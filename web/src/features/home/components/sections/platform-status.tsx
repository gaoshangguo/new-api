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
*/
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { api } from '@/lib/api'

// P0-01 首页状态与商务联系：公开状态接口（无鉴权），展示公告、平台状态与商务联系方式。

type StatusCounts = Record<number, number>

type PublicStatusResponse = {
  success: boolean
  data: {
    channel_status_counts: StatusCounts
    announcements: Array<{
      id: number
      title: string
      content: string
      created_at: number
    }>
    contact: {
      email: string
      phone: string
      wechat: string
    }
  }
}

async function fetchPublicStatus() {
  const res = await api.get<PublicStatusResponse>('/api/business/status')
  return res.data.data
}

type AnnouncementProps = {
  title: string
  content: string
  time: string
}

function Announcement(props: AnnouncementProps) {
  return (
    <article className='rounded-xl border bg-background p-4'>
      <div className='flex items-center justify-between gap-2'>
        <h4 className='text-sm font-semibold'>{props.title}</h4>
        <span className='text-muted-foreground shrink-0 text-xs'>
          {props.time}
        </span>
      </div>
      <p className='text-muted-foreground mt-2 whitespace-pre-wrap text-sm leading-6'>
        {props.content}
      </p>
    </article>
  )
}

export function PlatformStatus() {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['home', 'platform-status'],
    queryFn: fetchPublicStatus,
    staleTime: 60 * 1000,
    retry: 1,
  })

  const announcements = data?.announcements ?? []
  const contact = data?.contact
  const hasContact = Boolean(
    contact?.email || contact?.phone || contact?.wechat
  )
  const hasContent = announcements.length > 0 || hasContact

  if (isLoading || !hasContent) {
    return null
  }

  const fmtTime = (ts: number) => {
    const d = new Date(ts * 1000)
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(
      d.getDate()
    ).padStart(2, '0')}`
  }

  return (
    <section className='px-6 py-20 md:py-24'>
      <div className='mx-auto grid max-w-6xl gap-8 lg:grid-cols-3'>
        <div className='lg:col-span-2'>
          <div className='mb-6'>
            <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
              {t('Platform status')}
            </h2>
            <p className='text-muted-foreground mt-2 text-sm'>
              {t('Maintenance, fault and model change notices.')}
            </p>
          </div>
          {announcements.length > 0 ? (
            <div className='flex flex-col gap-3'>
              {announcements.map((item) => (
                <Announcement
                  key={item.id}
                  title={item.title}
                  content={item.content}
                  time={fmtTime(item.created_at)}
                />
              ))}
            </div>
          ) : (
            <p className='text-muted-foreground text-sm'>
              {t('No announcements right now.')}
            </p>
          )}
        </div>

        <aside>
          <div className='rounded-2xl border bg-muted/30 p-6'>
            <h3 className='text-lg font-semibold'>{t('Business contact')}</h3>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Enterprise onboarding, invoicing and technical support.')}
            </p>
            <dl className='mt-5 flex flex-col gap-3 text-sm'>
              {contact?.email ? (
                <div className='flex items-center gap-2'>
                  <dt className='text-muted-foreground w-14 shrink-0'>
                    {t('Email')}
                  </dt>
                  <dd className='break-all'>{contact.email}</dd>
                </div>
              ) : null}
              {contact?.phone ? (
                <div className='flex items-center gap-2'>
                  <dt className='text-muted-foreground w-14 shrink-0'>
                    {t('Phone')}
                  </dt>
                  <dd>{contact.phone}</dd>
                </div>
              ) : null}
              {contact?.wechat ? (
                <div className='flex items-center gap-2'>
                  <dt className='text-muted-foreground w-14 shrink-0'>
                    {t('WeChat')}
                  </dt>
                  <dd>{contact.wechat}</dd>
                </div>
              ) : null}
              {!hasContact ? (
                <p className='text-muted-foreground text-sm'>
                  {t('Contact sales for enterprise pricing.')}
                </p>
              ) : null}
            </dl>
          </div>
        </aside>
      </div>
    </section>
  )
}
