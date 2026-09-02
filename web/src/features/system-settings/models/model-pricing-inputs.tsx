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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { cn } from '@/lib/utils'

import {
  SettingsControlGroup,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { numericDraftRegex } from './model-pricing-core'
import { usePricingCurrency, type PricingCurrencyInfo } from './pricing-currency'

export function CurrencyPriceInput(props: {
  value: string
  placeholder?: string
  disabled?: boolean
  currency: PricingCurrencyInfo
  onUsdChange: (usd: string) => void
  onBlur?: () => void
}) {
  const { value, placeholder, disabled, currency, onUsdChange, onBlur } = props
  const [draft, setDraft] = useState(() => currency.toDisplay(value))
  const [focused, setFocused] = useState(false)

  useEffect(() => {
    if (!focused) {
      setDraft(currency.toDisplay(value))
    }
  }, [currency, focused, value])

  const handleChange = (raw: string) => {
    if (!numericDraftRegex.test(raw)) return
    setDraft(raw)
    onUsdChange(currency.toUsd(raw))
  }

  const handleBlur = () => {
    setFocused(false)
    setDraft(currency.toDisplay(currency.toUsd(draft)))
    onBlur?.()
  }

  return (
    <InputGroupInput
      inputMode='decimal'
      value={draft}
      placeholder={placeholder}
      disabled={disabled}
      onChange={(event) => handleChange(event.target.value)}
      onFocus={() => setFocused(true)}
      onBlur={handleBlur}
    />
  )
}

export function PriceInput(props: {
  value: string
  placeholder?: string
  disabled?: boolean
  onChange: (value: string) => void
}) {
  const currency = usePricingCurrency()
  return (
    <InputGroup>
      <InputGroupAddon>{currency.symbol}</InputGroupAddon>
      <CurrencyPriceInput
        value={props.value}
        placeholder={props.placeholder}
        disabled={props.disabled}
        currency={currency}
        onUsdChange={props.onChange}
      />
      <InputGroupAddon align='inline-end'>
        {currency.perMillionSuffix}
      </InputGroupAddon>
    </InputGroup>
  )
}

export function PriceLane(props: {
  title: string
  description: string
  placeholder: string
  value: string
  enabled: boolean
  disabled?: boolean
  onEnabledChange: (checked: boolean) => void
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const effectiveDisabled = props.disabled || !props.enabled

  return (
    <SettingsControlGroup
      className={cn('space-y-3', effectiveDisabled && 'opacity-75')}
      data-disabled={effectiveDisabled || undefined}
    >
      <SettingsSwitchField
        checked={props.enabled}
        disabled={props.disabled}
        onCheckedChange={props.onEnabledChange}
        label={props.title}
        description={props.description}
        aria-label={props.title}
      />
      <PriceInput
        value={props.value}
        placeholder={props.placeholder}
        disabled={effectiveDisabled}
        onChange={props.onChange}
      />
      <p className='text-muted-foreground text-xs'>
        {props.enabled
          ? t('Price per 1M tokens.')
          : t('Disabled lanes are omitted on save.')}
      </p>
    </SettingsControlGroup>
  )
}
