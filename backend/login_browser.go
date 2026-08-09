package backend

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yann0917/dedao-gui/backend/app"
)

// loginSrv / loginHelperCmd 仅用于清理，避免重复拉起与资源泄漏。
var loginSrv *http.Server
var loginHelperCmd *exec.Cmd

// OpenLoginBrowser 拉起一个独立进程 login_helper.exe，打开指向 dedao.cn 的原生
// 浏览器窗口（与 dedao.cn 同源，易盾滑块可正常渲染）。用户在窗口内手机号+滑块登录后，
// login_helper 会把登录态 Cookie 通过本地 HTTP 回调 POST 回本进程，再由后端解析完成登录。
//
// 这样做的好处：
//   - webview_go 必须在主线程运行，独立进程有自己的主线程，不会出现白屏；
//   - 关闭登录窗口只会退出 login_helper 进程，绝不会拖垮主程序。
//
// 完成后通过 Wails 事件通知前端：
//   - "login:success" -> { user, cookie }
//   - "login:error"   -> string(错误信息)
func (a *App) OpenLoginBrowser() error {
	// 0) 清理上一次可能残留的实例（避免重复拉起 / 监听端口泄漏）
	if loginHelperCmd != nil && loginHelperCmd.Process != nil {
		_ = loginHelperCmd.Process.Kill()
		loginHelperCmd = nil
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

	mux := http.NewServeMux()
	mux.HandleFunc("/cb", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		cookie := string(raw)
		w.WriteHeader(http.StatusNoContent)
		// 异步处理，避免阻塞 HTTP 服务关闭
		go a.finishLoginFromCookie(cookie)
		// 捕获当前 server 引用，避免快速重复打开时误关新的回调服务
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

	// 2) 定位 login_helper.exe（与主程序 exe 同目录）
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位主程序路径: %v", err)
	}
	helperPath := filepath.Join(filepath.Dir(exePath), "login_helper.exe")
	if _, err := os.Stat(helperPath); err != nil {
		return fmt.Errorf("找不到登录窗口组件 login_helper.exe（应在 %s）", helperPath)
	}

	// 3) 拉起独立进程
	cmd := exec.Command(helperPath, cbURL)
	cmd.Dir = filepath.Dir(exePath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动登录窗口失败: %v", err)
	}
	loginHelperCmd = cmd

	// helper 进程退出（用户手动关窗，或登录完成被 Kill）时，回收回调服务。
	go func() {
		_ = cmd.Wait()
		if loginSrv != nil {
			_ = loginSrv.Shutdown(context.Background())
			loginSrv = nil
		}
	}()
	return nil
}

// finishLoginFromCookie 解析 Cookie 完成登录并通知前端，随后清理 helper 进程。
func (a *App) finishLoginFromCookie(cookie string) {
	cookie = sanitizeCookie(cookie)
	if cookie == "" {
		runtime.EventsEmit(a.Ctx, "login:error", "未能获取登录态")
		return
	}
	user, err := app.LoginByCookie(cookie)
	if err != nil || user == nil {
		msg := "登录失败：未能解析登录态"
		if err != nil {
			msg = err.Error()
		}
		runtime.EventsEmit(a.Ctx, "login:error", msg)
		return
	}
	// 关闭 helper 进程（若仍在运行）
	if loginHelperCmd != nil && loginHelperCmd.Process != nil {
		_ = loginHelperCmd.Process.Kill()
		loginHelperCmd = nil
	}
	runtime.EventsEmit(a.Ctx, "login:success", map[string]interface{}{
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
