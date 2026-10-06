# ctfile-down

一个跨平台的城通网盘（CTfile）下载工具，支持 **分段加速下载** 与 **Web UI**。

- 单文件二进制，零依赖，无需安装 aria2 等外部程序
- 自动解析城通分享链接与提取码
- 直链解析 + HTTP Range 分段下载，规避免费限速
- 内置 Web UI：粘贴链接 → 解析 → 下载，实时进度 / 速度 / 取消
- 跨平台：Windows / Linux / macOS（amd64 / arm64 / 386 / arm）

## 快速开始

### 下载二进制

从 [Releases](https://github.com/Gordonynh/ctfile-down/releases) 下载对应平台的二进制，解压后直接运行。

### CLI 下载

```bash
./ctfile-down "https://url67.ctfile.com/f/65712267-17569899321195-e8ce7f?p=5577"
```

指定输出目录 / 提取码：

```bash
./ctfile-down "https://url67.ctfile.com/f/xxx" -o ./downloads
./ctfile-down "https://url67.ctfile.com/f/xxx" -p 5577 -o ./downloads
```

`-o` 可以放在链接之前或之后。若 `-o` 指向一个已存在的目录，则文件会以原名保存到该目录；否则视为完整文件路径。

### Web UI

```bash
./ctfile-down serve -addr :8080 -dir ./downloads
```

然后浏览器打开 <http://localhost:8080>。

## 工作原理

1. **解析链接**：从分享链接中提取文件标识与提取码，调用 `getfile.php` 获取文件元信息（文件名、大小、校验参数）。
2. **获取直链**：调用 `get_down_url.php` 换取一次性下载直链（`downurl`）。
3. **分段下载**：官方数据 CDN 只有在请求带 `Range` 头时才返回文件（否则返回 503 反盗链页）；且单个直链的满速额度约 15.9 MB，之后会被限速到 ~100 KB/s。因此工具把文件切成约 15 MB 的段，**每一段都重新换取一个独立直链**并串行下载，最后按序合并，从而尽量保持在满速区间并保证文件完整。
4. **校验**：每段校验字节数，合并后校验总大小。

> 说明：官方直链的数据 CDN 对同一 IP 的并发连接存在限制（实测多连接并发会被 503 拦截），因此本项目采用「分片 + 每片换直链 + 串行」的策略，而非盲目并发。如需真正的 64 线程并发，需借助第三方中继（参考 [ericwang2006/ctfile-cli](https://github.com/ericwang2006/ctfile-cli) 的做法）。

## 编译

要求 Go 1.22+。

```bash
git clone https://github.com/Gordonynh/ctfile-down.git
cd ctfile-down
go build -o ctfile-down .
```

### 交叉编译全平台

```bash
make build-all      # 输出到 dist/
```

支持的目标（`make build-all`）：

| 平台 | 架构 |
|------|------|
| windows | amd64, arm64 |
| linux | amd64, arm64, 386, arm(v7) |
| darwin | amd64, arm64 |

## Web API

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/resolve` | `{"url":"..."}` → 返回文件名 / 大小 |
| `POST` | `/api/download` | `{"url":"..."}` → 创建下载任务 |
| `GET` | `/api/tasks` | 任务列表 |
| `GET` | `/api/tasks/{id}` | 单个任务详情 |
| `POST` | `/api/tasks/{id}` | 取消任务 |
| `DELETE` | `/api/tasks/{id}` | 删除任务（清理临时文件） |

## 免责声明

本项目仅供学习交流使用。请勿用于非法用途；下载内容请遵守版权及城通网盘服务条款，后果自负。
