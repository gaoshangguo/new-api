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
*/
import { describe, expect, test } from 'vitest'

import {
  configToRows,
  formatUserModelRatioConfig,
  parseUserModelRatioConfig,
  rowsToConfig,
} from '../user-model-ratio-config'

describe('user model ratio config validation', () => {
  test('accepts a valid nested config and normalizes it', () => {
    const result = parseUserModelRatioConfig(
      '{"100": {"gpt-4o": 0.9, "gpt-4o-mini": 1}, "102": {"gpt-4o": 0}}'
    )
    expect(result.ok).toBe(true)
    if (!result.ok) return
    expect(result.config).toEqual({
      '100': { 'gpt-4o': 0.9, 'gpt-4o-mini': 1 },
      '102': { 'gpt-4o': 0 },
    })
  })

  test('accepts an empty string as an empty config', () => {
    const result = parseUserModelRatioConfig('')
    expect(result).toEqual({ ok: true, config: {} })
  })

  test('rejects malformed JSON', () => {
    const result = parseUserModelRatioConfig('{"100": ')
    expect(result.ok).toBe(false)
    if (result.ok) return
    expect(result.error).toBe('Invalid JSON')
  })

  test('rejects non-object payloads', () => {
    for (const value of ['[]', '"text"', '42', 'null']) {
      const result = parseUserModelRatioConfig(value)
      expect(result.ok).toBe(false)
      if (result.ok) continue
      expect(result.error).toBe('JSON must be an object')
    }
  })

  test('rejects a user whose value is not a model map', () => {
    const result = parseUserModelRatioConfig('{"100": 0.9}')
    expect(result.ok).toBe(false)
    if (result.ok) return
    expect(result.error).toBe('JSON must be an object')
  })

  test('rejects negative, non-finite, and non-number ratios', () => {
    for (const value of [
      '{"100": {"gpt-4o": -0.1}}',
      '{"100": {"gpt-4o": 1e999}}',
      '{"100": {"gpt-4o": "0.9"}}',
    ]) {
      const result = parseUserModelRatioConfig(value)
      expect(result.ok).toBe(false)
      if (result.ok) continue
      expect(result.error).toBe('Ratio must be a non-negative number')
    }
  })

  test('format round-trips a parsed config', () => {
    const parsed = parseUserModelRatioConfig('{"7": {"gemini-pro": 1.25}}')
    expect(parsed.ok).toBe(true)
    if (!parsed.ok) return
    const formatted = formatUserModelRatioConfig(parsed.config)
    expect(JSON.parse(formatted)).toEqual({ '7': { 'gemini-pro': 1.25 } })
  })
})

describe('user model ratio rows (visual editor)', () => {
  test('configToRows flattens the nested config into editable rows', () => {
    const rows = configToRows({
      '100': { 'gpt-4o': 0.9, 'gpt-4o-mini': 1 },
      '102': { 'gpt-4o': 0 },
    })
    expect(rows).toEqual([
      { id: 1, userId: '100', model: 'gpt-4o', ratio: '0.9' },
      { id: 2, userId: '100', model: 'gpt-4o-mini', ratio: '1' },
      { id: 3, userId: '102', model: 'gpt-4o', ratio: '0' },
    ])
  })

  test('rowsToConfig rebuilds the nested config and trims inputs', () => {
    const result = rowsToConfig([
      { id: 1, userId: ' 100 ', model: ' gpt-4o ', ratio: '0.9' },
      { id: 2, userId: '102', model: 'gpt-4o-mini', ratio: '1' },
    ])
    expect(result).toEqual({
      ok: true,
      config: { '100': { 'gpt-4o': 0.9 }, '102': { 'gpt-4o-mini': 1 } },
    })
  })

  test('rows round-trip through configToRows and rowsToConfig', () => {
    const config = { '7': { 'gemini-pro': 1.25, 'gemini-flash': 0 } }
    const rows = configToRows(config)
    const result = rowsToConfig(rows)
    expect(result).toEqual({ ok: true, config })
  })

  test('rowsToConfig rejects non-positive or non-numeric user ids', () => {
    for (const userId of ['', 'abc', '0', '-1', '1.5']) {
      const result = rowsToConfig([
        { id: 1, userId, model: 'gpt-4o', ratio: '0.9' },
      ])
      expect(result.ok).toBe(false)
      if (result.ok) continue
      expect(result.error).toBe('User ID must be a positive integer')
    }
  })

  test('rowsToConfig rejects empty model names', () => {
    const result = rowsToConfig([
      { id: 1, userId: '100', model: '  ', ratio: '0.9' },
    ])
    expect(result.ok).toBe(false)
    if (result.ok) return
    expect(result.error).toBe('Model name is required')
  })

  test('rowsToConfig rejects empty, negative, and non-finite ratios', () => {
    for (const ratio of ['', '-0.1', '1e999', 'abc']) {
      const result = rowsToConfig([
        { id: 1, userId: '100', model: 'gpt-4o', ratio },
      ])
      expect(result.ok).toBe(false)
      if (result.ok) continue
      expect(result.error).toBe('Ratio must be a non-negative number')
    }
  })

  test('rowsToConfig rejects duplicate user and model pairs', () => {
    const result = rowsToConfig([
      { id: 1, userId: '100', model: 'gpt-4o', ratio: '0.9' },
      { id: 2, userId: '100', model: 'gpt-4o', ratio: '1' },
    ])
    expect(result.ok).toBe(false)
    if (result.ok) return
    expect(result.error).toBe('Duplicate entries for the same user and model')
  })
})