<img src="images/deadpool.png" style="zoom:30%;transform: scale(0.3);" width="35%" height="35%"/>

# DeadpoolPlus — 全球多协议代理池

DeadpoolPlus 是 [Deadpool](https://github.com/thinkoaa/Deadpool) 的增强版，从 **FOFA**、**Hunter**、**Quake** 等网络空间测绘平台自动化采集 **SOCKS5 / HTTP** 代理，经存活检测后汇聚成本地代理池，供 Burp Suite、Proxifier、SwitchyOmega 等工具轮询切换出口 IP。

> 🚀 **核心增强：**
> 1. **多协议支持** — 除 SOCKS5 外，新增 HTTP 代理支持（HTTP CONNECT 隧道），代理池容量大幅提升
> 2. **FOFA 多国查询** — 按国家逐个查询，内置 42 个国家（排除 CN/HK/MO），每条查询可拉取最多 10000 条

---

## 更新日志

**2026-06-07（DeadpoolPlus）**
1. 🆕 多协议支持：上游代理从仅 SOCKS5 扩展到 **SOCKS5 + HTTP**（HTTP CONNECT 隧道）
2. 代理地址格式统一为 `protocol://IP:PORT`（如 `socks5://1.2.3.4:1080`、`http://5.6.7.8:8080`）
3. FOFA 新增 `httpQueryStrings` 配置，自动查询 42 个国家的 HTTP 代理
4. 新增 `parseProxyURL`、`dialViaHTTPConnect`、`dialViaHTTPSConnect` 辅助函数
5. `CheckSocks` → `CheckProxy`，健康检查支持多协议自动分发

**2026-06-04（DeadpoolPlus）**
1. FOFA 支持多查询语句：`queryString` → `queryStrings` 数组，按国家分别查询，绕过单次 10000 条上限
2. 内置全球 184 个国家/地区查询列表（排除 CN/HK/MO），代理采集量从万级跃升至百万级

**2026-04-29（原版）**
1. 添加 GoReleaser 自动发版配置
2. 代理健康检查优化 + 统计信息展示 + 优雅关闭
3. 代理切换策略优化（随机轮询）+ 配置验证功能
4. 修复 4 个 Bug + 日志分级功能
5. 添加 ANSI 颜色输出 + 移除弃用的 rand.Seed + 支持强制退出

**2024-09-15：** 增加周期性任务，定时检测存活、定时从网络空间取代理

**2024-09-12：** Go 1.23，新增 SOCKS5 账号密码认证

---

## 免责声明

本工具仅面向**合法授权**的企业安全建设行为。使用者应确保行为符合当地法律法规并已取得足够授权。非法使用产生的后果由使用者自行承担。

---

## 目录

- [0x01 核心思路](#0x01-核心思路)
- [0x02 效果展示](#0x02-效果展示)
- [0x03 快速开始](#0x03-快速开始)
- [0x04 配置说明](#0x04-配置说明)
- [0x05 GitHub Action 自动化](#0x05-github-action-自动化)
- [0x06 编译多平台二进制](#0x06-编译多平台二进制)

---

## 0x01 核心思路

在攻防过程中 IP 被 ban 是家常便饭。DeadpoolPlus 解决的是一个经典问题：**如何白嫖海量高质量的 SOCKS5 代理，且不花钱。**

### 工作流程

```
┌──────────┐   ┌──────────┐   ┌──────────┐
│  FOFA    │   │  Hunter  │   │  Quake   │    ← 网络空间测绘平台
│ 184国查询│   │ (可选)    │   │ (可选)    │
└────┬─────┘   └────┬─────┘   └────┬─────┘
     │               │               │
     └───────────────┼───────────────┘
                     ▼
         ┌───────────────────────┐
         │  lastData.txt         │   ← 本地已有代理（可手动补充）
         └───────────┬───────────┘
                     ▼
         ┌───────────────────────┐
         │  去重 + 并发存活检测    │   ← 可配并发数/超时/关键字/地理围栏
         └───────────┬───────────┘
                     ▼
         ┌───────────────────────┐
         │  有效代理池            │   ← 写入 lastData.txt 持久化
         └───────────┬───────────┘
                     ▼
         ┌───────────────────────┐
         │  本地 SOCKS5 服务      │   ← 默认 127.0.0.1:10086
         │  随机轮询 + 统计 + 淘汰 │
         └───────────┬───────────┘
                     ▼
            外部工具接入使用
     （Burp / Proxifier / SwitchyOmega / ...）
```

### DeadpoolPlus vs 原版 Deadpool

| 特性 | 原版 Deadpool | DeadpoolPlus |
|---|---|---|
| 上游代理协议 | 仅 SOCKS5 | **SOCKS5 + HTTP**（HTTP CONNECT 隧道） |
| FOFA 查询 | 单条 `queryString` | `queryStrings` + `httpQueryStrings` 双数组 |
| 单次最大采集量 | 10,000 条 | 42 × 10,000 × 2协议 ≈ **840,000 条** |
| 国家覆盖 | 单个国家或 `country!="CN"` | 42 个高产国家逐一查询 |
| 代理统计 | ✅ | ✅ |
| 随机轮询 | ✅ | ✅ |
| 连续失败淘汰 | ✅ | ✅ |
| 优雅关闭 | ✅ | ✅ |
| 地理围栏 | ✅ | ✅ |

---

## 0x02 效果展示

启动后自动从各平台拉取代理并进行存活检测：

![启动](images/new.jpg)

FOFA 多国轮询查询过程：

![FOFA 查询](images/init.png)

监听到请求后随机轮询代理转发：

![轮询](images/polling.png)

验证代理出口 IP：

![出口 IP](images/internetIP.png)

目录爆破场景（IP 被 ban 自动切下一个）：

![爆破](images/test.jpg)

---

## 0x03 快速开始

### 1. 配置 API Key

编辑 `config.toml`，填入网络空间测绘平台的 API Key：

```toml
[FOFA]
switch = 'open'                              # 启用
apiUrl = 'https://fofa.info/api/v1/search/all'
email = 'your@email.com'
key = 'your-fofa-api-key'
queryStrings = [...]                         # 已内置 184 个国家，开箱即用
resultSize = 10000                           # 每条查询最大返回数

[HUNTER]
switch = 'open'
# ... 填入 key

[QUAKE]
switch = 'open'
# ... 填入 key
```

> **至少开启一个平台**即可运行。推荐主要靠 FOFA（覆盖最广），Hunter 和 Quake 作为补充。

### 2. 运行

```bash
# 直接运行（使用默认 config.toml 和 lastData.txt）
./deadpoolplus

# 指定配置文件
./deadpoolplus -c custom_config.toml

# 只使用本地已有代理文件，不从平台拉取
./deadpoolplus -l my_proxies.txt

# 显示帮助
./deadpoolplus -h
```

### 3. 在工具中配置代理

将任意工具的 SOCKS5 代理指向：

```
socks5://127.0.0.1:10086
```

如果配置了用户名密码认证，填上即可。

#### Burp Suite

<img src="images/burp.png" style="zoom: 28%;" width="65%" height="65%"/>

#### Proxifier

<img src="images/Proxifier.png" style="zoom:25%;transform: scale(0.25);" width="35%" height="35%" />

#### SwitchyOmega

<img src="images/SwitchyOmega.png" style="zoom:33%;" width="65%" height="65%" />

### 4. 运行中操作

| 操作 | 功能 |
|---|---|
| 按 `Enter` | 随机切换到下一个代理 IP |
| 按 `s` + `Enter` | 查看代理统计（使用次数/成功率/响应时间/连败） |
| `Ctrl+C` 一次 | 优雅退出：打统计、等活跃连接完成（最多 30s） |
| `Ctrl+C` 两次 | 强制退出 |

---

## 0x04 配置说明

完整配置项参考 `config.toml`：

```toml
[listener]
IP = '127.0.0.1'          # 监听地址
PORT = 10086              # 监听端口
userName = ''             # 认证用户名（空=不认证）
password = ''             # 认证密码
logLevel = 'normal'       # normal: 仅重要信息 / debug: 打印每个请求的代理

[task]
periodicChecking = ''     # cron 表达式，定期检测内存中代理存活，例: 0 */5 * * *
periodicGetSocks = ''     # cron 表达式，定期重新从平台拉取代理，例: 0 6 * * 6

[checkSocks]
checkURL = 'https://www.baidu.com'    # 检测用的 URL
checkRspKeywords = '百度一下'           # 响应中应包含的关键字
maxConcurrentReq = 200                 # 并发检测数（VPS 可设 500-1000）
timeout = 6                            # 超时（秒）
maxFailCount = 3                       # 连续失败 N 次后淘汰

[checkSocks.checkGeolocate]
switch = 'close'                       # 地理围栏开关
checkURL = 'https://qifu-api.baidubce.com/ip/local/geo/v1/district'
excludeKeywords = ['澳门','香港','台湾']
includeKeywords = ['中国']

[FOFA]                                 # ★ DeadpoolPlus 核心增强
switch = 'open'
apiUrl = 'https://fofa.info/api/v1/search/all'
email = 'your@email.com'
key = 'your-key'
queryStrings = [                       # 按国家数组查询，已内置 184 国
    'protocol=="socks5" && country="US" && banner="Method:No Authentication"',
    'protocol=="socks5" && country="JP" && banner="Method:No Authentication"',
    # ... 更多见 config.toml
]
resultSize = 10000                     # 每条最多 10000

[QUAKE]
switch = 'close'
apiUrl = 'https://quake.360.net/api/v3/search/quake_service'
key = 'your-key'
queryString = 'service:socks5 AND country:"CN" AND response:"No authentication"'
resultSize = 500

[HUNTER]
switch = 'close'
apiUrl = 'https://hunter.qianxin.com/openApi/search'
key = 'your-key'
queryString = 'protocol=="socks5"&&protocol.banner="No authentication"&&ip.country="CN"'
resultSize = 200                       # 需为 100 的倍数
```

### 进阶技巧

1. **筛选不拦截恶意 Payload 的代理：** 关闭地理围栏，将 `checkURL` 改为无 WAF 的公网地址，URL 中带测试 Payload，`checkRspKeywords` 设为目标正常返回的字符片段。

2. **针对特定目标筛选：** 将 `checkURL` 设为目标地址，`checkRspKeywords` 设为只有通过代理能访问目标时才会出现的字符串。

---

## 0x05 GitHub Action 自动化

利用 GitHub Action 定时运行 DeadpoolPlus，自动更新 `lastData.txt`。

### 1. Import 仓库

由于 fork 无法修改仓库可见性，需要 **Import** 为本仓库使其变成私有。

![Import 1](./images/import1.png)
![Import 2](./images/import2.png)

> ⚠️ **务必勾选 Private！** 否则 API Key 会随公开仓库泄漏。

### 2. 配置 Action 写入权限

![权限 1](./images/right1.png)
![权限 2](./images/right2.png)

### 3. 添加 Workflow

`.github/workflows/schedule.yml`：

```yaml
name: schedule

on:
  workflow_dispatch:
  schedule:
    - cron: "0 0 */5 * *"

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Check out code
        uses: actions/checkout@v3
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: 1.23.x
          check-latest: true
          cache: true

      - name: Run search
        run: bash entrypoint.sh

      - name: Commit and push if changed
        run: |
          git config --global user.name 'your-name'
          git config --global user.email 'your-email'
          if git diff --quiet -- lastData.txt; then
            echo "lastData.txt has not been modified."
          else
            git add lastData.txt
            git commit -m "update lastData.txt"
            git push
          fi
```

### 4. 启动脚本

`entrypoint.sh`：

```sh
#!/bin/bash
go build -o deadpoolplus main.go
timeout --preserve-status 150 ./deadpoolplus
status=$?
if [ $status -eq 124 ]; then
    echo "The command timed out."
else
    echo "The command finished successfully."
fi
exit 0
```

> `timeout` 值根据数据量调整。FOFA 184 个国家查询耗时会比原版长很多，建议设足够大。

### 5. 完整目录结构

![结构](./images/struct.png)

---

## 0x06 编译多平台二进制

```bash
# Linux x64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/deadpoolplus_linux_amd64 main.go

# Linux ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o build/deadpoolplus_linux_arm64 main.go

# Windows x64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/deadpoolplus_windows_amd64.exe main.go

# Windows ARM64
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="-s -w" -o build/deadpoolplus_windows_arm64.exe main.go

# macOS Intel
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o build/deadpoolplus_darwin_amd64 main.go

# macOS Apple Silicon
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o build/deadpoolplus_darwin_arm64 main.go
```

或使用 GoReleaser：`goreleaser build --snapshot --clean`

---

## 致谢

- [Deadpool](https://github.com/thinkoaa/Deadpool) — 原作者 [thinkoaa](https://github.com/thinkoaa)，本工具在此基础上增强而来
- [go-socks5](https://github.com/armon/go-socks5) — SOCKS5 协议实现
- [go-toml](https://github.com/pelletier/go-toml) — TOML 解析
- [cron](https://github.com/robfig/cron) — Cron 调度

---

> **如果觉得有用，给原版和 DeadpoolPlus 都点个 Star ⭐**
