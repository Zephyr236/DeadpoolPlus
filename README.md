<img src="images/deadpool.png" style="zoom:30%;transform: scale(0.3);" width="35%" height="35%"/>

# DeadpoolPlus — 全球多协议代理池

DeadpoolPlus 是一个多源、多协议的 SOCKS5 代理池工具，受 [Deadpool](https://github.com/thinkoaa/Deadpool) 启发并大幅增强。可从 **FOFA 空间测绘**、**公开代理池服务**、**GitHub/API 公开列表** 等多种渠道自动化采集代理，支持 **SOCKS5 / SOCKS4 / HTTP / HTTPS** 四种协议，经存活检测后汇聚成本地 SOCKS5 代理池，供 Burp Suite、Proxifier、SwitchyOmega 等工具轮询切换出口 IP。

> 🚀 **一句话：** 自动从全网搜集代理 → 多协议死/活检测 → 本地 SOCKS5 服务 → 工具接入即用

## 快速开始

### 1. 配置

编辑 `config.toml`，填入 FOFA 账号：

```toml
[FOFA]
switch = 'open'
email = 'your@email.com'
key = 'your-fofa-key'
```

### 2. 运行

```bash
# Linux/Mac
./DeadpoolPlus_linux_amd64

# Windows
DeadpoolPlus_windows_amd64.exe

# 指定配置文件
./DeadpoolPlus_linux_amd64 -c custom.toml
```

### 3. 接入工具

将 Burp/Proxifier/SwitchyOmega 的 SOCKS5 代理设置为 `127.0.0.1:10086`。

### 4. 运行时操作

| 操作 | 功能 |
|---|---|
| 按 `s` + 回车 | 查看代理统计（使用次数/成功率/响应时间） |
| 按回车 | 随机切换到下一个代理 IP |
| 创建 `.dump_stats` 文件 | 后台运行时触发统计输出 |
| `Ctrl+C` | 优雅退出 |

### 5. 首次运行（无 lastData.txt）

如果没有 `lastData.txt`，第一次运行程序会从零开始采集代理（耗时 1-2 分钟）。若不想等待，可从仓库直接下载 GitHub Actions 定时采集好的文件：

```bash
# 从仓库下载最新的 lastData.txt
curl -O https://raw.githubusercontent.com/Zephyr236/DeadpoolPlus/main/lastData.txt

# 启动即可秒级就绪
./DeadpoolPlus_linux_amd64
```

> 前提：仓库已配置 GitHub Actions 定时采集（见下方章节），产出 `lastData.txt` 并自动提交到仓库。

---

## 代理数据流

```
┌─────────────┐   ┌──────────────┐   ┌──────────────┐
│ FOFA SOCKS5 │   │ 代理池爬取    │   │ 公开列表下载  │
│ (24国查询)   │   │ (pool/all)   │   │ (30+ GitHub) │
└──────┬──────┘   └──────┬───────┘   └──────┬───────┘
       │                  │                   │
       └──────────────────┼───────────────────┘
                          ▼
              ┌─────────────────────┐
              │ 去重 + 健康检测      │ ← HTTP msftconnecttest, 并发1000, 超时3s
              │ (四协议自动探测)     │
              └─────────┬───────────┘
                        ▼
              ┌─────────────────────┐
              │ 有效代理池           │ → lastData.txt 持久化
              └─────────┬───────────┘
                        ▼
              ┌─────────────────────┐
              │ 本地 SOCKS5 服务     │ ← 127.0.0.1:10086
              │ 随机轮询 + 统计淘汰  │
              └─────────┬───────────┘
                        ▼
                 外部工具接入
```

---

## 配置说明

```toml
[listener]
IP = '127.0.0.1'          # 监听地址
PORT = 10086              # 监听端口
userName = ''             # 认证（空=不认证）
password = ''
logLevel = 'normal'       # normal | debug

[checkSocks]
checkURL = 'http://www.msftconnecttest.com/connecttest.txt'  # 健康检测 URL
checkRspKeywords = 'Microsoft Connect Test'                   # 关键字
maxConcurrentReq = 1000   # 并发数（VPS 500-1000）
timeout = 3               # 超时秒数
maxFailCount = 3          # 连续失败 N 次淘汰

[FOFA]
switch = 'open'
email = 'your@email.com'
key = 'your-fofa-key'

# SOCKS5 查询（24 个高产国家）
queryStrings = [
    'protocol=="socks5" && country="JP" && banner="Method:No Authentication"',
    ...
]
resultSize = 10000

# 代理池爬取：搜索公开代理池 → /all 获取代理
poolQueryString = 'body="get all proxy from proxy pool"'
poolResultSize = 10000

# 公开列表：30+ GitHub/API 源
proxyListUrls = [
    'https://raw.githubusercontent.com/TheSpeedX/PROXY-List/refs/heads/master/socks5.txt',
    ...
]
```

---

## 代理来源

### 1. FOFA 空间测绘
按国家查询 SOCKS5 代理（已筛选 24 个高产国家 >100 条/国），单次最多 10,000 条/国。

### 2. 代理池爬取
从 FOFA 搜索公开的代理池服务（`body="get all proxy from proxy pool"`），并发请求 `/all` 接口获取已维护代理。实测 358 个池 → 2,138 个可用代理。

### 3. 公开列表下载
从 30+ GitHub/API 源批量下载代理列表，支持有/无协议前缀两种格式。无协议前缀的 IP:PORT 自动生成四种协议变体，由健康检测筛选。

### 4. 本地文件
`lastData.txt` 每行一个代理（`protocol://IP:PORT` 或 `IP:PORT`），随程序启动自动加载。

---

## 多协议支持

DeadpoolPlus 支持四种上游代理协议：

| 协议 | 拨号方式 | 备注 |
|---|---|---|
| SOCKS5 | `golang.org/x/net/proxy.SOCKS5()` | 标准实现 |
| SOCKS4 | 自实现拨号器 | 仅 IPv4 |
| HTTP | HTTP CONNECT 隧道 | 明文代理 |
| HTTPS | TLS + HTTP CONNECT | 加密代理 |

**智能协议探测：** 当代理来源只提供 IP:PORT 而无协议信息时，自动生成 `socks5://`、`socks4://`、`http://`、`https://` 四种变体，由健康检测筛选有效协议。不同协议的同 IP:PORT 视为不同代理。

---

## 统计面板

按 `s` + 回车查看实时统计：

```
==========================================================================================
  Proxy Stats (total: 24)
==========================================================================================
  ADDR                        USES     OK   FAIL    RATE   AVG(ms) STREAK
--------------------------------------------------------------------------------
  http://103.183.13.94:8080     16     16      0  100.0%     416ms     0
  socks5://174.138.16.218:1080  14     14      0  100.0%     539ms     0
  socks4://193.43.159.161:1080  12     10      2   83.3%    1780ms     0
  socks5://149.28.22.132:1080   12     12      0  100.0%     799ms     0
==========================================================================================
```

- ADDR 列直接显示协议前缀，一眼区分代理类型
- 按使用次数降序排列
- 颜色标记：绿色正常 / 黄色警告 / 红色异常
- 连续失败达到阈值自动淘汰

---

## 快速启动模式

如果有 `lastData.txt`（上次运行保存的有效代理），程序会**直接加载并立即启动 SOCKS5 服务**，FOFA 采集和新代理检测在后台异步进行。

```
# 首次运行（无 lastData.txt）
./deadpoolplus                    # 采集 → 检测 → 启动服务（等待 ~1-2 分钟）

# 后续运行（有 lastData.txt）
./deadpoolplus                    # 秒级启动！后台自动补充新代理
```

---

## 命令行参数

| 参数 | 说明 |
|---|---|
| `-c, --config <path>` | 指定配置文件路径（默认 config.toml） |
| `-l, --lastdata <path>` | 指定 lastdata 文件路径 |
| `--collect-only` | 仅采集+检测代理后退出，不启动 SOCKS5 服务（适用于定时任务） |
| `-h, --help` | 显示帮助信息 |

```bash
# CI/定时任务模式：采完即退
./deadpoolplus --collect-only
```

---

## GitHub Actions 定时采集

项目已内置 CI 工作流 `.github/workflows/collect-proxies.yml`，每 6 小时自动采集代理并提交 `lastData.txt`。

**部署步骤：**
1. Fork/Import 本仓库（**设为 Private**）
2. Settings → Actions → General → 勾选 Read and write permissions
3. Settings → Secrets → Actions → 添加 `FOFA_KEY`
4. Actions 标签页手动触发或等待定时执行

---

## 编译

```bash
# 单平台
go build -ldflags="-s -w" -o deadpoolplus main.go

# 多平台
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/DeadpoolPlus_linux_amd64 .
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o build/DeadpoolPlus_windows_amd64.exe .
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o build/DeadpoolPlus_darwin_amd64 .
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o build/DeadpoolPlus_darwin_arm64 .
```

---

## 致谢

- [Deadpool](https://github.com/thinkoaa/Deadpool) — 原作者 [thinkoaa](https://github.com/thinkoaa)，本项目在此基础上发展而来
- [go-socks5](https://github.com/armon/go-socks5) — SOCKS5 协议实现
- [go-toml](https://github.com/pelletier/go-toml) — TOML 解析

---

## 免责声明

本工具仅面向**合法授权**的企业安全建设行为。使用者应确保行为符合当地法律法规并已取得足够授权。
