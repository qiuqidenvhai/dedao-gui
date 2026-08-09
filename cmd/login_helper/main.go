package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/webview/webview_go"
)

// login_helper 是一个独立进程，负责打开指向 dedao.cn 的原生 WebView2 窗口。
// 之所以做成独立进程：webview_go 必须在主线程运行（其 init 锁了 OS 线程），
// 放进 Wails 主程序（在 goroutine 里跑）会导致 WebView2 白屏，且关窗会拖垮主程序。
//
// 用法：login_helper.exe <callbackURL>
//   - 窗口内用户在 dedao.cn 用手机号+滑块登录；
//   - 本进程通过 WebView2 原生 CookieManager 轮询读取全部 Cookie（含 HttpOnly 的 GAT/ISID）；
//   - 检测到登录态后，用 Go 的 HTTP 客户端把 cookie POST 给 <callbackURL>（绕过混合内容限制）；
//   - 随后关闭窗口并退出。

var logf = func(format string, args ...interface{}) {}

func main() {
	// 无控制台窗口时，把关键日志写到临时文件便于排查。
	logPath := ""
	if tmp, err := os.UserCacheDir(); err == nil {
		logPath = tmp + "/dedao_login_helper.log"
	} else {
		logPath = os.TempDir() + "/dedao_login_helper.log"
	}
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		logf = func(format string, args ...interface{}) {
			fmt.Fprintf(f, time.Now().Format("15:04:05")+" "+format+"\n", args...)
		}
		defer f.Close()
	}
	logf("start, args=%v", os.Args)

	// 安装 C 层崩溃过滤器：万一原生 Cookie 读取段错误，把崩溃地址写进 .crash.log
	webview.InstallCrashHandler()

	cb := ""
	if len(os.Args) > 1 {
		cb = os.Args[1]
	}

	// 清掉上次持久化的 WebView2 用户数据目录（%APPDATA%/login_helper.exe），
	// 让每次打开登录窗都是全新会话。否则主程序退出登录后，小窗仍会显示“已登录”，
	// 且下次拉起时直接带着旧登录态，退出登录无法真正清除小窗状态。
	if appData := os.Getenv("APPDATA"); appData != "" {
		dataDir := filepath.Join(appData, "login_helper.exe")
		if err := os.RemoveAll(dataDir); err == nil {
			logf("cleared persisted webview data: %s", dataDir)
		}
	}

	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("登录得到")
	w.SetSize(460, 820, webview.HintNone)

	var fired int32
	var loggedOnce int32

	// 原生轮询：每 1.5s 在 UI 线程读一次 Cookie，检测到登录态即回传并关窗。
	go func() {
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if atomic.LoadInt32(&fired) == 1 {
				return
			}
			w.Dispatch(func() {
				if atomic.LoadInt32(&fired) == 1 {
					return
				}
				cs := w.PollCookies()
				if cs != "" && atomic.CompareAndSwapInt32(&loggedOnce, 0, 1) {
					gat, isid := cookieGatIsid(cs)
					logf("cookie read ok, len=%d, hasGAT=%v, hasISID=%v", len(cs), gat, isid)
				}
				if cs != "" && cookieHasLogin(cs) {
					if atomic.CompareAndSwapInt32(&fired, 0, 1) {
						// 关键：DevTools 返回的是 JSON，必须转成 "name=value; name=value"
						// 格式再回传，否则主程序 services.ParseCookies 解析不出 GAT/ISID
						// （之前把 JSON 直接传过去 -> 空 cookie -> 主程序 401 未登录）。
						header := cookieListToHeader(cs)
						logf("login detected, cookie header len=%d", len(header))
						go func() {
							postCookie(cb, header)
							time.Sleep(300 * time.Millisecond)
							w.Terminate()
						}()
					}
				}
			})
		}
	}()

	w.Navigate("https://www.dedao.cn/")
	w.Run()
	logf("window closed, exit")
}

// cookieGatIsid 从 Network.getCookies 返回的 JSON 里提取 GAT/ISID 是否存在。
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

// cookieHasLogin 判断原生 Cookie JSON 里是否已有登录态。
// GAT/ISID 是 dedao 的登录令牌，且为 HttpOnly，只能靠原生读取拿到。
func cookieHasLogin(jsonStr string) bool {
	gat, isid := cookieGatIsid(jsonStr)
	return gat && isid
}

// cookieListToHeader 把 Network.getCookies 返回的 JSON 转成主程序期望的
// "name=value; name=value" 格式。主程序 services.ParseCookies 是按 ';' 和 '='
// 切分的，若直接把 JSON 传过去，GAT/ISID 会解析为空 -> 主程序拿到空登录态 -> 401。
// 这里把全部 cookie 都带上（含 csrfToken 等），保证主程序后端 API 鉴权完整。
func cookieListToHeader(jsonStr string) string {
	var resp struct {
		Cookies []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
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
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

func postCookie(cb, cookie string) {
	if cb == "" {
		logf("no callback url, cookie dropped")
		return
	}
	body := strings.NewReader(cookie)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(cb, "text/plain", body)
	if err != nil {
		logf("postCookie err: %v", err)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	logf("postCookie ok status=%d", resp.StatusCode)
}
