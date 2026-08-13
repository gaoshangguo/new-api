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
import {
  Building2,
  Gauge,
  Layers3,
  ShieldCheck,
  WalletCards,
  Workflow,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

const sections = [
  {
    label: 'Are these challenges slowing you down?',
    items: [
      {
        title: 'Too many model integrations, too much maintenance',
        description:
          'Different model APIs and SDKs make each individual integration slow and costly.',
      },
      {
        title: 'High token costs from fragmented procurement',
        description:
          'Buying from individual providers limits pricing leverage, especially for high-frequency Agent workloads.',
      },
      {
        title: 'Accounts, budgets, and usage are hard to control',
        description:
          'Separate top-ups and consoles leave teams without a unified view of spend and budgets.',
      },
      {
        title: 'Single-route dependency puts reliability at risk',
        description:
          'Model outages and rate limits can directly interrupt business-critical workloads.',
      },
      {
        title: 'Model selection has a steep learning curve',
        description:
          'Complex sign-up, top-up, and testing flows make experimentation slower than it should be.',
      },
      {
        title: 'Compliance risk leaves overseas model access uncertain',
        description:
          'Business workloads need compliant, auditable access to model providers.',
      },
    ],
  },
  {
    label: 'The capabilities you need, in one platform',
    items: [
      {
        title: 'Unified API gateway for multiple models',
        description:
          'Use one standardized API to connect mainstream model providers once and call them everywhere.',
      },
      {
        title: 'Intelligent traffic orchestration',
        description:
          'Route requests by scenario and automatically fail over when a provider is unavailable.',
      },
      {
        title: 'Visual usage and budget management',
        description:
          'Monitor usage in real time, analyze costs, and set balance and budget alerts.',
      },
      {
        title: 'Enterprise-grade access controls',
        description:
          'Manage roles and departments with clear, tiered permissions.',
      },
      {
        title: 'High concurrency with low latency',
        description:
          'Keep business workloads responsive under demanding production traffic.',
      },
      {
        title: '24/7 technical support',
        description:
          'Get onboarding guidance, troubleshooting, and architecture advice when you need it.',
      },
    ],
  },
] as const

const audience = [
  {
    title: 'Independent builders',
    description:
      'Try a broad range of models at low cost and validate ideas quickly with usage-based billing.',
    icon: WalletCards,
  },
  {
    title: 'AI Agent developers',
    description:
      'Work with mainstream Agent frameworks while controlling high-frequency calling costs and reliability.',
    icon: Workflow,
  },
  {
    title: 'Enterprise teams',
    description:
      'Move from model integration to cost control and compliance with a single operational foundation.',
    icon: Building2,
  },
] as const

const icons = [Layers3, Gauge, ShieldCheck, Workflow, WalletCards, Building2]

export function WebsiteDetails() {
  const { t } = useTranslation()

  return (
    <>
      {sections.map((section, sectionIndex) => (
        <section
          key={section.label}
          className={
            sectionIndex === 0
              ? 'bg-muted/30 px-6 py-24 md:py-32'
              : 'px-6 py-24 md:py-32'
          }
        >
          <div className='mx-auto max-w-6xl'>
            <h2 className='mx-auto mb-12 max-w-2xl text-center text-3xl font-bold tracking-tight md:text-4xl'>
              {t(section.label)}
            </h2>
            <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-3'>
              {section.items.map((item, index) => {
                const Icon = icons[index]

                return (
                  <article key={item.title} className='rounded-2xl border bg-background p-6'>
                    <Icon className='text-primary size-5' />
                    <h3 className='mt-6 font-semibold'>{t(item.title)}</h3>
                    <p className='text-muted-foreground mt-2 text-sm leading-6'>
                      {t(item.description)}
                    </p>
                  </article>
                )
              })}
            </div>
          </div>
        </section>
      ))}

      <section className='bg-primary text-primary-foreground px-6 py-24'>
        <div className='mx-auto max-w-6xl'>
          <h2 className='mb-12 text-center text-3xl font-bold'>
            {t('A solution for every AI builder')}
          </h2>
          <div className='grid gap-4 md:grid-cols-3'>
            {audience.map((item) => {
              const Icon = item.icon

              return (
                <article
                  key={item.title}
                  className='rounded-2xl border border-white/15 bg-white/10 p-6'
                >
                  <Icon className='size-6' />
                  <h3 className='mt-6 text-lg font-semibold'>{t(item.title)}</h3>
                  <p className='mt-2 text-sm leading-6 opacity-80'>
                    {t(item.description)}
                  </p>
                </article>
              )
            })}
          </div>
        </div>
      </section>
    </>
  )
}
