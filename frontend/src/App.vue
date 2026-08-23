<script lang="ts" setup>
import { onMounted, onUnmounted, computed } from 'vue'
import Menu from './components/Menu.vue'
import GlobalAudioPlayer from './components/GlobalAudioPlayer.vue'
import { ElConfigProvider } from 'element-plus'
import 'element-plus/es/components/message/style/css'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import { themeStore } from './stores/theme'
import { settingStore } from './stores/setting'
import { playerStore } from './stores/player'
import { AudioDetailAlias, RefreshHallCache, GetActiveUser, GetSettings, GetDownloadPath } from '../wailsjs/go/backend/App'
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import { setFontFamily } from './utils/utils'
import { userStore } from './stores/user'
import { Local } from './utils/storage'

// 初始化主题
const store = themeStore()
const sStore = settingStore()
const uStore = userStore()

// 启动时从后端配置回填登录态：后端 config（已持久化到 %APPDATA%）是登录态的权威来源，
// Pinia store 重开即清空，必须由后端回填，否则“重新打开后主程序未登录”。
const hydrateUser = async () => {
  try {
    const u = await GetActiveUser()
    if (u && u.uid_hazy) {
      if (!uStore.userList.some((item) => item.uid_hazy === u.uid_hazy)) {
        uStore.userList.push(u)
      }
      uStore.user = u
    }
  } catch (e) {
    // 忽略：未登录或后端暂无用户
  }
}

// 全局监听登录成功（即使登录弹窗已关闭，主程序也能同步登录态）
const onLoginSuccess = (data: { user: any; cookie?: string }) => {
  if (data && data.user && data.user.uid_hazy) {
    if (!uStore.userList.some((item) => item.uid_hazy === data.user.uid_hazy)) {
      uStore.userList.push(data.user)
    }
    uStore.user = data.user
    if (data.cookie) Local.set("cookies", data.cookie)
  }
}

// 启动时从后端配置回填设置（主题色/字体/工具路径等）。
// 主窗口 WebView2 数据目录为每进程临时目录，localStorage 重启即空，
// 所以设置必须依赖后端 config.json 回填，避免“关闭软件后设置丢失”。
const hydrateSettings = async () => {
  try {
    // 下载目录存在后端 config.DownloadPath，不在 SettingsData 里
    const dlDir = await GetDownloadPath()
    sStore.setting.downloadDir = dlDir || ''
    const s = await GetSettings()
    if (!s) return
    // 回填全部设置项到 settingStore
    sStore.setting.theme = s.theme || ''
    sStore.setting.color = s.color || ''
    sStore.setting.ffmpegDir = s.ffmpegDir || ''
    sStore.setting.wkhtmltopdfDir = s.wkhtmltopdfDir || ''
    sStore.setting.fontFamily = s.fontFamily || 'default'
    if (s.color) store.setThemeColor(s.color)
    setFontFamily(s.fontFamily || 'default')
  } catch (e) {
    // 忽略：后端暂无保存值
  }
}

onMounted(() => {
  store.initTheme()
  setFontFamily(sStore.setting.fontFamily || 'default')
  // 启动即回填登录态与设置
  hydrateUser()
  hydrateSettings()
  EventsOn("login:success", onLoginSuccess)
  // 已登录则后台刷新大厅商品缓存（每次重新打开 App 各一次，搜索时直接读内存）
  if (userStore().user) {
    RefreshHallCache()
  }
})

const pStore = playerStore()
const odobCache = new Map<string, { src: string; poster?: string }>()
const odobPending = new Map<string, Promise<{ src: string; poster?: string }>>()

const globalPlayerHeight = computed(() => {
  if (!pStore.hasTrack) return 0
  const h = Number(pStore.barHeight) || 0
  if (h > 0) return h
  return pStore.collapsed ? 76 : 120
})

const mainStyle = computed(() => {
  return { '--global-player-height': `${globalPlayerHeight.value}px` } as any
})

const resolveOdobSrc = async (aliasId: string) => {
  const key = String(aliasId || '').trim()
  if (!key) return { src: '' }
  const cached = odobCache.get(key)
  if (cached) return cached
  const pending = odobPending.get(key)
  if (pending) return pending
  const p = AudioDetailAlias(key)
    .then((detail) => {
      const src = String(detail?.mp3_play_url ?? '').trim()
      const poster = String(detail?.icon ?? '').trim() || undefined
      const val = { src, poster }
      if (src) odobCache.set(key, val)
      return val
    })
    .finally(() => {
      odobPending.delete(key)
    })
  odobPending.set(key, p)
  return p
}

const onResolveTrack = async (ev: any) => {
  const detail = ev?.detail || {}
  const contextKey = String(detail?.contextKey ?? '')
  if (contextKey !== 'odob:study') return
  const trackId = String(detail?.trackId ?? '')
  if (!trackId) return
  const aliasId = trackId.startsWith('odob:') ? trackId.slice(5) : trackId
  if (!aliasId) return
  try {
    const { src, poster } = await resolveOdobSrc(aliasId)
    if (!src) return
    pStore.updateTrackSource(trackId, src, poster)
  } catch {
  }
}

onMounted(() => {
  window.addEventListener('player:resolveTrack', onResolveTrack as any)
})

onUnmounted(() => {
  window.removeEventListener('player:resolveTrack', onResolveTrack as any)
  EventsOff("login:success")
})

// Element Plus 主题配置
const elementTheme = computed(() => store.isDark ? 'dark' : 'light')
</script>

<template>
  <el-config-provider :locale="zhCn" :theme="elementTheme">
    <el-container>
      <el-header>
        <Menu />
      </el-header>
      <el-main :style="mainStyle">
        <router-view></router-view>
      </el-main>
      <GlobalAudioPlayer />
      <!-- <el-footer>Footer</el-footer> -->
    </el-container>
  </el-config-provider>
</template>

<style lang="scss">
@import url("./assets/css/font.css");

body {

  // background-color: transparent;
  img {
    max-width: 100%;
  }
}


// #app {
//   // position: relative;
//   // width: 900px;
//   // height: 520px;
// }

.el-container {
  height: 100%;
  background-color: var(--bg-color);
  transition: background-color 0.3s ease;
}

.el-header {
  position: relative;
  flex-direction: row;
  flex-wrap: nowrap;
  align-items: center;
  justify-content: space-between;
  height: 60px;
  padding: 0 0;
  background-color: var(--bg-color);
  border-bottom: 1px solid var(--border-color-lighter);
  transition: background-color 0.3s ease, border-color 0.3s ease;

  .el-menu {
    height: auto;
    display: flex;
    flex-direction: row;
    flex-wrap: nowrap;
    align-items: center;
    justify-content: flex-start;
    background-color: transparent;
    border-bottom-width: 0px;
    flex: 1;

    .el-menu-item:hover,
    .el-menu-item:active,
    .el-menu-item:focus{
      background:none;
    }

    ul {
      a,
      a:hover,
      a:active,
      a:visited,
      a:link,
      a:focus {
        display: inline-block;
        padding: 0 5px;
        margin-right: 8px;
        text-align: center;
        text-decoration: none;
        white-space: nowrap;
        text-decoration: none;
        color: var(--text-color);
        transition: color 0.3s ease;
      }
    }
  }

}

.el-main {
  overflow: hidden;
  color: var(--text-color-secondary);
  width: 100%;
  height: 100%;
  padding-bottom: calc(var(--global-player-height, 0px) + 10px);
  background-color: var(--bg-color);
  transition: background-color 0.3s ease, color 0.3s ease;

  .el-pagination {
    margin-top: 10px;
    margin-bottom: 10px;
  }

  // 『只要在el-table元素中定义了height属性，即可实现固定表头的表格』不生效解决办法。
  .el-table {
    .el-table__body-wrapper {
      height: calc(100% - 5px) !important; // 表格高度减去表头的高度
    }
  }

  .el-breadcrumb {
    margin-bottom: 15px;
  }


}
</style>
