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
import { useTranslation } from 'react-i18next'

import { usePricingData } from '@/features/pricing/hooks'
import { getLobeIcon } from '@/lib/lobe-icon'

const MIN_MARQUEE_ITEMS = 16
const MAX_VENDORS = 14

/**
 * Infinite logo marquee of the configured upstream vendors. Data comes from
 * the same public pricing endpoint that powers the models marketplace, so the
 * home page never duplicates the vendor list.
 */
export function ModelsMarquee() {
  const { t } = useTranslation()
  const { vendors, isLoading } = usePricingData()

  const visibleVendors = vendors
    .filter((vendor) => Boolean(vendor.icon))
    .slice(0, MAX_VENDORS)

  if (isLoading || visibleVendors.length === 0) {
    return null
  }

  // Repeat the vendor set so each half of the track fills wide screens; the
  // animation loops seamlessly because both halves are identical.
  const repeats = Math.max(
    1,
    Math.ceil(MIN_MARQUEE_ITEMS / visibleVendors.length)
  )
  const half = Array.from({ length: repeats }).flatMap(() => visibleVendors)

  return (
    <section
      aria-label={t('Models')}
      className='border-border/40 relative z-10 border-y'
    >
      <div className='mx-auto max-w-6xl px-6 py-8 md:py-10'>
        <p className='text-muted-foreground mb-5 text-center text-xs font-medium tracking-widest uppercase'>
          {t('Models')}
        </p>
        <div className='marquee-container relative overflow-hidden [mask-image:linear-gradient(to_right,transparent,black_12%,black_88%,transparent)]'>
          <div className='animate-scroll-left flex w-max items-center gap-3'>
            {[0, 1].map((copy) => (
              <div
                key={copy}
                aria-hidden={copy === 1}
                className='flex items-center gap-3'
              >
                {half.map((vendor, index) => (
                  <span
                    key={`${copy}-${vendor.id}-${index}`}
                    className='border-border/50 bg-background text-foreground/80 flex shrink-0 items-center gap-2 rounded-full border px-4 py-2 text-xs font-medium'
                  >
                    {getLobeIcon(vendor.icon as string, 16)}
                    <span>{vendor.name}</span>
                  </span>
                ))}
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
