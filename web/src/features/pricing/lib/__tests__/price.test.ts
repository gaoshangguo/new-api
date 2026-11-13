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
import { beforeEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import type { PricingModel } from '../../types'
import {
  formatDurationPriceSummary,
  formatVideoTokenPriceSummary,
} from '../price'

function baseModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: [],
    ...overrides,
  }
}

describe('formatDurationPriceSummary', () => {
  beforeEach(() => {
    useSystemConfigStore.setState({
      config: {
        ...useSystemConfigStore.getState().config,
        currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'USD' },
      },
    })
  })

  test('uses the fallback (default) per-second price when configured', () => {
    const model = baseModel({
      billing_mode: 'per_duration',
      billing_duration_price: { '720p': 0.02, default: 0.04 },
    })
    expect(formatDurationPriceSummary(model)).toBe('$0.04')
  })

  test('falls back to the first finite resolution when no default price', () => {
    const model = baseModel({
      billing_mode: 'per_duration',
      billing_duration_price: { '1080p': 0.09 },
    })
    expect(formatDurationPriceSummary(model)).toBe('$0.09')
  })

  test('applies the recharge rate when enabled', () => {
    const model = baseModel({
      billing_mode: 'per_duration',
      billing_duration_price: { default: 0.05 },
    })
    // 0.05 × 4 / 7 ≈ 0.0286
    expect(formatDurationPriceSummary(model, true, 4, 7)).toBe('$0.0286')
  })

  test('returns empty string when the model has no per-duration price', () => {
    expect(formatDurationPriceSummary(baseModel())).toBe('')
  })
})

describe('formatVideoTokenPriceSummary', () => {
  beforeEach(() => {
    useSystemConfigStore.setState({
      config: {
        ...useSystemConfigStore.getState().config,
        currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'USD' },
      },
    })
  })

  test('uses the fallback per-1M-token price when configured', () => {
    const model = baseModel({
      billing_mode: 'video_token',
      billing_token_price: { '720p': 1.2, default: 1.5 },
    })
    expect(formatVideoTokenPriceSummary(model)).toBe('$1.5')
  })

  test('falls back to the first finite resolution price', () => {
    const model = baseModel({
      billing_mode: 'video_token',
      billing_token_price: { '1080p': 1.8 },
    })
    expect(formatVideoTokenPriceSummary(model)).toBe('$1.8')
  })

  test('returns empty string when the model has no token price', () => {
    expect(formatVideoTokenPriceSummary(baseModel())).toBe('')
  })
})
