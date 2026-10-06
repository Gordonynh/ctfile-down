# ctfile-down

一个跨平台的城通网盘（CTfile）下载工具，支持 **分段加速下载** 与 **Web UI**。

- 单文件二进制，零依赖，无需安装 aria2 等外部程序
- 自动解析城通分享链接与提取码
- 直链解析 + HTTP Range 分段下载，规避免费限速
- 内置 Web UI：粘贴链接 → 解析 → 下载，实时进度 / 速度 / 取消
- 跨平台：Windows / Linux / macOS（amd64 / arm64 / 386 / arm）

## 快速开始

从 [Releases](https://github.com/Gordonynh/ctfile-down/releases) 下载对应平台的产物。

### macOS

下载 `ctfile-down_x.y.z_darwin_universal.dmg`，打开后把 **ctfile-down** 拖进「应用程序」，之后双击图标即可：

- 打开的是一个**原生 App 窗口**，窗口内就是 Web UI；
- App 会在后台自动启动内置服务（随机端口，仅监听 127.0.0.1），退出 App 时自动关闭；
- 文件默认保存到 `~/Downloads/ctfile-down`。

> 首次打开若提示「无法验证开发者」（未签名分发），请**右键 → 打开**，或执行
> `xattr -dr com.apple.quarantine /Applications/ctfile-down.app`。

### Windows

双击 `ctfile-down_x.y.z_windows_amd64.exe` 即可：

- 弹出控制台窗口显示服务地址与日志；
- 自动用默认浏览器打开 Web UI；
- 关闭控制台窗口即停止服务。

### Linux

```bash
chmod +x ctfile-down_*_linux_amd64
./ctfile-down_*_linux_amd64        # 无参数：启动服务并自动打开浏览器
```

在文件管理器中双击可执行文件同样会启动服务并打开浏览器（若无图形环境，请用下面的
`serve` 子命令并在浏览器中访问提示的地址）。

### CLI 下载

```bash
./ctfile-down "https://url67.ctfile.com/f/65712267-17569899321195-e8ce7f?p=5577"
./ctfile-down "https://url67.ctfile.com/f/xxx" -o ./downloads
./ctfile-down "https://url67.ctfile.com/f/xxx" -p 5577 -o ./downloads
```

`-o` 可以放在链接之前或之后。若 `-o` 指向一个已存在的目录，则文件会以原名保存到该目录；否则视为完整文件路径。

### serve 子命令

```bash
./ctfile-down serve                 # 默认 127.0.0.1:8080
./ctfile-down serve -addr :8080 -dir ~/Downloads -open
```

| 选项 | 说明 |
|------|------|
| `-addr` | 监听地址，默认 `127.0.0.1:8080`；端口写 `0` 表示随机端口 |
| `-dir`  | 保存目录，默认 `~/Downloads/ctfile-down` |
| `-open` | 启动后自动打开浏览器 |

启动时会输出一行机器可读的就绪标记（供 GUI 启动器使用）：

```
[ready] http://127.0.0.1:8080
```

## 支持的链接格式

城通分享链接的一般结构是 `https://<域名>/<类型>/<ID>[?p=<提取码>]`：

| 形式 | 说明 |
|------|------|
| `https://url67.ctfile.com/f/65712267-17569899321195-e8ce7f?p=5577` | 文件链接，带提取码 |
| `https://url67.ctfile.com/f/65712267-17569899321195-e8ce7f` | 文件链接，**不带提取码** |
| `url67.ctfile.com/f/xxx?p=5577` | 省略协议头也可以 |
| `https://url.ctfile.com/s/xxx?p=1234&fk=..&d=..` | 分享链接（可识别，暂不支持下载） |

- **域名**：支持 `ctfile.com` / `ctfile.net` / `ctfile.cc` / `ctfile.cn` / `545c.com`
  及其各级子域（如 `url67.`、`url.`、`wap.`、`www.`）。
- **提取码**：可用查询参数 `?p=` / `?password=` / `?passcode=` 提供，也可留空。
  若链接中不含提取码而该文件又需要，程序会明确提示「需要提取码」，此时：
  - Web UI：在「提取码」输入框填写后重试；
  - CLI：用 `-p <提取码>` 指定。
- **类型**：目前支持 `/f/`（文件）下载；`/d/`（目录）、`/l/`（列表）、`/s/`（分享文件夹）
  会被识别并给出提示，暂不支持下载。`ctfile://xturl...` 这类 App 内分享码无法直接使用，
  请改用网页分享链接。

## 工作原理

1. **解析链接**：从分享链接中提取文件标识与提取码，调用 `getfile.php` 获取文件元信息（文件名、大小、校验参数）。
2. **获取直链**：调用 `get_down_url.php` 换取一次性下载直链（`downurl`）。
3. **分段下载**：官方数据 CDN 只有在请求带 `Range` 头时才返回文件（否则返回 503 反盗链页）；且单个直链的满速额度约 15.9 MB，之后会被限速到 ~100 KB/s。因此工具把文件切成约 15 MB 的段，**每一段都重新换取一个独立直链**并串行下载，最后按序合并，从而尽量保持在满速区间并保证文件完整。
4. **校验**：每段校验字节数，合并后校验总大小。

> 说明：官方直链的数据 CDN 对同一 IP 的并发连接存在限制（实测多连接并发会被 503 拦截），因此本项目采用「分片 + 每片换直链 + 串行」的策略，而非盲目并发。如需真正的 64 线程并发，需借助第三方中继（参考 [ericwang2006/ctfile-cli](https://github.com/ericwang2006/ctfile-cli) 的做法）。

## 编译

要求 Go 1.22+。macOS 打包 `.app` 还需要 Xcode 命令行工具（`swiftc`、`lipo`、`iconutil`）。

```bash
git clone https://github.com/Gordonynh/ctfile-down.git
cd ctfile-down
go build -o ctfile-down .
```

### 构建全平台产物

```bash
make build-all      # 8 个平台的独立二进制 -> dist/
make app            # macOS .app（仅 darwin 主机）
make dmg            # macOS DMG 安装包（仅 darwin 主机）
make release-all    # 上面全部（在 macOS 上会额外产出 .app 与 DMG）
```

支持的独立二进制目标：

| 平台 | 架构 |
|------|------|
| windows | amd64, arm64 |
| linux | amd64, arm64, 386, arm(v7) |
| darwin | amd64, arm64 |

macOS 的 `.app` 与 DMG 为 **universal**（arm64 + x86_64）通用二进制。

### macOS App 结构

```
ctfile-down.app/Contents/
├── MacOS/ctfile-down              # Swift + WKWebView 启动器（通用二进制）
├── Resources/ctfile-down-server   # 内置 Go 服务端（通用二进制）
├── Resources/icon.icns
└── Info.plist
```

启动器会拉起内置服务并读取其 `[ready] <url>` 输出，然后在 App 窗口内用 WKWebView
加载 Web UI；退出时终止子进程（子进程也会在父进程退出时自动退出，不会残留）。

## Web API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/resolve` | `{"url":"...","passcode":"..."}` → 文件名 / 大小 |
| `POST` | `/api/download` | `{"url":"...","passcode":"..."}` → 创建下载任务 |
| `GET` | `/api/config` | 运行时配置（保存目录等） |
| `GET` | `/api/tasks` | 任务列表 |
| `GET` | `/api/tasks/{id}` | 单个任务详情 |
| `POST` | `/api/tasks/{id}` | 取消任务 |
| `DELETE` | `/api/tasks/{id}` | 删除任务（清理临时文件） |

出错时返回 `{"code":"...","error":"..."}`，其中 `code` 便于前端区分处理：

| code | HTTP | 含义 |
|------|------|------|
| `need_passcode` | 401 | 链接需要提取码（或提取码不正确），请带上 `passcode` 重试 |
| `bad_link` | 400 | 链接格式/域名不支持 |
| `resolve_failed` | 502 | 上游解析或取直链失败 |
| `download_failed` | 400 | 创建下载任务失败 |

## 免责声明

本项目仅供学习交流使用。请勿用于非法用途；下载内容请遵守版权及城通网盘服务条款，后果自负。
