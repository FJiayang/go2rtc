# Xiaomi 账号自愈设计（serviceToken / passToken / UI 状态）

日期：2026-04-25
作者：mujing
状态：已批准，待实现

## 背景

go2rtc 的小米云对接存在三个相互关联的缺陷，参考第三方项目 `hass-xiaomi-miot`（`core/xiaomi_cloud.py`）后明确如下：

1. **serviceToken 运行期过期后无自愈**。`pkg/xiaomi/cloud.go:465-467` 的 `Request` 仅把 `code != 0` 包成 `errors.New("xiaomi: " + message)` 抛出，`internal/xiaomi.cloudRequest` 不做识别、也不重试。`hass-xiaomi-miot` 的做法是识别 `code ∈ {2,3}` 或 message 含 `auth err` / `invalid signature` / `SERVICETOKEN_EXPIRED` 后自动重登。
2. **UI 重登后 `clouds[userID]` 不更新**。`internal/xiaomi/xiaomi.go:326-339` 成功分支里 `auth = nil` 丢弃了刚拿到有效会话的 `Cloud` 实例，而 `clouds[userID]` 仍引用旧的过期实例 —— 导致当前必须"删 yml + 重启"才能恢复。
3. **passToken 被风控吊销后无恢复路径**。yml 只保存 `passToken`，`apiAuth` 成功后密码即弃，后续无法重走完整 `Login`。

## 目标与非目标

**目标**
- serviceToken 过期时在当前请求路径上自动自愈，对上层透明。
- UI 重新登录后无需重启即可生效。
- passToken 被吊销时，只要此前 UI 登录过一次、且 yml 与本机匹配，即可无人干预恢复。
- 对 upstream 影响面最小，便于后续同步 upstream 变更。

**非目标**
- 不做主动探测心跳（hass-miot 的 `async_check_auth` 不在本次范围）。
- 不做跨机器可迁移的凭据存储 —— 明确要求加密密码只在本机可解密。
- 不做 `schema.json` 更新。
- 不改 `pkg/xiaomi.Cloud` 的公开签名。

## 决策纪要

- **重登凭据**：`passToken` + **本机加密**的密码。密钥派生自 `MAC + hostname`，做到跨机器不可用。
- **触发时机**：惰性 —— `cloud.Request` 命中过期特征时由 `cloudRequest` 包一层重登 + 单次重发。
- **需要验证码/短信时**：放弃重试，原错透传，用户回到 UI 完成。
- **UI 登录后缓存**：`clouds[userID] = auth`，复用该会话。
- **yml 结构**：保留旧 `xiaomi:` 不动，仅新增 `xiaomi_accounts:`，不做值形态迁移。
- **去重**：`refreshCloud` 用 5 秒时间窗粗略去重，不引入 `singleflight` 依赖。

## 组件

### 新增

- `pkg/xiaomi/errors.go`
  - `var ErrTokenExpired = errors.New("xiaomi: token expired")`
- `pkg/xiaomi/secret.go`
  - `Encrypt(plaintext string) (string, error)`
  - `Decrypt(ciphertext string) (string, error)`
  - `deriveKey()`：取第一块非 loopback、MAC 非全零的 `net.Interface.HardwareAddr`，拼 `os.Hostname()` 做 info 材料，HKDF-SHA256 → 32 字节；完全无网卡时 fallback 仅 hostname。
  - AES-256-GCM，nonce 12 B 随机前缀，整体 base64 输出。
- `internal/xiaomi/relogin.go`
  - `relogin(userID string) (*xiaomi.Cloud, error)`：先 `LoginWithToken`；失败且有 `xiaomi_accounts[userID]` 则 `Login(username, Decrypt(enc_password))`。命中 `*xiaomi.LoginError` 原样返回。

### 修改

- `pkg/xiaomi/cloud.go`
  - `Request` 中 `res1.Code != 0` 命中 `isTokenExpired(code, message)` 时返回 `fmt.Errorf("xiaomi: %s: %w", message, ErrTokenExpired)`；其它 `code != 0` 保持现状。
  - 私有函数 `isTokenExpired(code int, message string) bool` —— 表驱动：`code ∈ {2,3}` 或 message 含 `auth err` / `invalid signature` / `SERVICETOKEN_EXPIRED`。
- `internal/xiaomi/xiaomi.go`
  - `Init`：同时读 `xiaomi` 和 `xiaomi_accounts`。新增包级 `var accounts map[string]Account`，类型 `Account{ Username, EncPassword string }`（yaml tag `username` / `enc_password`）。
  - `cloudRequest`：`errors.Is(err, xiaomi.ErrTokenExpired)` → `refreshCloud(userID)` → 单次重发原请求。
  - `refreshCloud(userID string) (*xiaomi.Cloud, error)`：进入时 `cloudsMu.Lock`，若 `map[userID]time.Time lastRefresh` 记录距今 < 5 s，直接复用当前 `clouds[userID]`；否则调 `relogin(userID)`，成功后更新 `clouds` / `tokens` / `PatchConfig(["xiaomi", userID], newPassToken)`。
  - `apiAuth` 成功分支：
    1. `cloudsMu.Lock` 下 `clouds[userID] = auth`
    2. `tokens[userID] = token`
    3. `accounts[userID] = Account{Username: username, EncPassword: Encrypt(password)}`（Encrypt 失败仅记日志、不阻塞）
    4. `PatchConfig(["xiaomi", userID], token)`
    5. `PatchConfig(["xiaomi_accounts", userID], map[string]string{...})`
    6. `auth = nil`

### 不动

- `www/add.html`、所有对外 API 形状、`schema.json`。

## 数据流

### 常规请求

```
cloudUserRequest → getCloud 命中 clouds → cloud.Request → 返回
```

### serviceToken 过期

```
cloud.Request  →  code∈{2,3} 或关键字命中  →  wrap ErrTokenExpired
        ↓
cloudRequest 捕获 → refreshCloud(userID)
        ├─ 近 5s 已刷 → 复用当前 cloud
        └─ 否则 relogin(userID)
              ├─ LoginWithToken(tokens[userID])
              │     ok → 更新 clouds/tokens/yaml
              └─ 失败 & accounts[userID] 存在
                    Login(username, Decrypt(enc_password))
                    ok → 更新 clouds/tokens/yaml
                    err(*LoginError) → 透传
                    err（其他）     → 透传
        ↓
cloudRequest 用新 cloud 重发 1 次（仅 1 次）
```

### UI 登录

```
apiAuth Login/LoginWithCaptcha/LoginWithVerify 成功
    → clouds[userID] = auth
    → tokens[userID]  = passToken
    → accounts[userID] = {username, Encrypt(password)}
    → PatchConfig(xiaomi.<userID>, passToken)
    → PatchConfig(xiaomi_accounts.<userID>, {username, enc_password})
    → auth = nil
```

## 错误处理

| 场景 | 行为 |
|---|---|
| `cloud.Request` 网络错误 | 原样返回，不重登 |
| `cloud.Request` code∈{2,3} / 关键字 | wrap `ErrTokenExpired` |
| `cloud.Request` 其他 code != 0 | 原样返回，不重登 |
| relogin 命中 `*LoginError`（验证码/短信） | 透传，不重发原请求；用户应从 UI 完成 |
| relogin 的 LoginWithToken 失败 + 无 `accounts[userID]` | 透传 LoginWithToken 错；`warn` 日志 `no stored password, UI relogin required` |
| `Decrypt` 失败（跨机器迁移 / MAC 变更） | 视作无 enc_password，走 UI 路径；`warn` 日志 |
| `Encrypt` 失败（派生密钥失败） | UI 登录时记 `error` 日志，仍保存 passToken，跳过 enc_password；不阻塞登录成功 |
| `PatchConfig` 写盘失败 | 不回滚内存；`error` 日志；下次重启回到旧 token，relogin 会再补 |

日志级别：
- relogin 成功：`info`
- relogin 失败：`warn`
- `ErrTokenExpired` 命中：`debug`

## 并发

`cloudsMu` 覆盖 `getCloud` / `refreshCloud` / `apiAuth` 的 map 写入。`refreshCloud` 在持锁时比对 `lastRefresh[userID]`，< 5 s 直接复用，避免同一 userID 在一批并发请求里重复 `Login`。

## 测试

### 单元

`pkg/xiaomi/cloud_test.go`
- `isTokenExpired` 表驱动：code=2/3、三类关键字 message → true；其他 → false。
- `Request` 用 `httptest` 模拟加密响应（构造本地 `ssecurity`，RC4 加密 `{code:3,message:"auth err"}`），断言 `errors.Is(err, ErrTokenExpired)`。

`pkg/xiaomi/secret_test.go`
- `Encrypt` / `Decrypt` 回环。
- 同进程内 `deriveKey` 多次调用结果一致。
- 注入不同 hostname/MAC 后 Decrypt 失败。

### 集成

`internal/xiaomi/relogin_test.go`
- LoginWithToken 成功 → clouds/tokens 更新。
- LoginWithToken 失败 + Login 成功 → 同上，accounts 不变。
- LoginWithToken 失败 + 无 enc_password → 返回错。
- relogin 返回 `*LoginError` → 透传。
- 20 个并发请求同时触发 `ErrTokenExpired`，实际 Login 调用次数为 1（5 s 去重正确）。

### 手工冒烟

1. 仅带旧 `xiaomi: {uid: token}` 启动 → 正常拉流 → 人工吊销 passToken → 下一次拉流收到 UI 登录错误（因无 enc_password）。
2. 过 UI 登录一次 → yml 出现 `xiaomi_accounts` → 人工吊销 passToken → 下一次拉流自动恢复，yml 中 `xiaomi.<userID>` passToken 已更新。
3. 把 yml 拷到另一台机器启动 → 首次请求发现 `ErrTokenExpired`，解密 enc_password 失败 → 回到 UI 登录路径。

## 兼容与回滚

- 旧用户 yml 中仅有 `xiaomi:` 键：行为保持不变；自愈能力在用户下次 UI 登录后自动启用。
- 本次改动只新增 yaml 键，不改 schema.json；回滚只需 `git revert` 对应提交，yml 中多出的 `xiaomi_accounts` 键会被忽略。
- 升级 / 同步 upstream 时冲突面集中在 `internal/xiaomi/xiaomi.go` 的 `Init` 与 `apiAuth` 两处。
