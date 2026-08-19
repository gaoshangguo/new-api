# B2 设计：Key 预算与分层限流（P0-05 + P0-11）

日期：2026-08-08
状态：用户已批准
依据：`docs/website/需求实现任务清单.md` P0-05、P0-11；`AI_API中转平台_功能需求说明书.md`

## 一、整体上下文

P0 补全工程批次：B1 密钥安全（✅ 已完成）→ **B2 预算与限流（本批）** → B3 价格版本 → B4 网关能力 → B5 用户门户 → B6 经营与运维 → B7 收尾。

## 二、需求与验收要点

**P0-05 Key 预算与限制**：日/月/总预算、模型/渠道白名单、IP、并发限制；请求路径强制执行。
**P0-11 分层限流与并发**：企业、用户、项目、Key、模型、渠道的 RPM/TPM/并发；Redis 与降级路径测试。

## 三、现状（代码证据）

- `model/token.go:14-33`：`RemainQuota`/`UsedQuota`/`UnlimitedQuota`/`ExpiredTime`/`ModelLimitsEnabled`/`ModelLimits`/`AllowIps`/`Group`。无日/月预算、无渠道白名单、无并发、无 RPM/TPM。
- 请求路径：`middleware/auth.go:460-474`（IP 白名单）、`middleware/distributor.go:68-86`（模型白名单）、`service/quota.go:398 PreConsumeTokenQuota`（总预算预扣）、`service/billing_session.go:252,327`。
- 限流：`middleware/model-rate-limit.go`（用户级请求数：成功数+总数双限，Redis 固定窗口列表+令牌桶 `common/limiter`，内存降级 :133-165）；`middleware/rate-limit.go`（IP/全局/搜索，Lua 原子固定窗口）。
- 项目级运行时：`model/business_project_runtime.go`（预算预占、模型/渠道白名单 `AllowsChannel`、认证时预算检查），分发过滤 `model/channel_cache.go:220-231 filterChannelsByAllowedIDs`。
- 无任何并发闸门实现；无 TPM；无 Key/模型/渠道级 RPM。

## 四、已确认的设计决策（用户批准）

1. **TPM 深度**：RPM 精确请求数 + TPM 预估（`max_tokens` 或输入估算计入窗口，流式结束不回溯）。
2. **日/月预算窗口**：日历窗口（自然日/月）+ 消费日志聚合 + Redis 缓存（miss 时聚合回填）。
3. **并发闸门**：Redis Lua 原子 INCR/DECR + TTL，Redis 不可用回退进程内 atomic 计数。
4. 模型级限流用全局 option（MVP，per-model 排后）；渠道级限流放 Channel 字段。

## 五、设计细节

### 5.1 数据模型（新字段，三库 AutoMigrate 兼容）

| 实体 | 字段 | 类型/标签 | 语义 |
| --- | --- | --- | --- |
| Token | `DailyQuota` | int64 bigint | 日预算（0=不限） |
| Token | `MonthlyQuota` | int64 bigint | 月预算（0=不限） |
| Token | `ChannelLimits` | *string text | Key 级渠道白名单 JSON `{"channels":[id...]}` |
| Token | `MaxConcurrentRequests` | int | Key 级并发上限（0=不限） |
| Token | `RateLimitRPM` | int | Key 级 RPM（0=不限） |
| Token | `RateLimitTPM` | int64 | Key 级 TPM 预估（0=不限） |
| Channel | `RateLimitRPM` | int | 渠道级 RPM（0=不限） |
| Channel | `RateLimitTPM` | int64 | 渠道级 TPM（0=不限） |
| Company | `RateLimitRPM` | int | 企业级 RPM（0=不限） |
| Company | `RateLimitTPM` | int64 | 企业级 TPM（0=不限） |
| Company | `MaxConcurrentRequests` | int | 企业级并发（0=不限） |
| BusinessProject | `RateLimitRPM` | int | 项目级 RPM（0=不限） |
| BusinessProject | `RateLimitTPM` | int64 | 项目级 TPM（0=不限） |
| BusinessProject | `MaxConcurrentRequests` | int | 项目级并发（0=不限） |

模型级：全局 option `MODEL_RATE_LIMIT_RPM`/`MODEL_RATE_LIMIT_TPM`（0=不限，MVP 统一所有模型）。

### 5.2 日/月预算执行（日历窗口 + 日志聚合 + 缓存）

- 检查点：`PreConsumeTokenQuota` 之前（service/quota.go），与总预算同路径 fail-fast。
- 已用量聚合：`Log` 表按 `token_id = ? AND type = consume AND created_at >= 窗口起点` SUM(quota)。窗口起点：自然日 0 点 / 自然月 1 日 0 点（服务器本地时区）。
- 热路径缓存：Redis 键 `token_daily:<token_id>:<YYYYMMDD>` / `token_monthly:<token_id>:<YYYYMM>`，TTL 25 小时 / 35 天；miss 时聚合回填。Redis 不可用直接聚合（降级，正确性优先）。
- 超限：HTTP 429 + i18n 消息（与既有限流消息风格一致），不消耗总预算。
- 注意：预扣成功但结算退款会改变已用口径——已用量以消费日志为准（consume 类型），与 B1 前既有消费投影口径一致；本批不做回溯修正（预算检查是尽力拦截，事后超额已有 `over_budget_pending_reconciliation` 机制兜底项目级，Key 级超预算仅拦截新请求）。

### 5.3 分层限流（新中间件）

- 新文件 `middleware/model-rate-limit.go` 扩展或新建 `middleware/scope-rate-limit.go`，作用域矩阵：
  - 企业（Company 字段）→ 用户（既有，保留）→ 项目（BusinessProject 字段）→ Key（Token 字段）→ 模型（全局 option）→ 渠道（Channel 字段，分发后检查）
- RPM：复用 `common/limiter`（Redis 固定窗口列表 + 令牌桶，内存降级），键 `rl:rpm:<scope>:<id>`。
- TPM：预估——`max_tokens`（存在时）+ 输入 tokens 估算（`common.CountTokenInput` 或近似 `len(content)/4`），计入 Redis 固定窗口 `rl:tpm:<scope>:<id>`；流式结束不回溯。
- 检查顺序：认证后立即（企业→用户→项目→Key→模型）→ 渠道选择后（distributor 之后、relay 请求前）。
- 渠道级超限：HTTP 429 拒绝（重试换渠道排后）。渠道级限流仅启用 `RateLimitRPM/TPM > 0` 的渠道。
- 配置来源：各实体字段 + 全局 option；全部 0 = 不限（默认行为不变）。

### 5.4 并发闸门（Redis Lua + 内存降级）

- 借-还：请求进入时 `INCR rl:inflight:<scope>:<id>`；若返回值 > limit → `DECR` 回退 + 429；否则继续。请求结束（成功/失败/panic，defer 保证）`DECR` 释放。Lua 脚本原子化（INCR 检查超限时自动 DECR），TTL 12h 兜底防泄漏。
- 降级：Redis 不可用时进程内 `sync/atomic` 计数（单节点语义）。
- 作用域：企业/用户/项目/Key（Channel 级并发排后）。
- 检查位置：与限流同一检查点；Key 级并发在认证后，项目级贴近既有预算预占检查点。
- 注意：并发闸门拒绝的请求不消耗预算、不产生消费日志。

### 5.5 Key 级渠道白名单

- 认证阶段解析 `Token.ChannelLimits`（非法 JSON fail-closed，沿用项目白名单语义）→ context。
- 分发过滤链（`model/channel_cache.go:220-231` 同点）追加 token 级过滤，与项目白名单**叠加**（都配置时取交集）。
- 前端：文本输入 JSON（带校验），沿用项目白名单表单模式。

### 5.6 前端与配置

- `web/src/features/keys`：表单加 6 字段（daily_quota、monthly_quota、channel_limits、max_concurrent_requests、rate_limit_rpm、rate_limit_tpm）+ 列表列（日/月预算显示）。
- `web/src/features/channels`：表单加 RateLimitRPM/TPM（可选显示，后端字段必做）。
- 新 option：`MODEL_RATE_LIMIT_RPM`/`MODEL_RATE_LIMIT_TPM`（0=不限）。
- i18n：新文案进 en（源串）+ zh.json；**本批先手动加 en.json 源串 + zh.json，并运行 i18n:sync 前先确认同步方向（B1 遗留：en.json 必须先有源串，否则 sync 可能裁剪 zh.json）**。

### 5.7 测试计划

1. 日/月预算：窗口边界（当日 0 点/月初 0 点）、超限拒绝、0=不限、缓存回填、Redis 降级直接聚合。
2. 分层限流：各作用域 RPM 精确计数、TPM 预估（max_tokens 与估算两路径）、超限 429、0=不限、内存降级（miniredis）。
3. 并发闸门：借还平衡（正常完成/异常失败/panic）、超限拒绝、TTL 防泄漏、内存降级。
4. 渠道白名单：token 级过滤、与项目白名单叠加取交集、非法配置 fail-closed。
5. docker 验证：配置 Key 日预算与 RPM → 真实请求超限被 429 → 次日窗口重置 → 计费与日志正常；并发配置下并发请求超限。

### 5.8 风险与边界

- 日/月已用量缓存失效窗口：最多延迟 TTL 内的计数，可接受（尽力拦截语义）。
- TPM 预估不准：文档声明估算口径（max_tokens 优先）。
- 渠道级限流 429 不换渠道：文档声明（重试换渠道排后）。
- 并发计数 TTL 12h 防泄漏：正常请求远短于 TTL，异常进程崩溃由 TTL 兜底。
- 三库兼容：全部字段 int64/int/text + bigint 标签，无方言 SQL；聚合查询用 GORM 方法。
- relaykit 独立性：本批不触碰 relaykit/。
