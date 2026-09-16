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
import { Loader2, RefreshCw } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { CAPTCHA_CODE_LENGTH } from '@/features/auth/constants'
import { cn } from '@/lib/utils'

interface CaptchaInputProps {
  image: string
  code: string
  onCodeChange: (code: string) => void
  onRefresh: () => void
  isLoading?: boolean
  className?: string
}

export function CaptchaInput(props: CaptchaInputProps) {
  const { t } = useTranslation()
  const refreshLabel = t('Click to refresh')

  let preview: ReactNode
  if (props.image) {
    preview = (
      <img
        src={props.image}
        alt={t('Verification code image')}
        className='h-full w-auto'
      />
    )
  } else if (props.isLoading) {
    preview = <Loader2 className='text-muted-foreground h-4 w-4 animate-spin' />
  } else {
    preview = <RefreshCw className='text-muted-foreground h-4 w-4' />
  }

  return (
    <div className={cn('grid gap-2', props.className)}>
      <Label htmlFor='captcha-code'>{t('Graphical verification code')}</Label>
      <div className='flex items-center gap-2'>
        <Input
          id='captcha-code'
          className='h-10 flex-1 font-mono tracking-widest'
          placeholder={t('Verification code')}
          value={props.code}
          autoComplete='off'
          autoCapitalize='characters'
          spellCheck={false}
          maxLength={CAPTCHA_CODE_LENGTH}
          onChange={(event) =>
            props.onCodeChange(event.target.value.toUpperCase())
          }
        />
        <button
          type='button'
          onClick={props.onRefresh}
          disabled={props.isLoading}
          aria-label={refreshLabel}
          title={refreshLabel}
          className='border-input bg-muted/40 hover:bg-muted flex h-10 items-center justify-center overflow-hidden rounded-lg border px-1 transition-colors disabled:opacity-60'
        >
          {preview}
        </button>
      </div>
    </div>
  )
}
