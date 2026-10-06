package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Gordonynh/ctfile-down/internal/ctfile"
	"github.com/Gordonynh/ctfile-down/internal/server"
)

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		serveCmd(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("ctfile-down %s\n", version)
	case "help", "-h", "--help":
		usage()
	default:
		downloadCmd(os.Args[1:])
	}
}

func usage() {
	fmt.Print(`ctfile-down — 城通网盘(CTFile)下载工具，支持分段加速与 Web UI

用法:
  ctfile-down <链接> [选项]    下载单个文件
  ctfile-down serve [选项]     启动 Web UI

下载选项:
  -o string   输出目录或文件路径 (默认: 当前目录)
  -p string   提取码 (可选，未提供时自动读取链接中的 ?p=)

serve 选项:
  -addr string 监听地址 (默认 ":8080")
  -dir string  默认保存目录 (默认 "./downloads")
`)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "错误: "+format+"\n", a...)
	os.Exit(1)
}

func downloadCmd(args []string) {
	// 手动解析参数，使 -o / -p 可放在链接之前或之后。
	var (
		out      = "."
		passcode string
		positional []string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--output":
			if i+1 < len(args) {
				out = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "-o="):
			out = strings.TrimPrefix(a, "-o=")
		case a == "-p" || a == "--passcode":
			if i+1 < len(args) {
				passcode = args[i+1]
				i++
			}
		case strings.HasPrefix(a, "-p="):
			passcode = strings.TrimPrefix(a, "-p=")
		case a == "-h" || a == "--help":
			usage()
			os.Exit(0)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) < 1 {
		fmt.Fprintln(os.Stderr, "错误: 缺少下载链接")
		usage()
		os.Exit(2)
	}

	link, err := ctfile.ParseLink(positional[0])
	if err != nil {
		fatal("解析链接失败: %v", err)
	}
	if passcode != "" {
		link.Passcode = passcode
	}

	client := ctfile.New(link)
	info, err := client.Resolve()
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("文件: %s (%s)\n", info.FileName, info.SizeDisplay)

	dst := out
	if st, err := os.Stat(dst); err == nil && st.IsDir() {
		dst = filepath.Join(dst, info.FileName)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("开始下载到: %s\n", dst)
	start := time.Now()
	lastTime := time.Now()
	var lastDone int64

	err = client.Download(ctx, info, dst, func(done, total int64) {
		now := time.Now()
		if elapsed := now.Sub(lastTime).Seconds(); elapsed >= 0.5 || done == total {
			speed := int64(float64(done-lastDone) / elapsed)
			pct := 100 * float64(done) / float64(total)
			fmt.Printf("\r  %6.2f%%  %s / %s  %s/s    ",
				pct, humanBytes(done), humanBytes(total), humanBytes(speed))
			lastTime, lastDone = now, done
		}
	})
	if err != nil {
		fmt.Printf("\n下载失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n完成 ✓ 用时 %.1fs  →  %s\n", time.Since(start).Seconds(), dst)
}

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", ":8080", "监听地址")
	dir := fs.String("dir", "./downloads", "默认保存目录")
	_ = fs.Parse(args)

	srv := server.New(*dir)
	fmt.Printf("ctfile-down %s  Web UI 已启动: http://localhost%s\n", version, *addr)
	if err := srv.ListenAndServe(*addr); err != nil {
		fatal("服务启动失败: %v", err)
	}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
