// ctfile-down macOS 启动器
//
// 作为一个原生 App 窗口运行：启动内嵌的 ctfile-down-server 子进程，
// 读取它输出的 [ready] <url> 就绪行，然后用 WKWebView 在该窗口中
// 加载 Web UI。退出时终止子进程。

import Cocoa
import WebKit

final class AppDelegate: NSObject, NSApplicationDelegate, WKNavigationDelegate {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var server: Process?
    private let outputPipe = Pipe()
    private let inputPipe = Pipe()   // 保持写端打开；启动器退出时关闭，服务端据此自杀
    private var buffer = ""
    private var serverURL: URL?
    private var retryCount = 0

    func applicationDidFinishLaunching(_ notification: Notification) {
        buildMenu()
        buildWindow()
        startServer()
    }

    // MARK: - 菜单（提供 Cmd+Q 等标准操作）

    private func buildMenu() {
        let mainMenu = NSMenu()
        let appMenuItem = NSMenuItem()
        mainMenu.addItem(appMenuItem)

        let appMenu = NSMenu()
        appMenu.addItem(withTitle: "关于 ctfile-down",
                        action: #selector(NSApplication.orderFrontStandardAboutPanel(_:)),
                        keyEquivalent: "")
        appMenu.addItem(NSMenuItem.separator())
        appMenu.addItem(withTitle: "隐藏 ctfile-down",
                        action: #selector(NSApplication.hide(_:)),
                        keyEquivalent: "h")
        appMenu.addItem(NSMenuItem.separator())
        appMenu.addItem(withTitle: "退出 ctfile-down",
                        action: #selector(NSApplication.terminate(_:)),
                        keyEquivalent: "q")
        appMenuItem.submenu = appMenu

        NSApplication.shared.mainMenu = mainMenu
    }

    // MARK: - 窗口

    private func buildWindow() {
        let rect = NSRect(x: 0, y: 0, width: 1000, height: 740)
        window = NSWindow(contentRect: rect,
                          styleMask: [.titled, .closable, .miniaturizable, .resizable],
                          backing: .buffered,
                          defer: false)
        window.title = "ctfile-down"
        window.minSize = NSSize(width: 680, height: 480)
        window.center()
        window.setFrameAutosaveName("CTFileDownMainWindow")

        let config = WKWebViewConfiguration()
        config.preferences.setValue(true, forKey: "developerExtrasEnabled")
        webView = WKWebView(frame: rect, configuration: config)
        webView.autoresizingMask = [.width, .height]
        webView.navigationDelegate = self
        if #available(macOS 13.3, *) {
            webView.isInspectable = true
        }

        window.contentView = webView
        window.makeKeyAndOrderFront(nil)
        NSApplication.shared.activate(ignoringOtherApps: true)

        loadPlaceholder()
    }

    private func loadPlaceholder() {
        let html = """
        <!DOCTYPE html><html><head><meta charset="utf-8"><style>
        html,body{height:100%}
        body{margin:0;background:#0f1117;color:#e6e8ee;
          font-family:-apple-system,BlinkMacSystemFont,"PingFang SC",sans-serif;
          display:flex;align-items:center;justify-content:center}
        .box{text-align:center}
        .ring{width:36px;height:36px;margin:0 auto 18px;border-radius:50%;
          border:3px solid #2a2f3a;border-top-color:#3b82f6;animation:spin 0.9s linear infinite}
        @keyframes spin{to{transform:rotate(360deg)}}
        .t{font-size:14px;color:#8b93a7}
        </style></head><body>
        <div class="box"><div class="ring"></div><div class="t">正在启动服务…</div></div>
        </body></html>
        """
        webView.loadHTMLString(html, baseURL: nil)
    }

    // MARK: - 启动内置服务

    private func startServer() {
        guard let path = serverBinaryPath() else {
            showError("未找到内置的 ctfile-down-server 可执行文件。")
            return
        }

        let saveDir = defaultSaveDir()
        try? FileManager.default.createDirectory(atPath: saveDir, withIntermediateDirectories: true)

        let p = Process()
        p.executableURL = URL(fileURLWithPath: path)
        p.arguments = ["serve", "-addr", "127.0.0.1:0", "-dir", saveDir, "-watch-stdin"]
        p.standardOutput = outputPipe
        p.standardError = outputPipe
        p.standardInput = inputPipe
        p.terminationHandler = { [weak self] proc in
            DispatchQueue.main.async {
                guard let self = self else { return }
                if proc.terminationStatus != 0 {
                    self.showError("服务进程已退出（状态码 \(proc.terminationStatus)）。")
                }
            }
        }
        outputPipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            self?.consume(data)
        }

        do {
            try p.run()
        } catch {
            showError("无法启动服务：\(error.localizedDescription)")
            return
        }
        server = p
    }

    /// 解析子进程输出，寻找 [ready] <url> 行。
    private func consume(_ data: Data) {
        FileHandle.standardOutput.write(data) // 转发日志到控制台
        guard let chunk = String(data: data, encoding: .utf8) else { return }
        buffer += chunk
        while let nl = buffer.firstIndex(of: "\n") {
            let line = String(buffer[buffer.startIndex..<nl])
            buffer.removeSubrange(buffer.startIndex...nl)
            guard line.hasPrefix("[ready] ") else { continue }
            let raw = String(line.dropFirst("[ready] ".count))
                .trimmingCharacters(in: .whitespacesAndNewlines)
            if let url = URL(string: raw) {
                DispatchQueue.main.async { [weak self] in self?.load(url) }
            }
        }
    }

    private func load(_ url: URL) {
        serverURL = url
        webView.load(URLRequest(url: url))
    }

    private func serverBinaryPath() -> String? {
        if let bundled = Bundle.main.path(forResource: "ctfile-down-server", ofType: nil) {
            return bundled
        }
        // 回退：与启动器可执行文件同目录（便于开发时直接运行）。
        let exe = URL(fileURLWithPath: CommandLine.arguments[0]).resolvingSymlinksInPath()
        let sibling = exe.deletingLastPathComponent()
            .appendingPathComponent("ctfile-down-server").path
        return FileManager.default.fileExists(atPath: sibling) ? sibling : nil
    }

    private func defaultSaveDir() -> String {
        let base = FileManager.default.urls(for: .downloadsDirectory, in: .userDomainMask).first
            ?? URL(fileURLWithPath: NSHomeDirectory())
        return base.appendingPathComponent("ctfile-down").path
    }

    private func showError(_ message: String) {
        let alert = NSAlert()
        alert.messageText = "ctfile-down"
        alert.informativeText = message
        alert.alertStyle = .warning
        alert.runModal()
    }

    // MARK: - 生命周期

    func applicationWillTerminate(_ notification: Notification) {
        outputPipe.fileHandleForReading.readabilityHandler = nil
        if let p = server, p.isRunning {
            p.terminate()
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }

    // Web 加载失败时短暂重试，等待服务就绪。
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!,
                 withError error: Error) {
        guard let url = serverURL, retryCount < 10 else { return }
        retryCount += 1
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) { [weak self] in
            self?.webView.load(URLRequest(url: url))
        }
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        retryCount = 0
    }
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
