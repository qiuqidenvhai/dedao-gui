// Native WebView2 cookie reader for webview_go.
//
// webview_go 的 JS 注入无法读到 HttpOnly 的登录 Cookie（GAT/ISID），
// 因此这里通过 WebView2 的 DevTools 协议 Network.getCookies 读取全部 Cookie
// （包含 HttpOnly），供登录态检测使用。
//
// 用法（配合 webview.go 里的 Go 封装 PollCookies）：
//   cookie_request(w)  // 异步发起一次读取（同一时刻只允许一个在途请求）
//   cookie_take()      // 若上一次读取已完成，返回全局缓冲区的 char*，否则返回 nullptr
//
// 注意：不要 #include "webview.h"，否则会把 webview 的实现函数重复编进本文件。
// 这里只前向声明需要用到的 webview_get_native_handle。
#include <Windows.h>
#include <WebView2.h>
#include <string.h>
#include <stdlib.h>
#include <cstdio>

extern "C" void *webview_get_native_handle(void *w, int kind);
#define WEBVIEW_NATIVE_HANDLE_KIND_BROWSER_CONTROLLER 2

static char g_cookie_buf[1 << 21]; // 2MB，足够放下所有 cookie 的 JSON
static int g_cookie_ready = 0;
static int g_cookie_pending = 0;

// Network.getCookies 的异步完成回调。WebView2 接口在本头文件里既是 C++ 抽象类
// 也是 C 虚表，直接继承并实现虚函数即可。
class DTPHandler : public ICoreWebView2CallDevToolsProtocolMethodCompletedHandler {
public:
    DTPHandler() {}
    HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **ppv) override {
        (void)riid;
        *ppv = static_cast<ICoreWebView2CallDevToolsProtocolMethodCompletedHandler *>(this);
        return S_OK;
    }
    ULONG STDMETHODCALLTYPE AddRef() override { return 1; }
    ULONG STDMETHODCALLTYPE Release() override { return 1; }
    HRESULT STDMETHODCALLTYPE Invoke(HRESULT errorCode, LPCWSTR resultJson) override {
        g_cookie_buf[0] = 0;
        if (errorCode == S_OK && resultJson) {
            int len = WideCharToMultiByte(CP_UTF8, 0, resultJson, -1,
                                          g_cookie_buf, (int)sizeof(g_cookie_buf),
                                          nullptr, nullptr);
            if (len <= 0) g_cookie_buf[0] = 0;
        }
        g_cookie_pending = 0;
        g_cookie_ready = 1;
        return S_OK;
    }
};

static DTPHandler g_handler;

extern "C" void cookie_request(void *w) {
    if (g_cookie_pending) return;
    void *h = webview_get_native_handle(w, WEBVIEW_NATIVE_HANDLE_KIND_BROWSER_CONTROLLER);
    if (!h) return;
    // 全程空指针/失败检查；WebView2 已初始化时这条链路是安全的，不会崩。
    ICoreWebView2Controller *ctrl = (ICoreWebView2Controller *)h;
    ICoreWebView2 *wv = nullptr;
    if (FAILED(ctrl->get_CoreWebView2(&wv)) || !wv) return;
    // 走 base 接口（ICoreWebView2）的 DevTools 协议，无需 QueryInterface 到 _2，
    // 避免 fork 自带 WebView2.h 与本机运行时版本不一致导致 _2 虚表偏移错位而崩溃。
    g_cookie_pending = 1;
    g_cookie_ready = 0;
    HRESULT hr = wv->CallDevToolsProtocolMethod(L"Network.getCookies", L"{}", &g_handler);
    wv->Release();
    if (FAILED(hr)) {
        // 极端情况下未触发回调，重置标志让下次轮询可重试，避免永久卡在 pending。
        g_cookie_pending = 0;
    }
}

// 返回全局 cookie JSON 缓冲区的指针（若上一次读取已完成），否则返回 nullptr。
// 调用方必须立即用 C.GoString 拷贝走——下一次 cookie_request 会重置状态。
extern "C" char *cookie_take() {
    if (!g_cookie_ready) return nullptr;
    g_cookie_ready = 0;
    return g_cookie_buf;
}

// 顶层异常过滤器：万一 C 层段错误导致进程崩溃，把异常码+地址写进日志，
// 便于在无控制台环境下定位崩溃点（Go 的 recover 抓不住 C 段错误）。
static LONG WINAPI crash_filter(EXCEPTION_POINTERS *ep) {
    char path[MAX_PATH];
    if (GetEnvironmentVariableA("LOCALAPPDATA", path, MAX_PATH)) {
        strncat(path, "\\dedao_login_helper.crash.log", MAX_PATH - strlen(path) - 1);
    } else {
        strncpy(path, "C:\\dedao_login_helper.crash.log", MAX_PATH - 1);
    }
    FILE *f = fopen(path, "a");
    if (f) {
        fprintf(f, "CRASH exception=0x%08X addr=%p\n",
                ep->ExceptionRecord ? ep->ExceptionRecord->ExceptionCode : 0,
                ep->ExceptionRecord ? ep->ExceptionRecord->ExceptionAddress : 0);
        fclose(f);
    }
    return EXCEPTION_EXECUTE_HANDLER;
}

extern "C" void install_crash_handler() {
    SetUnhandledExceptionFilter(crash_filter);
}
