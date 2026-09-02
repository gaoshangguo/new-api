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
import { describe, expect, test } from 'vitest'

import { displayToUsdValue, usdToDisplayValue } from '../pricing-currency'

describe('pricing currency conversion', () => {
  test('usdToDisplayValue multiplies by the exchange rate', () => {
    expect(usdToDisplayValue('2', 7.3)).toBe('14.6')
    expect(usdToDisplayValue('0.04', 7.3)).toBe('0.292')
  })

  test('displayToUsdValue divides by the exchange rate', () => {
    expect(displayToUsdValue('14.6', 7.3)).toBe('2')
    expect(displayToUsdValue('0.292', 7.3)).toBe('0.04')
  })

  test('round-trip restores the original displayed value', () => {
    expect(usdToDisplayValue(displayToUsdValue('20', 7.3), 7.3)).toBe('20')
    expect(usdToDisplayValue(displayToUsdValue('21.9', 7.3), 7.3)).toBe('21.9')
    expect(usdToDisplayValue(displayToUsdValue('0.0000146', 7.3), 7.3)).toBe(
      '0.0000146'
    )
  })

  test('empty input passes through as empty', () => {
    expect(usdToDisplayValue('', 7.3)).toBe('')
    expect(displayToUsdValue('', 7.3)).toBe('')
  })

  test('rate of 1 is identity (USD mode)', () => {
    expect(usdToDisplayValue('3.5', 1)).toBe('3.5')
    expect(displayToUsdValue('3.5', 1)).toBe('3.5')
  })
})
