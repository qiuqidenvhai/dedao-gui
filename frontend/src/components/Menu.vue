<template>
    <div class="menu-container">
        <!-- 搜索框 -->
        <GlobalSearch />

        <el-menu router :default-active="activeIndex" class="el-menu" mode="horizontal"
             text-color="var(--text-primary)"
             active-text-color="var(--accent-color)"
             @select="handleSelect" :collapse-transition="false">
            <menu-item v-for="(menu, key) in allRoutes" :key="key" :menu="menu" :path="menu.path" />
        </el-menu>

        <!-- 右侧用户/主题区 -->
        <div class="right-area">
            <!-- 未登录：显示登录按钮 -->
            <el-button
                v-if="!isLoggedIn"
                type="primary"
                size="small"
                :icon="User"
                @click="goLogin"
                class="login-btn"
            >
                登录
            </el-button>

            <!-- 已登录：显示当前用户和退出 -->
            <div v-else class="user-area">
                <span class="nickname" @click="goProfile">{{ userSt.user?.nickname || '用户' }}</span>
                <el-button
                    type="danger"
                    size="small"
                    plain
                    :icon="SwitchButton"
                    @click="handleLogout"
                    class="logout-mini-btn"
                >
                    退出
                </el-button>
            </div>

            <!-- 主题切换按钮 -->
            <div class="theme-switch-container">
                <el-switch
                    v-model="isDark"
                    :active-action-icon="Moon"
                    :inactive-action-icon="Sunny"
                    inline-prompt
                    @change="toggleDark"
                    class="theme-switch"
                />
                <span style="margin-left: 8px; color: var(--text-primary); font-size: 12px;">
                    {{ isDark ? '暗色' : '亮色' }}
                </span>
            </div>
        </div>
    </div>
</template>

<script lang="ts" setup>
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Moon, Sunny, User, SwitchButton } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import MenuItem from './MenuItem.vue'
import GlobalSearch from './GlobalSearch.vue'
import { themeStore } from '../stores/theme'
import { userStore } from '../stores/user'
import { WindowReloadApp } from '../../wailsjs/runtime'

const route = useRoute()
const router = useRouter()

// 主题相关
const store = themeStore()
const isDark = computed(() => store.isDark)

const toggleDark = () => {
    store.toggleTheme()
}

// 用户相关
const userSt = userStore()
const isLoggedIn = computed(() => !!userSt.user?.nickname)

const goLogin = () => {
    router.push('/user/login')
}

const goProfile = () => {
    router.push('/user/profile')
}

const handleLogout = async () => {
    try {
        await userSt.logout()
    } catch (e) {
        console.error(e)
        ElMessage.error('退出失败')
    }
}

const allRoutes = router.options.routes.filter(route =>
    route.meta?.name !== '主题' // 过滤掉主题页面
)
const activeIndex = computed(() => {
    return route.path
})
const handleSelect = (key: string, keyPath: string[]) => {
    // console.log(key, keyPath)
}
</script>

<style scoped>
.menu-container {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
}

.right-area {
    display: flex;
    align-items: center;
    margin-left: auto;
    padding: 0 16px;
    gap: 12px;
}

.user-area {
    display: flex;
    align-items: center;
    gap: 8px;
}

.nickname {
    color: var(--text-primary);
    font-size: 14px;
    cursor: pointer;
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.nickname:hover {
    color: var(--accent-color);
}

.logout-mini-btn {
    --el-button-size: 24px;
}

.theme-switch-container {
    display: flex;
    align-items: center;
    min-width: 88px;
    justify-content: flex-end;
}

.theme-switch {
    --el-switch-on-color: var(--accent-color);
    --el-switch-off-color: var(--border-soft);
    --el-switch-border-color: var(--border-soft);
}

.theme-switch :deep(.el-switch__core) {
    background-color: var(--card-bg);
    border-color: var(--border-soft);
    transition: all 0.3s ease;
}

.theme-switch :deep(.el-switch__core:hover) {
    border-color: var(--accent-color);
}

.theme-switch :deep(.el-switch__action) {
    background-color: var(--card-bg);
    color: var(--text-primary);
    transition: all 0.3s ease;
}

.theme-switch :deep(.el-switch__action:hover) {
    color: var(--accent-color);
}

/* 暗色模式下的主题切换按钮 */
.theme-dark .theme-switch :deep(.el-switch__core) {
    background-color: var(--card-bg) !important;
    border-color: var(--border-soft) !important;
}

.theme-dark .theme-switch :deep(.el-switch__core:hover) {
    border-color: var(--accent-color) !important;
}

.theme-dark .theme-switch :deep(.el-switch__action) {
    background-color: var(--card-bg) !important;
    color: var(--text-primary) !important;
}

.theme-dark .theme-switch :deep(.el-switch__action:hover) {
    color: var(--accent-color) !important;
}

/* 菜单样式适配 */
.el-menu {
    background-color: transparent !important;
    border-bottom: none !important;
}

.el-menu :deep(.el-menu-item) {
    color: var(--text-primary) !important;
    background-color: transparent !important;
    border-bottom: 2px solid transparent !important;
    transition: all 0.3s ease !important;
}

.el-menu :deep(.el-menu-item:hover) {
    color: var(--accent-color) !important;
    background-color: var(--card-hover-bg) !important;
    border-bottom-color: var(--accent-color) !important;
}

.el-menu :deep(.el-menu-item.is-active) {
    color: var(--accent-color) !important;
    background-color: var(--card-hover-bg) !important;
    border-bottom-color: var(--accent-color) !important;
    font-weight: 500 !important;
}

.el-menu :deep(.el-sub-menu__title) {
    color: var(--text-primary) !important;
    background-color: transparent !important;
    border-bottom: 2px solid transparent !important;
    transition: all 0.3s ease !important;
}

.el-menu :deep(.el-sub-menu__title:hover) {
    color: var(--accent-color) !important;
    background-color: var(--card-hover-bg) !important;
    border-bottom-color: var(--accent-color) !important;
}

/* 暗色主题下的菜单样式 */
.theme-dark .el-menu {
    background-color: transparent !important;
    border-bottom: none !important;
}

.theme-dark .el-menu :deep(.el-menu-item) {
    color: var(--text-primary) !important;
    background-color: transparent !important;
    border-bottom: 2px solid transparent !important;
}

.theme-dark .el-menu :deep(.el-menu-item:hover) {
    color: var(--accent-color) !important;
    background-color: var(--card-hover-bg) !important;
    border-bottom-color: var(--accent-color) !important;
}

.theme-dark .el-menu :deep(.el-menu-item.is-active) {
    color: var(--accent-color) !important;
    background-color: var(--card-hover-bg) !important;
    border-bottom-color: var(--accent-color) !important;
    font-weight: 500 !important;
}

.theme-dark .el-menu :deep(.el-sub-menu__title) {
    color: var(--text-primary) !important;
    background-color: transparent !important;
    border-bottom: 2px solid transparent !important;
}

.theme-dark .el-menu :deep(.el-sub-menu__title:hover) {
    color: var(--accent-color) !important;
    background-color: var(--card-hover-bg) !important;
    border-bottom-color: var(--accent-color) !important;
}
</style>
