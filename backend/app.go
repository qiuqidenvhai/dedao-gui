package backend

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/yann0917/dedao-gui/backend/config"
	"github.com/yann0917/dedao-gui/backend/downloader"
	"github.com/yann0917/dedao-gui/backend/services"
	"github.com/yann0917/dedao-gui/backend/utils"
)

// App struct
type App struct {
	Ctx       context.Context
	dlManager *DownloadManagerWrapper // 并行下载管理器（延迟初始化）
}

// DownloadManagerWrapper 包装 downloader.DownloadManager，提供懒初始化和安全访问
type DownloadManagerWrapper struct {
	mgr *downloader.DownloadManager
	mu  sync.Mutex
}

func (w *DownloadManagerWrapper) Get(ctx context.Context) *downloader.DownloadManager {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.mgr == nil {
		w.mgr = downloader.NewDownloadManager(ctx, 5) // 5 个并发工作线程
	}
	return w.mgr
}

func (w *DownloadManagerWrapper) Shutdown() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.mgr != nil {
		w.mgr.Cancel()
		w.mgr.Wait()
		w.mgr = nil
	}
}

// Trace 记录启动里程碑到 dedao_error.log，用于诊断"打不开"到底卡在哪一步
func Trace(msg string) {
	p := traceLogPath()
	if p == "" {
		return
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

func traceLogPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "dedao_error.log")
	}
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "dedao_error.log")
	}
	return ""
}

// NewApp creates a new App application struct
func NewApp() *App {
	Trace("NewApp: 后端实例已创建")
	// dlManager 必须在这里建好：BatchCourseDownload / BatchEbookDownload 直接调用
	// a.dlManager.Get(...)，如果它是 nil，nil 接收者进 Get() 第一行 w.mu.Lock() 就
	// 空指针 panic，整个进程退出（表现为点批量下载后程序直接没了）。
	return &App{dlManager: &DownloadManagerWrapper{}}
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.Ctx = ctx

	// 启动即检查登录令牌是否过期。
	// 得到的 GAT 是 JWT，exp 就是过期时间；过期后所有需登录接口都会 401，
	// 界面就会停在「看着像已登录、数据全空」的状态。
	// 这里直接清掉登录数据（走 Logout：清内存态 + 删配置文件 + 重建匿名会话），
	// 程序落到登录页，用户重新扫码/手机号登录即可。
	// 注意：必须真的清数据，而不是只跳页面——否则登录页仍带着失效令牌，二维码刷不出来。
	if u := config.Instance.ActiveUser(); u != nil && u.CookieOptions.GAT != "" {
		if services.GATExpired(u.CookieOptions.GAT) {
			Trace("Startup: 登录令牌已过期，清除登录数据以便重新登录")
			if err := a.ClearExpiredLogin(); err != nil {
				Trace("Startup: 清除登录数据失败: " + err.Error())
			} else {
				Trace("Startup: 登录数据已清除（设置项保留）")
			}
		}
	}

	Trace("OnStartup: 后端就绪，窗口上下文已建立")
}

func (a *App) Shutdown(ctx context.Context) {
	if a.dlManager != nil {
		a.dlManager.Shutdown()
	}
	setupCleanupOnExit()
}

func (a *App) DomReady(ctx context.Context) {
	Trace("OnDomReady: 前端页面加载完成（窗口应已可见）")
}

func (a *App) OnSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	Trace("OnSecondInstanceLaunch: 检测到第二个实例启动")
	fmt.Println("OnSecondInstanceLaunch", secondInstanceData)
}

func setupCleanupOnExit() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Println("正在关闭程序...")

		// 获取 BadgerDB 实例并关闭
		db, err := utils.GetBadgerDB(utils.GetDefaultBadgerDBPath())
		if err == nil && db != nil {
			if err := db.Close(); err != nil {
				fmt.Printf("关闭数据库时出错: %v\n", err)
			} else {
				fmt.Println("数据库已安全关闭")
			}
		}

		os.Exit(0)
	}()
}
