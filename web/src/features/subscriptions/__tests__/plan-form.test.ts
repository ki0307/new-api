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
import type { TFunction } from 'i18next'
import { describe, expect, test } from 'vitest'

import {
  formValuesToPlanPayload,
  getPlanFormSchema,
  PLAN_FORM_DEFAULTS,
  planToFormValues,
} from '../lib/plan-form'
import type { SubscriptionPlan } from '../types'

describe('subscription owner groups', () => {
  test('editing a multi-group plan preserves restrictions and upstream options', () => {
    const values = planToFormValues({
      ...PLAN_FORM_DEFAULTS,
      id: 1,
      owner_group: 'vip, svip',
      downgrade_group: 'default',
      allow_wallet_overflow: false,
      allow_balance_pay: false,
    } as unknown as SubscriptionPlan)
    expect(values.owner_group).toEqual(['vip', 'svip'])
    const payload = formValuesToPlanPayload(values)
    expect(payload.plan.owner_group).toBe('vip,svip')
    expect(payload.plan.downgrade_group).toBe('default')
    expect(payload.plan.allow_wallet_overflow).toBe(false)
    expect(payload.plan.allow_balance_pay).toBe(false)
  })

  test('clearing groups saves an unrestricted plan', () => {
    const schema = getPlanFormSchema(((key: string) => key) as TFunction)
    const values = schema.parse({
      ...PLAN_FORM_DEFAULTS,
      title: 'Basic',
      owner_group: [],
    })
    expect(formValuesToPlanPayload(values).plan.owner_group).toBe('')
  })
})
