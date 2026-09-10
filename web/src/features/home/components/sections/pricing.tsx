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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Check } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Button } from '@/components/ui/button'
import { formatDuration, formatResetPeriod } from '@/features/subscriptions/lib'
import type { ApiResponse, PlanRecord } from '@/features/subscriptions/types'
import { formatQuota } from '@/lib/format'
import { api } from '@/lib/http-client'

const MAX_PLANS = 3

interface PricingProps {
  isAuthenticated?: boolean
}

/**
 * Subscription plan section. Plans come from the same public endpoint the
 * wallet purchase flow uses, so pricing stays managed in one place.
 */
export function Pricing(props: PricingProps) {
  const { t } = useTranslation()

  const { data } = useQuery({
    queryKey: ['home', 'public-plans'],
    // The landing page is public: a plan fetch must never trigger the global
    // session-expired redirect, so auth retry and error toasts stay off.
    queryFn: async () => {
      const response = await api.get<ApiResponse<PlanRecord[]>>(
        '/api/subscription/plans',
        { skipAuthRefresh: true, skipErrorHandler: true }
      )
      return response.data
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
  })

  const plans = (data?.data ?? [])
    .map((record) => record.plan)
    .filter((plan) => plan.enabled)
    .sort((a, b) => a.sort_order - b.sort_order)
    .slice(0, MAX_PLANS)

  if (plans.length === 0) {
    return null
  }

  const popularIndex = plans.length >= 3 ? 1 : 0
  const purchaseLink = props.isAuthenticated ? '/wallet' : '/sign-up'

  return (
    <section className='px-6 py-24 md:py-32'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-14 text-center'>
          <h2 className='text-2xl font-bold tracking-tight md:text-3xl'>
            {t('Choose your plan')}
          </h2>
          <p className='text-muted-foreground mt-3 text-sm'>
            {t('Flexible quota options for every stage.')}
          </p>
        </AnimateInView>

        <div className='grid gap-4 md:grid-cols-3 md:gap-6'>
          {plans.map((plan, index) => {
            const isPopular = index === popularIndex
            const totalAmount = Number(plan.total_amount || 0)
            const price = Number(plan.price_amount || 0).toFixed(2)
            const resetPeriod = formatResetPeriod(plan, t)

            const benefits = [
              `${t('Validity Period')}: ${formatDuration(plan, t)}`,
              resetPeriod !== t('No Reset')
                ? `${t('Quota Reset')}: ${resetPeriod}`
                : null,
              totalAmount > 0
                ? `${t('Total Quota')}: ${formatQuota(totalAmount)}`
                : `${t('Total Quota')}: ${t('Unlimited')}`,
            ].filter(Boolean) as string[]

            return (
              <AnimateInView
                key={plan.id}
                delay={index * 80}
                className={
                  isPopular
                    ? 'border-primary/60 bg-background relative flex flex-col rounded-2xl border p-6 shadow-sm md:p-7'
                    : 'border-border/60 bg-background relative flex flex-col rounded-2xl border p-6 md:p-7'
                }
              >
                {isPopular && (
                  <span className='bg-primary/10 text-primary absolute -top-2.5 left-6 rounded-full px-2.5 py-1 text-[10px] font-semibold tracking-wide'>
                    {t('Recommended')}
                  </span>
                )}
                <h3 className='text-sm font-semibold'>
                  {plan.title || t('Subscription Plans')}
                </h3>
                {plan.subtitle && (
                  <p className='text-muted-foreground mt-1.5 text-xs leading-relaxed'>
                    {plan.subtitle}
                  </p>
                )}
                <div className='mt-5 mb-6'>
                  <span className='text-3xl font-bold tracking-tight'>
                    ${price}
                  </span>
                </div>
                <div className='flex-1 space-y-2.5'>
                  {benefits.map((benefit) => (
                    <div
                      key={benefit}
                      className='text-muted-foreground flex items-center gap-2 text-xs'
                    >
                      <Check className='text-primary size-3.5 shrink-0' />
                      <span>{benefit}</span>
                    </div>
                  ))}
                </div>
                <Button
                  variant={isPopular ? 'default' : 'outline'}
                  className='mt-7 w-full'
                  render={<Link to={purchaseLink} />}
                >
                  {t('Subscribe Now')}
                </Button>
              </AnimateInView>
            )
          })}
        </div>
      </div>
    </section>
  )
}
