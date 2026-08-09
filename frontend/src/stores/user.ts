import { ref, computed, reactive } from "vue";
import { defineStore } from "pinia";
import { services } from '../../wailsjs/go/models'
import { Logout } from '../../wailsjs/go/backend/App'
import { WindowReloadApp } from '../../wailsjs/runtime'
import { Local } from '../utils/storage'
import router from '../router'

export const userStore = defineStore("userStore",  {
    state:() =>{
        return {
            userList:[] as services.User[],
            user:null as services.User | null
        }
    },
    actions: {
        async logout() {
            try {
                await Logout()
            } catch (e) {
                console.error('Backend logout failed:', e)
            }
            this.user = null
            this.userList = []
            Local.remove("cookies")
            Local.remove("userStore")
            // 显式跳转到登录页面
            try {
                await router.push('/user/login')
            } catch (e) {
                console.warn('router.push failed:', e)
            }
            // 重新加载窗口，确保所有组件重新初始化
            try {
                WindowReloadApp()
            } catch (error) {
                console.warn('WindowReloadApp failed:', error)
            }
        },
        isLoggedIn(): boolean {
            return !!this.user?.nickname
        }
    },
    persist: true,
});
