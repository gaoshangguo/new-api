import { BarChart3, BookOpen, Coins, CreditCard, ExternalLink, FileText, HelpCircle, Key, ListTodo, MessageSquare, Settings, Wallet, Zap } from 'lucide-react'
import { Link, useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

interface NavItem {
  title: string
  href?: string
  icon?: React.ElementType
  children?: NavItem[]
  external?: boolean
}

export function DocsSidebar() {
  const { t } = useTranslation()
  const location = useLocation()

  const navItems: NavItem[] = [
    { title: t('Quick Start'), href: '/docs/quick-start', icon: Zap },
    {
      title: t('Feature Guide'),
      children: [
        { title: t('API Keys'), href: '/docs/guide/api-keys', icon: Key },
        { title: t('Tokens & Quota'), href: '/docs/guide/tokens', icon: Coins },
        { title: t('Top Up'), href: '/docs/guide/topup', icon: Wallet },
        { title: t('Usage Logs'), href: '/docs/guide/usage-logs', icon: FileText },
        { title: t('Task Management'), href: '/docs/guide/tasks', icon: ListTodo },
        { title: t('Model Pricing'), href: '/docs/guide/pricing', icon: BarChart3 },
        { title: t('Subscription'), href: '/docs/guide/subscription', icon: CreditCard },
        { title: t('Chat Apps'), href: '/docs/guide/chat-apps', icon: MessageSquare },
        { title: t('Personal Settings'), href: '/docs/guide/personal-settings', icon: Settings },
      ],
    },
    { title: t('API Reference'), href: '/docs/api-reference', icon: BookOpen },
    {
      title: t('API Quick Integration'),
      href: 'https://app.apifox.com/main/teams/4381043?tab=project',
      icon: ExternalLink,
      external: true,
    },
    { title: t('FAQ'), href: '/docs/faq', icon: HelpCircle },
  ]

  return (
    <nav className="docs-guide-nav flex flex-col gap-1">
      {navItems.map((item) => {
        const hasActiveChild =
          item.children?.some((child) => location.pathname === child.href) ?? false
        const isActive = item.href ? location.pathname === item.href : false
        let heading: React.ReactNode = null
        if (item.external && item.href) {
          heading = (
            <a
              href={item.href}
              target="_blank"
              rel="noopener noreferrer"
              className={cn(
                'flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors hover:bg-accent hover:text-accent-foreground',
                'text-muted-foreground'
              )}
            >
              {item.icon && <item.icon className="h-4 w-4 shrink-0" />}
              <span>{item.title}</span>
            </a>
          )
        } else if (item.href) {
          heading = (
            <Link
              to={item.href}
              className={cn(
                'flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors hover:bg-accent hover:text-accent-foreground',
                isActive
                  ? 'bg-accent text-accent-foreground font-medium'
                  : 'text-muted-foreground'
              )}
            >
              {item.icon && <item.icon className="h-4 w-4 shrink-0" />}
              <span>{item.title}</span>
            </Link>
          )
        } else {
          heading = (
            <div
              className={cn(
                'px-3 py-1.5 text-xs font-semibold uppercase tracking-wider',
                hasActiveChild ? 'text-foreground' : 'text-muted-foreground'
              )}
            >
              {item.title}
            </div>
          )
        }
        return (
          <div key={item.title}>
            {heading}
            {item.children && (
              <div className="ml-3 mt-0.5 flex flex-col gap-0.5 border-l pl-3">
                {item.children.map((child) => {
                  if (!child.href) return null
                  return (
                    <Link
                      key={child.href}
                      to={child.href}
                      className={cn(
                        'flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors hover:bg-accent hover:text-accent-foreground',
                        location.pathname === child.href
                          ? 'bg-accent text-accent-foreground font-medium'
                          : 'text-muted-foreground'
                      )}
                    >
                      {child.icon && <child.icon className="h-3.5 w-3.5 shrink-0" />}
                      <span>{child.title}</span>
                    </Link>
                  )
                })}
              </div>
            )}
          </div>
        )
      })}
    </nav>
  )
}