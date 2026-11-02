import { userStore } from '../stores/user'
import router from '../router'
import { ClearExpiredLogin } from '../../wailsjs/go/backend/App'

/**
 * 统一处理「登录令牌失效」。
 *
 * 后端对过期/无效令牌统一返回 `401 Unauthorized`（services.handleHTTPResponse）。
 * 前端以前只有 Course / Ebook / Compass / Odob 四个页面判断这个字符串，
 * 首页与文章列表是直接 console.log 吞掉的，于是令牌过期后会停在
 * 「看着像已登录、数据全空、也不跳登录页」的状态。
 *
 * 处理方式：**先清掉登录数据，再跳登录页**。
 * 只跳页面不清数据的话，登录页仍带着失效令牌，二维码会一直转圈刷不出来。
 * 这里调后端 ClearExpiredLogin（清空用户/令牌并持久化，保留下载目录与设置），
 * 而不是 Logout（Logout 会连整个 config.json 一起删掉，设置没了）。
 *
 * 用法：在 catch 里第一句调用，返回 true 表示已按鉴权失败处理，调用方直接 return。
 */
export async function handleAuthError(error: unknown): Promise<boolean> {
  if (String(error ?? '').trim() !== '401 Unauthorized') return false

  const store = userStore()
  store.user = null
  store.userList = []

  try {
    await ClearExpiredLogin()
  } catch (e) {
    console.warn('清理过期登录数据失败:', e)
  }

  try {
    await router.push('/user/login')
  } catch (e) {
    console.warn('跳转登录页失败:', e)
  }
  return true
}
