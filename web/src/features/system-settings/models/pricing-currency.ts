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
import { useSystemConfig } from '@/hooks/use-system-config'

import { formatPricingNumber } from './pricing-format'

export type PricingCurrencyInfo = {
  enabled: boolean
  rate: number
  symbol: string
  perMillionSuffix: string
  toDisplay: (usdValue: string) => string
  toUsd: (displayValue: string) => string
}

const toNumberOrNull = (value: string): number | null => {
  if (value === '') return null
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

const SCALE_ROUND_DECIMALS = 10

const scaleString = (value: string, factor: number): string => {
  if (value === '') return ''
  const num = toNumberOrNull(value)
  if (num === null || factor <= 0) return value
  const scaled = num * factor
  return formatPricingNumber(Number(scaled.toFixed(SCALE_ROUND_DECIMALS)))
}

export function usdToDisplayValue(usdValue: string, rate: number): string {
  return scaleString(usdValue, rate)
}

export function displayToUsdValue(displayValue: string, rate: number): string {
  return scaleString(displayValue, 1 / rate)
}

export function usePricingCurrency(): PricingCurrencyInfo {
  const { currency } = useSystemConfig()
  const enabled = currency.quotaDisplayType === 'CNY'
  const rate = enabled ? currency.usdExchangeRate || 1 : 1

  return {
    enabled,
    rate,
    symbol: enabled ? '¥' : '$',
    perMillionSuffix: enabled ? '¥/1M' : '$/1M',
    toDisplay: (usdValue) => usdToDisplayValue(usdValue, rate),
    toUsd: (displayValue) => displayToUsdValue(displayValue, rate),
  }
}
