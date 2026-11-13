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

import type { UsageLog } from '../../data/schema'
import { formatModelName } from '../format'

function usageLog(other: Record<string, unknown>): UsageLog {
  return {
    model_name: 'real-upstream-model',
    other: JSON.stringify(other),
  } as UsageLog
}

describe('formatModelName', () => {
  test('admin view keeps the real model name and mapping detail', () => {
    const info = formatModelName(
      usageLog({
        is_model_mapped: true,
        upstream_model_name: 'channel-upstream-model',
      })
    )

    expect(info.name).toBe('real-upstream-model')
    expect(info.isMapped).toBe(true)
    expect(info.actualModel).toBe('channel-upstream-model')
  })

  test('user view prefers the public alias and hides the upstream model', () => {
    const info = formatModelName(
      usageLog({
        alias_name: 'public-alias',
        is_model_mapped: true,
        upstream_model_name: 'channel-upstream-model',
      }),
      true
    )

    expect(info.name).toBe('public-alias')
    expect(info.isMapped).toBe(false)
    expect(info.actualModel).toBeUndefined()
  })

  test('user view without an alias keeps the logged model name', () => {
    const info = formatModelName(usageLog({}), true)

    expect(info.name).toBe('real-upstream-model')
    expect(info.isMapped).toBe(false)
  })
})
