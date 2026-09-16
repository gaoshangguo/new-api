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
import i18next from 'i18next'
import { useCallback, useEffect, useState } from 'react'
import { toast } from 'sonner'

import { getCaptcha } from '@/features/auth/api'
import { useStatus } from '@/hooks/use-status'

/**
 * Hook for managing the one-time image captcha challenge of registration.
 * A challenge is only valid for a single submission, so it must be refreshed
 * after every failed attempt.
 */
export function useCaptcha() {
  const { status } = useStatus()
  const [captchaId, setCaptchaId] = useState('')
  const [captchaImage, setCaptchaImage] = useState('')
  const [captchaCode, setCaptchaCode] = useState('')
  const [isLoadingCaptcha, setIsLoadingCaptcha] = useState(false)

  const isCaptchaEnabled = Boolean(status?.captcha_enabled)

  const refreshCaptcha = useCallback(async () => {
    setIsLoadingCaptcha(true)
    try {
      const res = await getCaptcha()
      const challenge = res?.data
      if (res?.success && challenge?.captcha_id && challenge.image) {
        setCaptchaId(challenge.captcha_id)
        setCaptchaImage(challenge.image)
        setCaptchaCode('')
        return
      }
      setCaptchaId('')
      setCaptchaImage('')
    } catch {
      // Errors are handled by the global interceptor
      setCaptchaId('')
      setCaptchaImage('')
    } finally {
      setIsLoadingCaptcha(false)
    }
  }, [])

  useEffect(() => {
    if (!isCaptchaEnabled) return
    void refreshCaptcha()
  }, [isCaptchaEnabled, refreshCaptcha])

  const validateCaptcha = useCallback((): boolean => {
    if (!isCaptchaEnabled) return true
    if (!captchaId || !captchaImage) {
      toast.info(i18next.t('Verification code is not ready, please refresh'))
      return false
    }
    if (!captchaCode.trim()) {
      toast.info(i18next.t('Please enter the verification code'))
      return false
    }
    return true
  }, [captchaCode, captchaId, captchaImage, isCaptchaEnabled])

  return {
    isCaptchaEnabled,
    captchaId,
    captchaImage,
    captchaCode,
    setCaptchaCode,
    isLoadingCaptcha,
    refreshCaptcha,
    validateCaptcha,
  }
}
