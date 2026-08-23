package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/webview/webview_go"
	"github.com/yann0917/dedao-gui/backend/app"
)

// loginSrv / loginCancel 仅用于清理，避免重复拉起与资源泄漏。
var loginSrv *http.Server
var loginCancel context.CancelFunc

// --- 登录诊断日志 ---
// 内联登录在真实机器上若失败（窗口打开但进不去站点），没有任何报错可见。
// 这里把所有关键节点写入 %LOCALAPPDATA%\dedao_login.log，便于事后定位。
func loginLogPath() string {
	if p := os.Getenv("LOCALAPPDATA"); p != "" {
		return filepath.Join(p, "dedao_login.log")
	}
	return "dedao_login.log"
}

func loginLog(format string, args ...interface{}) {
	f, err := os.OpenFile(loginLogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] %s\n", ts, fmt.Sprintf(format, args...))
}

// OpenLoginBrowser 在主程序进程内启动一个原生 WebView2 窗口（内联模式），
// 指向 dedao.cn。用户在窗口内完成手机号+滑块登录后，
// 通过 WebView2 原生 CookieManager 轮询读取 HttpOnly Cookie（GAT/ISID），
// 检测到登录态后通过本地 HTTP 回调 POST 回自身，再由后端解析完成登录。
//
// 内联后不再依赖外部 login_helper.exe，分发只需一个 exe。
//
// 完成后通过 Wails 事件通知前端：
//   - "login:success" -> { user, cookie }
//   - "login:error"   -> string(错误信息)
func (a *App) OpenLoginBrowser() error {
	// 0) 清理上一次可能残留的实例
	if loginCancel != nil {
		loginCancel()
		loginCancel = nil
	}
	if loginSrv != nil {
		_ = loginSrv.Shutdown(context.Background())
		loginSrv = nil
	}

	// 1) 本地回调服务器（随机空闲端口）
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("启动登录回调服务失败: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	cbURL := fmt.Sprintf("http://127.0.0.1:%d/cb", port)

	ctx, cancel := context.WithCancel(context.Background())
	loginCancel = cancel

	mux := http.NewServeMux()
	mux.HandleFunc("/cb", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		cookie := string(raw)
		w.WriteHeader(http.StatusNoContent)
		loginLog("callback /cb received, body_len=%d", len(cookie))
		go a.finishLoginFromCookie(cookie)
		localSrv := loginSrv
		go func() {
			time.Sleep(200 * time.Millisecond)
			if localSrv != nil {
				_ = localSrv.Shutdown(context.Background())
			}
		}()
	})

	loginSrv = &http.Server{Handler: mux}
	go loginSrv.Serve(listener)

	loginLog("OpenLoginBrowser start, cbURL=%s", cbURL)

	// 2) 内联启动 WebView2 登录窗口（独立 goroutine + LockOSThread）
	go a.runLoginWebView(ctx, cbURL)

	return nil
}

// runLoginWebView 在专用 goroutine 中运行 WebView2 登录窗口。
// 必须调用 runtime.LockOSThread() 以满足 webview_go 的线程要求。
// 全部逻辑就地完成（单 exe，无外部进程、无默认浏览器降级）。
func (a *App) runLoginWebView(ctx context.Context, cbURL string) {
	// ① 兜住 C 层崩溃：fork 的 cookies.cc 走 CDP，极端情况下段错误会被
	//   顶层过滤器接住；这里再补一层 Go recover，确保窗口异常不会拖死主程序。
	defer func() {
		if r := recover(); r != nil {
			loginLog("PANIC in runLoginWebView: %v", r)
			wailsRuntime.EventsEmit(a.Ctx, "login:error", "登录窗口异常，请重试（详见 dedao_login.log）")
		}
	}()

	// 锁定 OS 线程：webview_go 的 init() 和所有操作必须在同一线程
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 安装崩溃过滤器
	webview.InstallCrashHandler()

	// 给登录窗口的 WebView2 指定一个【每次启动唯一】的临时用户数据目录。
	// 关键修复：fork 默认把目录硬编码成 APPDATA\dedao-gui.exe（所有启动共享），
	// 一旦某次崩溃残留的浏览器进程锁住该目录，后续每次启动环境创建都会失败、
	// m_webview 为 null，紧接着 Navigate 空指针 → 整进程崩溃（即“手机号登录闪崩”）。
	// 改成唯一临时目录后，不同启动/残留进程不再互相抢占，根因消除。
	loginWV2Dir := filepath.Join(os.TempDir(), fmt.Sprintf("dedao-login-%d-%d", os.Getpid(), time.Now().UnixNano()))
	_ = os.MkdirAll(loginWV2Dir, 0700)
	_ = os.Setenv("DEDDAO_LOGIN_WV2_DIR", loginWV2Dir)
	defer func() {
		// 窗口关闭后尽力清理临时目录（失败忽略）
		_ = os.RemoveAll(loginWV2Dir)
	}()

	// ② 创建窗口：若 WebView2 初始化失败（如运行时缺失/环境冲突），
	//   就地报错而不是静默卡死或无响应。
	w := webview.New(false)
	if w == nil {
		loginLog("webview.New returned nil (WebView2 init failed)")
		wailsRuntime.EventsEmit(a.Ctx, "login:error", "无法创建登录窗口：WebView2 初始化失败，请确认系统已安装 WebView2 运行时")
		return
	}
	defer w.Destroy()
	// 环境未就绪（m_webview 为 null）时绝不 Navigate —— 否则空指针直接崩进程。
	// 这层保护确保即使环境创建失败，也只是报错、主程序不受影响。
	if !w.IsReady() {
		loginLog("webview created but WebView2 environment NOT ready (m_webview nil)")
		wailsRuntime.EventsEmit(a.Ctx, "login:error", "登录窗口初始化失败：WebView2 环境创建未就绪，请重试或确认已安装 WebView2 运行时")
		return
	}
	w.SetTitle("登录得到")
	w.SetSize(460, 820, webview.HintNone)
	loginLog("WebView2 window created")

	var fired int32
	var loggedOnce int32

	// ③ 登录超时兜底：120 秒内未检测到 GAT/ISID，就地明确报错并关闭窗口。
	deadline := time.After(120 * time.Second)

	// 原生轮询：每 1.5s 在 UI 线程读 Cookie，检测到登录态即回传并关窗
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-deadline:
				if atomic.LoadInt32(&fired) == 0 {
					loginLog("login timeout 120s, no GAT/ISID detected")
					wailsRuntime.EventsEmit(a.Ctx, "login:error", "登录超时：120 秒内未检测到登录态，请重试")
					w.Terminate()
				}
				return
			case <-ticker.C:
				if atomic.LoadInt32(&fired) == 1 {
					return
				}
				w.Dispatch(func() {
					if atomic.LoadInt32(&fired) == 1 {
						return
					}
					cs := w.PollCookies()
					if cs != "" && atomic.CompareAndSwapInt32(&loggedOnce, 0, 1) {
						loginLog("PollCookies sample names: %s", cookieNames(cs))
					}
				if cs != "" && cookieHasLogin(cs) {
					loginLog("login detected via PollCookies (GAT+ISID present)")
					if atomic.CompareAndSwapInt32(&fired, 0, 1) {
						header := cookieListToHeader(cs)
						loginLog("posting cookie header, len=%d", len(header))
						// 回传 Cookie 放后台 goroutine（网络 IO，不要阻塞 UI 线程），
						// 但窗口退出必须在 UI 线程调用 Terminate（webview.h 的 terminate_impl
						// 用的是 PostQuitMessage(0)，必须发给本线程，否则 WM_QUIT 发到别的线程，
						// 小窗消息循环收不到 → 关不掉）。此处就在 Dispatch 回调的 UI 线程上。
						go postCookie(cbURL, header)
						w.Terminate()
					}
				}
				})
			}
		}
	}()

	w.Navigate("https://www.dedao.cn/")
	loginLog("WebView2 navigating to https://www.dedao.cn/")
	w.Run()
	loginLog("WebView2 window closed")
}

// finishLoginFromCookie 解析 Cookie 完成登录并通知前端。
func (a *App) finishLoginFromCookie(cookie string) {
	cookie = sanitizeCookie(cookie)
	if cookie == "" {
		loginLog("finishLoginFromCookie: empty cookie -> login:error")
		wailsRuntime.EventsEmit(a.Ctx, "login:error", "未能获取登录态")
		return
	}
	user, err := app.LoginByCookie(cookie)
	if err != nil || user == nil {
		msg := "登录失败：未能解析登录态"
		if err != nil {
			msg = err.Error()
		}
		loginLog("finishLoginFromCookie error: %s", msg)
		wailsRuntime.EventsEmit(a.Ctx, "login:error", msg)
		return
	}
	loginLog("finishLoginFromCookie success, uid=%s", user.UIDHazy)
	// 取消 WebView（若仍在运行）
	if loginCancel != nil {
		loginCancel()
		loginCancel = nil
	}
	wailsRuntime.EventsEmit(a.Ctx, "login:success", map[string]interface{}{
		"user":   user,
		"cookie": cookie,
	})
}

// sanitizeCookie 简单清洗（去换行、去包裹引号）。
func sanitizeCookie(c string) string {
	c = strings.ReplaceAll(c, "\r", "")
	c = strings.ReplaceAll(c, "\n", "")
	c = strings.ReplaceAll(c, "\t", "")
	c = strings.TrimSpace(c)
	c = strings.Trim(c, `"`)
	return c
}

// cookieNames 仅取 Cookie 名列表（不含值），用于诊断日志，避免泄露敏感信息。
func cookieNames(jsonStr string) string {
	var resp struct {
		Cookies []struct {
			Name string `json:"name"`
		} `json:"cookies"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return "(parse error)"
	}
	names := make([]string, 0, len(resp.Cookies))
	for _, c := range resp.Cookies {
		names = append(names, c.Name)
	}
	return strings.Join(names, ",")
}

// --- 以下为 Cookie 处理辅助函数（原 login_helper/main.go） ---

func cookieGatIsid(jsonStr string) (gat bool, isid bool) {
	var resp struct {
		Cookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"cookies"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return false, false
	}
	for _, c := range resp.Cookies {
		if c.Name == "GAT" && len(c.Value) > 8 {
			gat = true
		}
		if c.Name == "ISID" && len(c.Value) > 8 {
			isid = true
		}
	}
	return
}

func cookieHasLogin(jsonStr string) bool {
	gat, isid := cookieGatIsid(jsonStr)
	return gat && isid
}

// cookieListToHeader 把 Network.getCookies 返回的 JSON 转成 "name=value; name=value" 格式。
// 只保留得到域名下的 Cookie（及其关键的 GAT/ISID），避免混入登录过程中第三方 iframe 的 Cookie。
func cookieListToHeader(jsonStr string) string {
	var resp struct {
		Cookies []struct {
			Name   string `json:"name"`
			Value  string `json:"value"`
			Domain string `json:"domain"`
		} `json:"cookies"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return ""
	}
	parts := make([]string, 0, len(resp.Cookies))
	for _, c := range resp.Cookies {
		if c.Name == "" {
			continue
		}
		keep := false
		if c.Domain != "" && strings.Contains(strings.ToLower(c.Domain), "dedao") {
			keep = true
		}
		if c.Name == "GAT" || c.Name == "ISID" {
			keep = true
		}
		if !keep {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

func postCookie(cb, cookie string) {
	if cb == "" {
		return
	}
	body := strings.NewReader(cookie)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(cb, "text/plain", body)
	if err != nil {
		loginLog("postCookie error: %v", err)
		return
	}
	loginLog("postCookie ok, status=%d", resp.StatusCode)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
