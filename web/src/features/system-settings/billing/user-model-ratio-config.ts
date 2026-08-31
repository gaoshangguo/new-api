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

// 用户级模型倍率覆盖配置解析（语义 A：替换，不叠加）。
// 结构：{ userId: { modelName: ratio } }，ratio 必须是非负的有限数。

export type UserModelRatioConfig = Record<string, Record<string, number>>

export type ParseUserModelRatioResult =
  | { ok: true; config: UserModelRatioConfig }
  | { ok: false; error: string }

export function parseUserModelRatioConfig(
  value: string
): ParseUserModelRatioResult {
  const trimmed = value.trim()
  if (!trimmed) {
    return { ok: true, config: {} }
  }

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return { ok: false, error: 'Invalid JSON' }
  }

  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { ok: false, error: 'JSON must be an object' }
  }

  const config: UserModelRatioConfig = {}
  for (const [userId, models] of Object.entries(parsed)) {
    if (models === null || typeof models !== 'object' || Array.isArray(models)) {
      return { ok: false, error: 'JSON must be an object' }
    }
    const modelRatios: Record<string, number> = {}
    for (const [modelName, ratio] of Object.entries(models)) {
      if (typeof ratio !== 'number' || !Number.isFinite(ratio) || ratio < 0) {
        return { ok: false, error: 'Ratio must be a non-negative number' }
      }
      modelRatios[modelName] = ratio
    }
    config[userId] = modelRatios
  }
  return { ok: true, config }
}

export function formatUserModelRatioConfig(config: UserModelRatioConfig): string {
  return JSON.stringify(config, null, 2)
}

// ----------------------------------------------------------------------------
// 可视化编辑器行模型：表格行 ⇄ 嵌套 JSON 的转换（userId -> {model -> ratio}）
// ----------------------------------------------------------------------------

export type UserModelRatioRow = {
  id: number
  userId: string
  model: string
  ratio: string
}

export function configToRows(
  config: UserModelRatioConfig,
  startId = 1
): UserModelRatioRow[] {
  const rows: UserModelRatioRow[] = []
  let nextId = startId
  for (const [userId, models] of Object.entries(config)) {
    for (const [model, ratio] of Object.entries(models)) {
      rows.push({ id: nextId++, userId, model, ratio: String(ratio) })
    }
  }
  return rows
}

export type RowsToConfigResult =
  | { ok: true; config: UserModelRatioConfig }
  | { ok: false; error: string }

export function rowsToConfig(rows: UserModelRatioRow[]): RowsToConfigResult {
  const config: UserModelRatioConfig = {}
  const seen = new Set<string>()
  for (const row of rows) {
    const userId = row.userId.trim()
    const model = row.model.trim()
    const ratioText = row.ratio.trim()
    if (!/^\d+$/.test(userId) || Number(userId) <= 0) {
      return { ok: false, error: 'User ID must be a positive integer' }
    }
    if (!model) {
      return { ok: false, error: 'Model name is required' }
    }
    const ratio = Number(ratioText)
    if (ratioText === '' || !Number.isFinite(ratio) || ratio < 0) {
      return { ok: false, error: 'Ratio must be a non-negative number' }
    }
    const pairKey = `${userId}\u0000${model}`
    if (seen.has(pairKey)) {
      return { ok: false, error: 'Duplicate entries for the same user and model' }
    }
    seen.add(pairKey)
    if (!config[userId]) {
      config[userId] = {}
    }
    config[userId][model] = ratio
  }
  return { ok: true, config }
}