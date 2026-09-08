# 文档站集成实施计划（方案 C 修正版：Rsbuild + MDX）

日期：2026-08-20
目标：将 `new-api-docs-v1` 用户文档集成到项目 `web/` 中，仅普通用户内容，构建/部署与主应用统一

## 一、技术方案

| 项 | 选型 |
| --- | --- |
| 构建 | 现有 Rsbuild + `@rsbuild/plugin-mdx`（新增） |
| MDX 渲染 | `@mdx-js/react` 3.1.1（已安装） |
| 路由 | TanStack Router（现有） |
| 样式 | Tailwind CSS（现有） |
| API 参考 | `redoc` 组件（OpenAPI JSON 驱动） |
| 内容源 | 从 `new-api-docs-v1` 迁移 `user/` 目录 MDX + 安装/部署/FAQ |

## 二、目录结构

```
web/src/
├── routes/_authenticated/docs/
│   ├── route.tsx                    # docs 布局（侧边栏 + 内容区）
│   ├── index.tsx                    # /docs → 重定向到 /docs/quick-start
│   ├── quick-start.tsx              # 快速开始（导入 MDX）
│   ├── api-reference.tsx            # API 参考（redoc 驱动）
│   ├── installation.tsx             # 安装部署（导入 MDX）
│   ├── guide/
│   │   ├── api-keys.tsx             # API 密钥管理
│   │   ├── tokens.tsx               # Token 额度
│   │   ├── topup.tsx                # 充值
│   │   ├── usage-logs.tsx           # 使用日志
│   │   ├── tasks.tsx                # 任务管理
│   │   ├── pricing.tsx              # 模型定价
│   │   ├── subscription.tsx         # 用户订阅
│   │   ├── chat-apps.tsx            # 聊天应用
│   │   └── personal-settings.tsx    # 个人设置
│   └── faq.tsx                      # 常见问题
├── content/docs/zh/
│   ├── quick-start.mdx              # 快速开始内容
│   ├── installation.mdx             # 安装部署内容
│   ├── guide/
│   │   ├── api-keys.mdx
│   │   ├── tokens.mdx
│   │   ├── topup.mdx
│   │   ├── usage-logs.mdx
│   │   ├── tasks.mdx
│   │   ├── pricing.mdx
│   │   ├── subscription.mdx
│   │   ├── chat-apps.mdx
│   │   └── personal-settings.mdx
│   └── faq.mdx
└── components/docs/
    ├── docs-sidebar.tsx              # 侧边栏导航（仅用户页面）
    └── docs-mdx-provider.tsx         # MDX 组件包装器
```

## 三、任务分解

### Task 1: 安装依赖 + Rsbuild 配置

**新增依赖**：`@rsbuild/plugin-mdx`、`redoc`

```bash
cd web && bun add @rsbuild/plugin-mdx redoc
```

`rsbuild.config.ts` 加插件：

```ts
import { pluginMdx } from '@rsbuild/plugin-mdx'

// plugins 数组加 pluginMdx()
plugins: [pluginReact(), pluginMdx(), pluginTailwindcss({ optimize: false })],
```

### Task 2: MDX 内容文件（从 new-api-docs-v1 迁移）

**范围**：仅以下目录/文件
- `content/docs/zh/guide/feature-guide/user/` 全部 9 个文件
- `content/docs/zh/installation/` 部署相关
- `content/docs/zh/guide/wiki/basic-concepts/` 基础概念
- `content/docs/zh/support/faq.mdx`（如有）

**处理**：MDX 中的图片/链接路径修正为项目内路径，`content/docs/zh/` → `content/docs/zh/`

### Task 3: 侧边栏导航组件

```tsx
// web/src/components/docs/docs-sidebar.tsx
const NAV_ITEMS = [
  { title: '快速开始', href: '/docs/quick-start' },
  { title: '安装部署', href: '/docs/installation' },
  {
    title: '功能指南',
    children: [
      { title: 'API 密钥', href: '/docs/guide/api-keys' },
      { title: 'Token 额度', href: '/docs/guide/tokens' },
      { title: '充值', href: '/docs/guide/topup' },
      { title: '使用日志', href: '/docs/guide/usage-logs' },
      { title: '任务管理', href: '/docs/guide/tasks' },
      { title: '模型定价', href: '/docs/guide/pricing' },
      { title: '订阅', href: '/docs/guide/subscription' },
      { title: '聊天应用', href: '/docs/guide/chat-apps' },
      { title: '个人设置', href: '/docs/guide/personal-settings' },
    ],
  },
  { title: 'API 参考', href: '/docs/api-reference' },
  { title: '常见问题', href: '/docs/faq' },
]
```

侧边栏样式：复用现有 `SectionPageLayout` 侧边栏模式，左侧固定 240px，右侧内容区。

### Task 4: 路由文件

每个路由文件是 ~20 行的薄壳，导入 MDX 内容并渲染：

```tsx
// web/src/routes/_authenticated/docs/guide/api-keys.tsx
import { createFileRoute } from '@tanstack/react-router'
import ApiKeys from '@/content/docs/zh/guide/api-keys.mdx'

export const Route = createFileRoute('/_authenticated/docs/guide/api-keys')({
  component: () => <ApiKeys />,
})
```

docs 布局路由 `route.tsx`：

```tsx
// web/src/routes/_authenticated/docs/route.tsx
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { DocsSidebar } from '@/components/docs/docs-sidebar'

export const Route = createFileRoute('/_authenticated/docs')({
  component: () => (
    <div className="flex h-full">
      <aside className="w-60 shrink-0 border-r p-4 overflow-auto">
        <DocsSidebar />
      </aside>
      <main className="flex-1 overflow-auto p-6 max-w-4xl">
        <article className="prose prose-slate dark:prose-invert max-w-none">
          <Outlet />
        </article>
      </main>
    </div>
  ),
})
```

### Task 5: API 参考页（redoc）

```tsx
// web/src/routes/_authenticated/docs/api-reference.tsx
import { createFileRoute } from '@tanstack/react-router'
import { RedocStandalone } from 'redoc'

export const Route = createFileRoute('/_authenticated/docs/api-reference')({
  component: () => (
    <RedocStandalone
      specUrl="/openapi/api.json"
      options={{
        nativeScrollbars: true,
        theme: { colors: { primary: { main: '#3b82f6' } } },
      }}
    />
  ),
})
```

`api.json` 从 `docs/openapi/api.json` 复制到 `web/public/openapi/api.json`（或 rsbuild 配 `publicDir`）。

### Task 6: 侧边栏入口

`use-sidebar-data.ts` 的 `General` 分组加一个入口：

```ts
{
  title: t('Documentation'),
  url: '/docs/quick-start',
  icon: BookOpen,
},
```

### Task 7: i18n + 构建验证

- 文档内容以中文为主（MDX 内容来自 `zh/` 目录），英文版后续补充
- `tsgo -b` 类型检查；`rsbuild build` 构建验证
- Docker 验证：`docker compose -f docker-compose.dev.yml up -d --build new-api`，访问 `/docs/quick-start`

## 四、工作量

| 任务 | 耗时 |
| --- | --- |
| 1. 依赖 + Rsbuild 配置 | 0.5h |
| 2. MDX 内容迁移 | 2h |
| 3. 侧边栏组件 | 1h |
| 4. 路由文件 | 1h |
| 5. API 参考 | 0.5h |
| 6. 侧边栏入口 | 0.5h |
| 7. 验证 + 修复 | 1h |
| **总计** | **6-7h** |

## 五、风险

- `@rsbuild/plugin-mdx` 与 Rsbuild 2 兼容性（需验证版本匹配）
- redoc 包体积较大（~1MB），考虑按需加载（`React.lazy`）或换用更轻量的 `swagger-ui-react`
- MDX 文件中的图片/链接跨仓库迁移需要路径修正