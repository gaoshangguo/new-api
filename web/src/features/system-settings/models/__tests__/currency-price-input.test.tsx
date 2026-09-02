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
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { CurrencyPriceInput } from '../model-pricing-inputs'
import { usePricingCurrency } from '../pricing-currency'

function setDisplayCurrency(type: 'USD' | 'CNY', rate: number) {
  useSystemConfigStore.setState({
    config: {
      ...useSystemConfigStore.getState().config,
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: type,
        usdExchangeRate: rate,
      },
    },
  })
}

function Harness({
  value,
  onUsdChange,
}: {
  value: string
  onUsdChange: (usd: string) => void
}) {
  const currency = usePricingCurrency()
  return (
    <CurrencyPriceInput
      value={value}
      placeholder='0.01'
      currency={currency}
      onUsdChange={onUsdChange}
    />
  )
}

describe('currency-aware price input', () => {
  beforeEach(() => {
    setDisplayCurrency('USD', 1)
  })

  test('CNY mode shows the converted value and reports USD on edit', () => {
    setDisplayCurrency('CNY', 7.3)
    const onUsdChange = vi.fn()
    render(<Harness value='2' onUsdChange={onUsdChange} />)
    const input = screen.getByRole('textbox')

    expect(input).toHaveValue('14.6')

    fireEvent.change(input, { target: { value: '20' } })
    expect(onUsdChange).toHaveBeenCalledTimes(1)
    const reportedUsd = onUsdChange.mock.calls[0][0]
    expect(Number(reportedUsd)).toBeCloseTo(20 / 7.3, 6)
  })

  test('blur normalizes the draft back to a clean display value', () => {
    setDisplayCurrency('CNY', 7.3)
    const onUsdChange = vi.fn()
    render(<Harness value='2' onUsdChange={onUsdChange} />)
    const input = screen.getByRole('textbox')

    fireEvent.change(input, { target: { value: '20' } })
    fireEvent.blur(input)

    expect(input).toHaveValue('20')
  })

  test('USD mode passes values through unchanged', () => {
    const onUsdChange = vi.fn()
    render(<Harness value='3.5' onUsdChange={onUsdChange} />)
    const input = screen.getByRole('textbox')

    expect(input).toHaveValue('3.5')

    fireEvent.change(input, { target: { value: '5' } })
    expect(onUsdChange).toHaveBeenCalledWith('5')
  })
})
