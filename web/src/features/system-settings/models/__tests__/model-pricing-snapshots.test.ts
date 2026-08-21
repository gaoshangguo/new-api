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

import {
  buildModelSnapshots,
  getModeLabel,
  getPriceDetail,
  getPriceSummary,
  getSnapshotSignature,
  isBasePricingUnset,
  type ModelPricingSnapshotInput,
} from '../model-pricing-snapshots'

const identity = (key: string) => key

function snapshotInput(overrides: Partial<ModelPricingSnapshotInput> = {}) {
  const empty: Record<string, string> = {}
  return {
    modelPrice: JSON.stringify(empty),
    modelRatio: JSON.stringify(empty),
    cacheRatio: JSON.stringify(empty),
    createCacheRatio: JSON.stringify(empty),
    completionRatio: JSON.stringify(empty),
    imageRatio: JSON.stringify(empty),
    audioRatio: JSON.stringify(empty),
    audioCompletionRatio: JSON.stringify(empty),
    billingMode: JSON.stringify(empty),
    billingExpr: JSON.stringify(empty),
    billingDurationPrice: JSON.stringify(empty),
    ...overrides,
  }
}

const perDurationInput = (prices: Record<string, number>) =>
  snapshotInput({
    billingMode: JSON.stringify({ 'seedance-video': 'per_duration' }),
    billingDurationPrice: JSON.stringify({ 'seedance-video': prices }),
  })

describe('model pricing snapshots - per-duration billing', () => {
  test('buildModelSnapshots infers per-duration mode and per-resolution prices', () => {
    const rows = buildModelSnapshots(
      perDurationInput({ '720p': 0.027, '1080p': 0.041, '4k': 0.081 })
    )

    expect(rows).toHaveLength(1)
    expect(rows[0].name).toBe('seedance-video')
    expect(rows[0].billingMode).toBe('per-duration')
    expect(rows[0].durationPrices).toEqual({
      '720p': '0.027',
      '1080p': '0.041',
      '4k': '0.081',
    })
    expect(rows[0].hasConflict).toBe(false)
  })

  test('getModeLabel maps per-duration to its display label', () => {
    expect(getModeLabel('per-duration')).toBe('Per-duration')
  })

  test('per-duration price summary lists each configured resolution', () => {
    const [row] = buildModelSnapshots(
      perDurationInput({ '720p': 0.027, '1080p': 0.041, '4k': 0.081 })
    )

    expect(getPriceSummary(row, identity)).toBe(
      '720p $0.027 · 1080p $0.041 · 4k $0.081'
    )
    expect(getPriceDetail(row, identity)).toBe(
      'Charged by generated duration'
    )
  })

  test('isBasePricingUnset treats per-duration as configured', () => {
    const [row] = buildModelSnapshots(perDurationInput({ '720p': 0.027 }))
    expect(isBasePricingUnset(row)).toBe(false)
  })

  test('snapshot signature captures resolution price changes', () => {
    const [before] = buildModelSnapshots(perDurationInput({ '720p': 0.027 }))
    const [after] = buildModelSnapshots(perDurationInput({ '720p': 0.031 }))

    expect(getSnapshotSignature(before)).not.toBe(getSnapshotSignature(after))
  })
})
