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
  MATCH_RANGE,
  MATCH_WITHIN,
  buildRequestRuleExpr,
  combineBillingExpr,
  requestRuleGroupsFromTrace,
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
  type RequestCondition,
  type RequestRuleGroup,
  type TimeCondition,
} from '../billing-expr'

const BASE_EXPR = 'tier("base", p * 4.5 + c * 13.5 + cr * 0.15)'

// Weekday business hours: Mon-Fri, 9-12 or 14-18, priced at 2x.
const RULE_EXPR =
  '((weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5) && ((hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12) || (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18)) ? 2 : 1)'

const COMBINED_EXPR = `(${BASE_EXPR}) * ${RULE_EXPR}`

function timeCondition(cond: RequestCondition | undefined): TimeCondition {
  expect(cond?.source).toBe('time')
  return cond as TimeCondition
}

function expectBusinessHoursGroup(groups: RequestRuleGroup[] | null) {
  expect(groups).not.toBeNull()
  const group = groups?.[0]
  expect(group).toBeDefined()
  if (!group) return
  expect(group.multiplier).toBe('2')
  expect(group.conditions).toHaveLength(2)

  const weekday = timeCondition(group.conditions[0])
  expect(weekday.timeFunc).toBe('weekday')
  expect(weekday.timezone).toBe('Asia/Shanghai')
  expect(weekday.mode).toBe(MATCH_WITHIN)
  expect(weekday.intervals).toEqual([
    { start: '1', end: '5', endInclusive: true },
  ])

  const hour = timeCondition(group.conditions[1])
  expect(hour.timeFunc).toBe('hour')
  expect(hour.timezone).toBe('Asia/Shanghai')
  expect(hour.mode).toBe(MATCH_WITHIN)
  expect(hour.intervals).toEqual([
    { start: '9', end: '12', endInclusive: false },
    { start: '14', end: '18', endInclusive: false },
  ])
}

describe('splitBillingExprAndRequestRules', () => {
  test('splits a within-range + OR-union request rule from the billing expr', () => {
    const split = splitBillingExprAndRequestRules(COMBINED_EXPR)
    expect(split.billingExpr).toBe(BASE_EXPR)
    expect(split.requestRuleExpr).not.toBe('')
    const groups = tryParseRequestRuleExpr(split.requestRuleExpr)
    expectBusinessHoursGroup(groups)
  })

  test('keeps a bare tier expression untouched', () => {
    const split = splitBillingExprAndRequestRules(BASE_EXPR)
    expect(split.billingExpr).toBe(BASE_EXPR)
    expect(split.requestRuleExpr).toBe('')
  })
})

describe('tryParseRequestRuleExpr', () => {
  test('parses the business-hours rule into two time conditions', () => {
    const groups = tryParseRequestRuleExpr(RULE_EXPR)
    expectBusinessHoursGroup(groups)
  })

  test('parses the merged form where within-range && is not parenthesized', () => {
    const mergedRuleExpr =
      '(weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5 && ((hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12) || (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18)) ? 2 : 1)'
    const groups = tryParseRequestRuleExpr(mergedRuleExpr)
    expectBusinessHoursGroup(groups)
  })

  test('parses a single within-range interval', () => {
    const groups = tryParseRequestRuleExpr(
      '(hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12 ? 2 : 1)'
    )
    expect(groups).not.toBeNull()
    const group = groups?.[0]
    expect(group).toBeDefined()
    if (!group) return
    const condition = timeCondition(group.conditions[0])
    expect(condition.mode).toBe(MATCH_WITHIN)
    expect(condition.intervals).toEqual([
      { start: '9', end: '12', endInclusive: false },
    ])
    expect(group.multiplier).toBe('2')
  })

  test('still parses the overnight range shape', () => {
    const groups = tryParseRequestRuleExpr(
      '(hour("Asia/Shanghai") >= 21 || hour("Asia/Shanghai") < 6 ? 0.5 : 1)'
    )
    expect(groups).not.toBeNull()
    const group = groups?.[0]
    expect(group).toBeDefined()
    if (!group) return
    const condition = timeCondition(group.conditions[0])
    expect(condition.mode).toBe(MATCH_RANGE)
    expect(condition.rangeStart).toBe('21')
    expect(condition.rangeEnd).toBe('6')
  })
})

describe('requestRuleGroupsFromTrace', () => {
  test('parses the backend canonical trace condition into a friendly group', () => {
    const groups = requestRuleGroupsFromTrace([
      {
        cond: 'weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5 && ((hour("Asia/Shanghai") >= 9 && hour("Asia/Shanghai") < 12) || (hour("Asia/Shanghai") >= 14 && hour("Asia/Shanghai") < 18))',
        multiplier: 2,
        matched: true,
      },
    ])
    expectBusinessHoursGroup(groups)
    expect(groups[0].matched).toBe(true)
  })
})

describe('round-trip', () => {
  test('rebuilt rule expression splits and re-parses to the same groups', () => {
    const split = splitBillingExprAndRequestRules(COMBINED_EXPR)
    const groups = tryParseRequestRuleExpr(split.requestRuleExpr)
    expect(groups).not.toBeNull()
    if (!groups) return

    const rebuilt = combineBillingExpr(
      split.billingExpr,
      buildRequestRuleExpr(groups)
    )
    const rebuiltSplit = splitBillingExprAndRequestRules(rebuilt)
    expect(rebuiltSplit.billingExpr).toBe(BASE_EXPR)
    const reparsed = tryParseRequestRuleExpr(rebuiltSplit.requestRuleExpr)
    expect(reparsed).toEqual(groups)
  })

  test('round-trips a single within-range interval with an inclusive end', () => {
    const ruleExpr =
      '(weekday("Asia/Shanghai") >= 1 && weekday("Asia/Shanghai") <= 5 ? 2 : 1)'
    const groups = tryParseRequestRuleExpr(ruleExpr)
    expect(groups).not.toBeNull()
    if (!groups) return
    const rebuilt = buildRequestRuleExpr(groups)
    expect(rebuilt).toBe(ruleExpr)
  })
})
