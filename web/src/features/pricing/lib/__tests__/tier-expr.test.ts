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

import { scaleExprPrices } from '../tier-expr'

describe('scaleExprPrices', () => {
  test('scales token price coefficients by the factor', () => {
    expect(scaleExprPrices('tier("base", p * 2 + c * 15)', 7.3)).toBe(
      'tier("base", p * 14.6 + c * 109.5)'
    )
    expect(scaleExprPrices('tier("base", p * 14.6 + c * 109.5)', 1 / 7.3)).toBe(
      'tier("base", p * 2 + c * 15)'
    )
  })

  test('scales all price variables including cache and media lanes', () => {
    const expr = 'tier("base", p * 1 + c * 2 + cr * 0.1 + cc * 1.5 + cc1h * 3 + img * 0.5 + img_o * 1 + ai * 2 + ao * 4)'
    expect(scaleExprPrices(expr, 10)).toBe(
      'tier("base", p * 10 + c * 20 + cr * 1 + cc * 15 + cc1h * 30 + img * 5 + img_o * 10 + ai * 20 + ao * 40)'
    )
  })

  test('leaves tier conditions untouched', () => {
    const expr =
      'len <= 200000 ? tier("standard", p * 3 + c * 15) : tier("long", p * 6 + c * 22.5)'
    expect(scaleExprPrices(expr, 7.3)).toBe(
      'len <= 200000 ? tier("standard", p * 21.9 + c * 109.5) : tier("long", p * 43.8 + c * 164.25)'
    )
  })

  test('leaves request rule multipliers untouched', () => {
    const expr = '(tier("base", p * 2 + c * 4)) * req(header("x") == "y") * 0.5'
    expect(scaleExprPrices(expr, 7.3)).toBe(
      '(tier("base", p * 14.6 + c * 29.2)) * req(header("x") == "y") * 0.5'
    )
  })

  test('does not confuse condition comparisons with prices', () => {
    const expr = 'c < 200 && p >= 100 ? tier("base", p * 2 + c * 4) : tier("fb", p * 0 + c * 0)'
    expect(scaleExprPrices(expr, 2)).toBe(
      'c < 200 && p >= 100 ? tier("base", p * 4 + c * 8) : tier("fb", p * 0 + c * 0)'
    )
  })

  test('factor of 1 returns the expression unchanged', () => {
    const expr = 'tier("base", p * 2 + c * 4)'
    expect(scaleExprPrices(expr, 1)).toBe(expr)
  })

  test('empty or missing expression returns as-is', () => {
    expect(scaleExprPrices('', 7.3)).toBe('')
    expect(scaleExprPrices('   ', 7.3)).toBe('   ')
  })

  test('round-trip restores the original coefficients', () => {
    const expr =
      'len <= 200000 ? tier("standard", p * 3 + c * 15 + cr * 0.3) : tier("long", p * 6 + c * 22.5)'
    expect(scaleExprPrices(scaleExprPrices(expr, 7.3), 1 / 7.3)).toBe(expr)
  })
})