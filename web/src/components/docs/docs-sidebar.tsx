import { BookOpen, Zap, Key, Coins, Wallet, FileText, ListTodo, BarChart3, CreditCard, MessageSquare, Settings, HelpCircle, Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation } from '@tanstack/react-router'
import { cn } from '@/lib/utils'

interface NavItem {
  title: string
  href?: string
  icon?: React.ElementType
  children?: NavItem[]
}

export function DocsSidebar() {
  const { t } = useTranslation()
  const location = useLocation()

  const navItems: NavItem[] = [
    { title: t('Quick Start'), href: '/docs/quick-start', icon: Zap },
    { title: t('Installation'), href: '/docs/installation', icon: Download },
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
    { title: t('FAQ'), href: '/docs/faq', icon: HelpCircle },
  ]

  return (
    <nav className="flex flex-col gap-1">
      {navItems.map((item) => (
        <div key={item.title}>
          {item.href ? (
            <Link
              to={item.href}
              className={cn(
                'flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors hover:bg-accent hover:text-accent-foreground',
                location.pathname === item.href
                  ? 'bg-accent text-accent-foreground font-medium'
                  : 'text-muted-foreground'
              )}
            >
              {item.icon && <item.icon className="h-4 w-4 shrink-0" />}
              <span>{item.title}</span>
            </Link>
          ) : (
            <div className="px-3 py-1.5 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              {item.title}
            </div>
          )}
          {item.children && (
            <div className="ml-3 mt-0.5 flex flex-col gap-0.5 border-l pl-3">
              {item.children.map((child) => (
                <Link
                  key={child.href}
                  to={child.href!}
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
              ))}
            </div>
          )}
        </div>
      ))}
    </nav>
  )
}