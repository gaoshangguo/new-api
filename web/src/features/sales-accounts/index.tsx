import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout/components/section-page-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'

type SalesAccountProfile = {
  user_id: number
  department: string
  region: string
  note: string
}

export function SalesAccountsPage() {
  const { t } = useTranslation()
  const [userId, setUserId] = useState('')
  const [profile, setProfile] = useState<SalesAccountProfile>({
    user_id: 0,
    department: '',
    region: '',
    note: '',
  })
  const numericUserId = Number(userId)

  const loadProfile = async () => {
    if (!Number.isSafeInteger(numericUserId) || numericUserId <= 0) return
    try {
      const response = await api.get(`/api/business/sales/accounts/${numericUserId}`)
      if (!response.data.success) throw new Error(response.data.message)
      setProfile(response.data.data)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Load failed'))
    }
  }

  const saveProfile = async () => {
    if (!Number.isSafeInteger(numericUserId) || numericUserId <= 0) return
    try {
      const response = await api.put(`/api/business/sales/accounts/${numericUserId}`, profile)
      if (!response.data.success) throw new Error(response.data.message)
      setProfile(response.data.data)
      toast.success(t('Saved successfully'))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Save failed'))
    }
  }

  const setAccountStatus = async (status: number) => {
    if (!Number.isSafeInteger(numericUserId) || numericUserId <= 0) return
    try {
      const response = await api.put(
        `/api/business/sales/accounts/${numericUserId}/status`,
        { status }
      )
      if (!response.data.success) throw new Error(response.data.message)
      toast.success(t('Saved successfully'))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Save failed'))
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Sales account profile')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='max-w-xl space-y-4 rounded-xl border bg-card p-4'>
          <div className='flex gap-2'>
            <Input inputMode='numeric' placeholder={t('User ID')} value={userId} onChange={(event) => setUserId(event.target.value)} />
            <Button onClick={() => void loadProfile()}>{t('Load')}</Button>
          </div>
          <Input placeholder={t('Department')} value={profile.department} onChange={(event) => setProfile((value) => ({ ...value, department: event.target.value }))} />
          <Input placeholder={t('Region')} value={profile.region} onChange={(event) => setProfile((value) => ({ ...value, region: event.target.value }))} />
          <Textarea placeholder={t('Note')} value={profile.note} onChange={(event) => setProfile((value) => ({ ...value, note: event.target.value }))} />
          <div className='flex flex-wrap gap-2'>
            <Button onClick={() => void saveProfile()}>{t('Save')}</Button>
            <Button variant='outline' onClick={() => void setAccountStatus(1)}>{t('Enable')}</Button>
            <Button variant='destructive' onClick={() => void setAccountStatus(2)}>{t('Disable')}</Button>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
