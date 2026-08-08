# B1 设计：渠道密钥生命周期（P0-16）

日期：2026-08-07
状态：用户已批准（方案 A）
依据：`docs/website/需求实现任务清单.md` P0-16；`docs/website/AI_API中转平台_需求验收与整改报告.md` R5

## 一、整体上下文：P0 补全工程（19 项，7 批次）

依赖优先分批开发，每批独立 spec → 实施计划 → 实施 → docker 验证：

| 批次 | 子项目 |
| --- | --- |
| B1 密钥安全 | P0-16 渠道密钥加密存储/轮换/到期提醒 |
| B2 预算与限流 | P0-05 Key 预算与限制、P0-11 分层限流与并发 |
| B3 价格版本 | P0-28 |
| B4 网关能力 | P0-09 异步回调、P0-10 模型别名、P0-12 错误码契约、P0-13 适配器能力声明、P0-14 多维路由、P0-15 路由轨迹 |
| B5 用户门户 | P0-01 首页状态、P0-02 企业开户、P0-03 手机号登录、P0-04 控制台、P0-06 项目管理 UI |
| B6 经营与运维 | P0-27 经营看板、P0-29 监控告警、P0-30 备份恢复、P0-31 数据保留 |
| B7 收尾 | P0-20 销售导出按钮、P0-26 审计前后值补全 |

本文档为 B1 的设计。

## 二、需求与验收要点（P0-16）

使用可轮换主密钥加密落库；迁移旧值；展示脱敏；到期和轮换提醒；绝不写日志。

## 三、现状（代码证据）

- `model/channel.go:26` Key 明文落库；多 key 模式以 `\n` 分隔存同一字段（`GetKeys()` :175-196、`GetNextEnabledKey()` :199+）。
- 明文消费点：`middleware/distributor.go:498`、`relay/relay_task.go:105`、`relay/mjproxy_handler.go:351,531`、`controller/channel_upstream_update.go:342,353,399,429`、`controller/channel-billing.go`（多处直接 `channel.Key`）、`controller/channel-test.go`、`controller/video_proxy.go:116`、`controller/video_proxy_gemini.go:217,224`、`controller/ratio_sync.go:268`、`service/codex_channel_models.go:49`、`model/channel_cache.go:84`。
- 列表/详情接口 `Omit("key")`（controller/channel.go:140,159）；`GetChannelKey` 端点需 SecureVerificationRequired + 审计（controller/channel.go:419-454），审计只记 id/name。
- relaykit 模块不读取 Channel.Key（已核查），加密仅影响 root 模块。
- 既有主密钥模式：`common/constants.go:35-36` SessionSecret/CryptoSecret；`common/init.go:57-63` 从文件/环境变量初始化并持久化 SessionSecret。
- 隐患：`controller/channel-billing.go:186` 将密钥拼进 URL（`?api_key=%s`），需确认不落日志。

## 四、方案选择

- **方案 A（选定）**：GORM 钩子透明加解密 + 前缀标记。所有消费方（GetNextEnabledKey、多 key 拆分、existing.Key 对比、direct 读取）拿到即明文，零调用方改动。
- 方案 B：读写入口集中加解密——10+ 处直接消费点易漏，不选。
- 方案 C：数据库层 TDE——SQLite/MySQL/PG 无统一机制，排除。

## 五、设计细节

### 5.1 密文格式与加解密入口

新增 `common/key_encrypt.go`：

- `EncryptChannelKey(plain string) (string, error)`：AES-256-GCM，nonce 12 字节随机，每次加密重新生成；输出 `enc:v1:<base64(nonce||ciphertext||tag)>`；空串原样返回。
- `DecryptChannelKey(stored string) (string, error)`：识别 `enc:v1:` 前缀；无前缀视为旧明文原样返回（兼容）；有前缀但解密失败返回错误，由调用方记 SysError 并 fail-closed，**绝不静默返回空串**。

### 5.2 主密钥管理

- 环境变量 `CHANNEL_KEY_MASTER_KEY`（**恰好 32 字节**；非空但长度不符时告警并回退——2026-08-08 修订，原「≥32 字节」会放行 33-40 字节值导致 aes.NewCipher 运行时失败）。
- 回退链：`CHANNEL_KEY_MASTER_KEY` → 数据目录持久化文件 `channel-key-master.key`（首次启动生成 32 随机字节，0600 权限；容器数据卷 `/data` 下，重启/重建容器均保留）→ `sha256(CRYPTO_SECRET||SESSION_SECRET)` 派生兜底（此路径启动时 SysError 告警）。
- 2026-08-07 修订：原设计「SessionSecret 已在 common/init.go 持久化」经核查为错误假设（SessionSecret 仅从 env 读取，默认部署每次重启随机，将导致迁移密文跨重启不可解密）。改为主密钥独立文件持久化，不改变既有 SessionSecret 行为。

### 5.3 GORM 钩子（model/channel.go）

- `BeforeSave`/`BeforeUpdate`：Key 非空且无 `enc:v1:` 前缀 → 加密。
- `AfterFind`：带前缀 → 解密；空串跳过（`Omit("key")` 列表查询不受影响）；解密失败保留密文原值并 SysError，请求路径 `GetNextEnabledKey` 因格式错误 fail-closed（返回 NoAvailableKey）。

### 5.4 存量迁移

- 启动例行 goroutine：扫描 Key 非空且无前缀的渠道，逐条加密落库，完成后 SysError 报告迁移数量。
- 钩子保证任何一次渠道更新也自动迁移单条。

### 5.5 多 key 模式

整体加密（`\n` 分隔整串一起加密）；MultiKeyStatusList 按 index 的状态管理不变；编辑/删除单 key 路径（controller/channel.go:1007-1080、:1870、:1938）读解密后明文再 join → 钩子重新加密。

### 5.6 到期提醒

- `Channel` 新增 `KeyExpiresAt *time.Time`（可空、无 default 标签，三库 AutoMigrate 兼容）。
- 前端渠道表单加「密钥到期时间」日期输入。
- 新增系统任务 `channel_key_expiry`，配置（参照 business_reminder 风格）：
  - `CHANNEL_KEY_EXPIRY_TASK_ENABLED`（默认 true）
  - `CHANNEL_KEY_EXPIRY_TASK_INTERVAL_MINUTES`（默认 360，范围 60–10080）
  - `CHANNEL_KEY_EXPIRY_ADVANCE_DAYS`（默认 7，范围 1–90）
  - `CHANNEL_KEY_EXPIRY_NOTIFY_ENABLED`（默认 false，与 BUSINESS_REMINDER_NOTIFY_ENABLED 一致）
- 命中提醒走 `NotifyRootUser`（复用 service/channel.go AutoBan 通知 root 的先例）+ 系统日志；同一渠道同一提醒窗口只提醒一次（按时间窗口去重）。

### 5.7 轮换

- 系统任务 `channel_key_re_encrypt`：手动触发（RootAuth 端点），用**当前**主密钥重写全部渠道密文。
- 轮换流程写入运维文档：改环境变量 → 重启 → 执行重加密任务 → 抽样验证；强调先重加密再换密钥（旧密钥读取失败 fail-closed）。

### 5.8 日志加固

- 排查 `channel-billing.go:186` 等把密钥拼进 URL 的位置：改为认证头请求，或确认 URL 不落日志、响应不含 key。
- `GetChannelKey` 审计只记 id/name（保留）。
- 加密工具自身日志只记渠道 id 与错误摘要，不记明文。

### 5.9 前端

- 渠道表单：密钥到期时间输入（日期选择，可空）。
- 渠道列表：不做到期状态列（最小化）；到期时间在渠道表单与详情中展示。

## 六、测试计划

1. common 单元测试：加解密往返、前缀识别、错误密钥解密失败、空串。
2. 模型钩子测试：Create 后 DB 中是密文、AfterFind 得到明文、旧明文兼容、解密失败 fail-closed。
3. 多 key 集成测试：创建多 key 渠道 → 缓存加载 → `GetNextEnabledKey` 轮询解密正确。
4. 迁移任务测试：存量明文 → 密文且幂等。
5. 到期任务测试：造数据触发提醒、去重、通知开关。
6. docker 验证：建渠道（含到期时间）→ psql 查库确认密文 → 真实 relay 请求走通 → 触发重加密任务再验证 → 日志无明文。

## 七、docker 验证步骤

1. `docker compose -f docker-compose.dev.yml up -d --build new-api`。
2. 登录 admin 建渠道（含到期时间）；psql 查 `channels.key` 为 `enc:v1:` 前缀。
3. 用 OpenAI 兼容模型发真实请求，验证解密链路与计费正常。
4. 触发 `channel_key_re_encrypt` 任务，抽样验证渠道可用。
5. 检查日志无明文 key；`GetChannelKey` 仍需二次验证 + 审计。

## 八、风险与迁移影响

- 无破坏性 schema 变更（仅新增可空字段 `key_expires_at`），三库 GORM AutoMigrate。
- 主密钥丢失 = 密钥不可恢复：文档与启动告警强调备份（与 B6 P0-30 备份恢复衔接）。
- 缓存中的渠道对象为解密后明文（与现状等价），仅存内存，随缓存淘汰释放。
