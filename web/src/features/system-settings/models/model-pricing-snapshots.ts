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
import { splitBillingExprAndRequestRules } from '@/features/pricing/lib/billing-expr'

import { safeJsonParse } from '../utils/json-parser'
import {
  DURATION_RESOLUTION_KEYS,
  type DurationResolutionKey,
  type PriceDisplay,
} from './model-pricing-core'
import { formatPricingNumber } from './pricing-format'

const USD_DISPLAY: PriceDisplay = {
  symbol: '$',
  toDisplay: (usdValue) => usdValue,
}

const formatPrice = (
  usdValue: string | undefined,
  display: PriceDisplay = USD_DISPLAY
) => (usdValue ? `${display.symbol}${display.toDisplay(usdValue)}` : '')

export type ModelPricingSnapshotInput = {
  modelPrice: string
  modelRatio: string
  cacheRatio: string
  createCacheRatio: string
  completionRatio: string
  imageRatio: string
  audioRatio: string
  audioCompletionRatio: string
  billingMode: string
  billingExpr: string
  billingDurationPrice: string
}

export type ModelPricingSnapshot = {
  name: string
  price?: string
  ratio?: string
  cacheRatio?: string
  createCacheRatio?: string
  completionRatio?: string
  imageRatio?: string
  audioRatio?: string
  audioCompletionRatio?: string
  durationPrices?: Partial<Record<DurationResolutionKey, string>>
  billingMode?: string
  billingExpr?: string
  requestRuleExpr?: string
  hasConflict: boolean
}

export type ModelRow = ModelPricingSnapshot & {
  saved?: ModelPricingSnapshot
  draft?: ModelPricingSnapshot
  isDraftChanged: boolean
  isDraftDeleted: boolean
  isDraftNew: boolean
}

export const hasPricingValue = (value?: string) =>
  value !== undefined && value !== ''

export const isBasePricingUnset = (snapshot?: ModelPricingSnapshot) =>
  !snapshot ||
  (snapshot.billingMode !== 'tiered_expr' &&
    snapshot.billingMode !== 'per-duration' &&
    !hasPricingValue(snapshot.price) &&
    !hasPricingValue(snapshot.ratio))

const toNumberOrNull = (value?: string) => {
  if (!hasPricingValue(value)) return null
  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

const ratioToPrice = (ratio?: string, denominator?: string) => {
  const ratioNumber = toNumberOrNull(ratio)
  const denominatorNumber = denominator ? toNumberOrNull(denominator) : 2
  if (ratioNumber === null || denominatorNumber === null) return ''
  return formatPricingNumber(ratioNumber * denominatorNumber)
}

export const getModeLabel = (mode?: string) => {
  if (mode === 'per-request') return 'Per-request'
  if (mode === 'tiered_expr') return 'Expression'
  if (mode === 'per-duration') return 'Per-duration'
  return 'Per-token'
}

export const getModeVariant = (
  mode?: string
): 'warning' | 'info' | 'success' => {
  if (mode === 'per-request') return 'warning'
  if (mode === 'tiered_expr') return 'info'
  if (mode === 'per-duration') return 'info'
  return 'success'
}

const getExpressionSummary = (
  row: ModelPricingSnapshot,
  t: (key: string) => string
) => {
  const tierCount = (row.billingExpr?.match(/tier\(/g) || []).length
  if (tierCount > 0) {
    return `${t('Tiered pricing')} · ${tierCount} ${t('tiers')}`
  }
  return t('Expression pricing')
}

export const getPriceSummary = (
  row: ModelPricingSnapshot,
  t: (key: string) => string,
  display: PriceDisplay = USD_DISPLAY
) => {
  if (row.billingMode === 'tiered_expr') {
    return getExpressionSummary(row, t)
  }
  if (row.billingMode === 'per-request') {
    return row.price
      ? `${formatPrice(row.price, display)} / ${t('request')}`
      : t('Unset price')
  }
  if (row.billingMode === 'per-duration') {
    const entries = DURATION_RESOLUTION_KEYS.filter((key) =>
      hasPricingValue(row.durationPrices?.[key])
    )
    if (entries.length === 0) return t('Unset price')
    return entries
      .map((key) => `${key} ${formatPrice(row.durationPrices?.[key], display)}`)
      .join(' · ')
  }

  const inputPrice = ratioToPrice(row.ratio)
  if (!inputPrice) return t('Unset price')

  const extraCount = [
    row.completionRatio,
    row.cacheRatio,
    row.createCacheRatio,
    row.imageRatio,
    row.audioRatio,
    row.audioCompletionRatio,
  ].filter(hasPricingValue).length

  return extraCount > 0
    ? `${t('Input')} ${formatPrice(inputPrice, display)} · ${extraCount} ${t(
        'extras'
      )}`
    : `${t('Input')} ${formatPrice(inputPrice, display)}`
}

export const getPriceDetail = (
  row: ModelPricingSnapshot,
  t: (key: string) => string,
  display: PriceDisplay = USD_DISPLAY
) => {
  if (row.billingMode === 'tiered_expr') {
    return row.requestRuleExpr
      ? t('Includes request rules')
      : t('Expression based')
  }
  if (row.billingMode === 'per-request') {
    return t('Fixed request price')
  }
  if (row.billingMode === 'per-duration') {
    return t('Charged by generated duration')
  }

  const inputPrice = ratioToPrice(row.ratio)
  if (!inputPrice) return t('No base input price')

  const details = [
    row.completionRatio &&
      `${t('Output')} ${formatPrice(
        ratioToPrice(row.completionRatio, inputPrice),
        display
      )}`,
    row.cacheRatio &&
      `${t('Cache')} ${formatPrice(
        ratioToPrice(row.cacheRatio, inputPrice),
        display
      )}`,
    row.createCacheRatio &&
      `${t('Cache write')} ${formatPrice(
        ratioToPrice(row.createCacheRatio, inputPrice),
        display
      )}`,
  ]
    .filter(Boolean)
    .slice(0, 2)

  return details.length > 0 ? details.join(' · ') : t('Base input price only')
}

export const buildModelSnapshots = ({
  modelPrice,
  modelRatio,
  cacheRatio,
  createCacheRatio,
  completionRatio,
  imageRatio,
  audioRatio,
  audioCompletionRatio,
  billingMode,
  billingExpr,
  billingDurationPrice,
}: ModelPricingSnapshotInput): ModelPricingSnapshot[] => {
  const priceMap = safeJsonParse<Record<string, number>>(modelPrice, {
    fallback: {},
    context: 'model prices',
  })
  const ratioMap = safeJsonParse<Record<string, number>>(modelRatio, {
    fallback: {},
    context: 'model ratios',
  })
  const cacheMap = safeJsonParse<Record<string, number>>(cacheRatio, {
    fallback: {},
    context: 'cache ratios',
  })
  const createCacheMap = safeJsonParse<Record<string, number>>(
    createCacheRatio,
    { fallback: {}, context: 'create cache ratios' }
  )
  const completionMap = safeJsonParse<Record<string, number>>(completionRatio, {
    fallback: {},
    context: 'completion ratios',
  })
  const imageMap = safeJsonParse<Record<string, number>>(imageRatio, {
    fallback: {},
    context: 'image ratios',
  })
  const audioMap = safeJsonParse<Record<string, number>>(audioRatio, {
    fallback: {},
    context: 'audio ratios',
  })
  const audioCompletionMap = safeJsonParse<Record<string, number>>(
    audioCompletionRatio,
    { fallback: {}, context: 'audio completion ratios' }
  )
  const billingModeMap = safeJsonParse<Record<string, string>>(billingMode, {
    fallback: {},
    context: 'billing mode',
  })
  const billingExprMap = safeJsonParse<Record<string, string>>(billingExpr, {
    fallback: {},
    context: 'billing expression',
  })
  const billingDurationPriceMap = safeJsonParse<
    Record<string, Record<string, number>>
  >(billingDurationPrice, { fallback: {}, context: 'billing duration price' })

  const modelNames = new Set([
    ...Object.keys(priceMap),
    ...Object.keys(ratioMap),
    ...Object.keys(cacheMap),
    ...Object.keys(createCacheMap),
    ...Object.keys(completionMap),
    ...Object.keys(imageMap),
    ...Object.keys(audioMap),
    ...Object.keys(audioCompletionMap),
    ...Object.keys(billingModeMap),
    ...Object.keys(billingExprMap),
    ...Object.keys(billingDurationPriceMap),
  ])

  return [...modelNames].map((name) => {
    const price = priceMap[name]?.toString() || ''
    const ratio = ratioMap[name]?.toString() || ''
    const cache = cacheMap[name]?.toString() || ''
    const createCache = createCacheMap[name]?.toString() || ''
    const completion = completionMap[name]?.toString() || ''
    const image = imageMap[name]?.toString() || ''
    const audio = audioMap[name]?.toString() || ''
    const audioCompletion = audioCompletionMap[name]?.toString() || ''
    const rawDurationPrices = billingDurationPriceMap[name]
    const durationPrices: Partial<Record<DurationResolutionKey, string>> = {}
    if (rawDurationPrices) {
      DURATION_RESOLUTION_KEYS.forEach((key) => {
        if (rawDurationPrices[key] != null) {
          durationPrices[key] = String(rawDurationPrices[key])
        }
      })
    }

    const modeForModel = billingModeMap[name]
    if (modeForModel === 'tiered_expr') {
      const fullExpr = billingExprMap[name] || ''
      const { billingExpr: pureExpr, requestRuleExpr } =
        splitBillingExprAndRequestRules(fullExpr)
      return {
        name,
        billingMode: 'tiered_expr',
        billingExpr: pureExpr,
        requestRuleExpr,
        price,
        ratio,
        cacheRatio: cache,
        createCacheRatio: createCache,
        completionRatio: completion,
        imageRatio: image,
        audioRatio: audio,
        audioCompletionRatio: audioCompletion,
        durationPrices,
        hasConflict: false,
      }
    }

    if (modeForModel === 'per_duration') {
      return {
        name,
        billingMode: 'per-duration',
        price,
        ratio,
        cacheRatio: cache,
        createCacheRatio: createCache,
        completionRatio: completion,
        imageRatio: image,
        audioRatio: audio,
        audioCompletionRatio: audioCompletion,
        durationPrices,
        hasConflict: false,
      }
    }

    return {
      name,
      price,
      ratio,
      cacheRatio: cache,
      createCacheRatio: createCache,
      completionRatio: completion,
      imageRatio: image,
      audioRatio: audio,
      audioCompletionRatio: audioCompletion,
      durationPrices,
      billingMode: price !== '' ? 'per-request' : 'per-token',
      hasConflict:
        price !== '' &&
        (ratio !== '' ||
          completion !== '' ||
          cache !== '' ||
          createCache !== '' ||
          image !== '' ||
          audio !== '' ||
          audioCompletion !== ''),
    }
  })
}

export const getSnapshotSignature = (snapshot?: ModelPricingSnapshot) => {
  if (!snapshot) return ''
  return JSON.stringify({
    price: snapshot.price || '',
    ratio: snapshot.ratio || '',
    cacheRatio: snapshot.cacheRatio || '',
    createCacheRatio: snapshot.createCacheRatio || '',
    completionRatio: snapshot.completionRatio || '',
    imageRatio: snapshot.imageRatio || '',
    audioRatio: snapshot.audioRatio || '',
    audioCompletionRatio: snapshot.audioCompletionRatio || '',
    durationPrices: snapshot.durationPrices
      ? DURATION_RESOLUTION_KEYS.reduce(
          (acc, key) => {
            acc[key] = snapshot.durationPrices?.[key] || ''
            return acc
          },
          {} as Record<DurationResolutionKey, string>
        )
      : {},
    billingMode: snapshot.billingMode || 'per-token',
    billingExpr: snapshot.billingExpr || '',
    requestRuleExpr: snapshot.requestRuleExpr || '',
  })
}
