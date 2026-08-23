package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/yann0917/dedao-gui/backend"

	"github.com/wailsapp/wails/v2/pkg/logger"
	_ "github.com/yann0917/dedao-gui/backend/config"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	// 任何启动期 panic 都不再静默崩溃，而是弹窗+写日志
	defer func() {
		if r := recover(); r != nil {
			showFatal("dedao-gui 发生异常", fmt.Sprintf("%v", r))
		}
	}()

	// 将标准 log 包输出重定向到 dedao_error.log（与 Trace 用同一路径）
	// Windows GUI 应用没有控制台，log.Printf 直接写文件才能看到调试输出
	var logPath string
	if exe, err := os.Executable(); err == nil {
		logPath = filepath.Join(filepath.Dir(exe), "dedao_error.log")
	} else if d, err := os.UserCacheDir(); err == nil {
		logPath = filepath.Join(d, "dedao_error.log")
	}
	if logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			log.SetOutput(f)
			log.SetFlags(log.Ltime | log.Lmsgprefix)
			log.SetPrefix("[LOG] ")
			log.Println("日志已重定向到:", logPath)
		}
	}

	backend.Trace("main: 程序入口，准备 wails.Run")
	app := backend.NewApp()

	// 主窗口 WebView2 用户数据目录：每进程唯一临时目录。
	// 之前改成持久共享目录（%APPDATA%/dedao-gui/webview）后，多次启动会抢同一 WebView2
	// 数据目录，导致导航卡死、前端页面加载不出来（OnDomReady 不触发，表现为“打不开/白窗”）。
	// 改回每进程独立临时目录可彻底避免目录争抢；需要持久化的状态（登录态、设置项）统一由
	// 后端 config.json 持久化，不再依赖 WebView2 的 localStorage。
	webviewUDD := filepath.Join(os.TempDir(), fmt.Sprintf("dedao-gui-webview-%d", os.Getpid()))

	err := wails.Run(&options.App{
		Title:     "dedao-gui",
		Width:     1280,
		Height:    1000,
		MinWidth:  1024,
		MinHeight: 768,
		MaxWidth:  2560,
		MaxHeight: 1440,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:   &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:          app.Startup,
		OnShutdown:         app.Shutdown,
		OnDomReady:         app.DomReady,
		LogLevel:           logger.DEBUG,
		LogLevelProduction: logger.ERROR,
		WindowStartState:   options.Normal,
		Bind: []interface{}{
			app,
		},
		ErrorFormatter: func(err error) any { return err.Error() },
		// 单实例锁已移除：残留/卡死的旧进程会悄悄让新进程退出，表现为"打不开"。
		// Windows 平台 specific options
		Windows: &windows.Options{
			// WebviewIsTransparent:              true,
			// WindowIsTranslucent:               false,
			// DisableWindowIcon:                 false,
			// DisableFramelessWindowDecorations: false,
			WebviewUserDataPath: webviewUDD,
			Theme:               windows.SystemDefault,
			CustomTheme: &windows.ThemeSettings{
				DarkModeTitleBar:   windows.RGB(20, 20, 20),
				DarkModeTitleText:  windows.RGB(200, 200, 200),
				DarkModeBorder:     windows.RGB(20, 0, 20),
				LightModeTitleBar:  windows.RGB(200, 200, 200),
				LightModeTitleText: windows.RGB(20, 20, 20),
				LightModeBorder:    windows.RGB(200, 200, 200),
			},
		},
		// Mac platform specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			Appearance:           mac.DefaultAppearance,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "dedao gui downloader",
				Message: "https://github.com/yann0917/dedao-gui",
				Icon:    icon,
			},
		},
		Linux: &linux.Options{
			Icon: icon,
		},
		// 生产构建不要自动打开 DevTools 检查器（调试遗留项，会导致每次启动弹调试窗）
		Debug: options.Debug{
			OpenInspectorOnStartup: false,
		},
	})

	// wails.Run 返回的错误（如 WebView2 运行时缺失）也弹窗，不再静默退出
	if err != nil {
		showFatal("dedao-gui 启动失败", err.Error())
	}
}

// showFatal 把致命错误弹窗显示，并写入 exe 同目录（或缓存目录）的 dedao_error.log
func showFatal(title, msg string) {
	detail := fmt.Sprintf("%s\n\n%s\n\n%s", title, msg, debug.Stack())
	if p := errorLogPath(); p != "" {
		_ = os.WriteFile(p, []byte(detail), 0644)
	}
	user32 := syscall.NewLazyDLL("user32.dll")
	mb := user32.NewProc("MessageBoxW")
	mb.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(detail))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("dedao-gui 错误"))),
		0,
	)
}

func errorLogPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "dedao_error.log")
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "dedao_error.log")
	}
	return "dedao_error.log"
}
