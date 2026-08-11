<template>
  <el-dialog
    v-model="dialogVisible"
    width="420px"
    :before-close="closeDialog"
    :show-close="true"
    destroy-on-close
    center
    class="login-dialog"
  >
    <el-tabs v-model="loginMode" class="login-tabs" stretch @tab-change="handleTabChange">
      <el-tab-pane label="扫码登录" name="qrcode">
        <div class="login-container">
          <div class="login-header">
            <h3 class="title">扫码登录</h3>
            <p class="subtitle">使用得到 App 或微信扫码登录</p>
          </div>

          <div class="qr-container">
            <div class="qr-wrapper">
              <el-image
                v-if="qrData.qrCode"
                class="qr-code-img"
                :src="qrData.qrCode"
                fit="fill"
              />
              <div v-else class="qr-loading">
                <el-icon class="is-loading"><Loading /></el-icon>
              </div>
            </div>
          </div>

          <div class="login-footer">
            <el-image
              src="https://piccdn2.umiwi.com/fe-oss/default/MTYzNzMwNzUyMzQy.png"
              class="app-logo"
            />
            <span class="footer-text">得到 App 扫码登录</span>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="手机号登录" name="phone">
        <div class="phone-login">
          <div class="login-header">
            <h3 class="title">手机号登录</h3>
            <p class="subtitle">在应用内浏览器窗口完成登录</p>
          </div>
          <p class="phone-tip">
            点击下方按钮会打开一个得到官网窗口，易盾滑块在该窗口内可正常加载；
            登录成功后登录态会自动回传，无需手动复制粘贴。
          </p>
          <el-button
            type="primary"
            class="phone-login-button"
            :loading="browserOpening"
            @click="openBrowserLogin"
          >
            打开登录窗口
          </el-button>
        </div>
      </el-tab-pane>
    </el-tabs>
  </el-dialog>
</template>

<script lang="ts" setup>
import { ref, reactive, onMounted, onBeforeUnmount } from "vue";
import { ElMessage } from "element-plus";
import { Loading } from "@element-plus/icons-vue";
import {
  GetQrcode,
  CheckLogin,
  OpenLoginBrowser,
  RefreshHallCache,
} from "../../wailsjs/go/backend/App";
import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime";
import { useRouter } from "vue-router";
import { userStore } from "../stores/user";
import { services } from "../../wailsjs/go/models";
import { Local } from "../utils/storage";

const store = userStore();
const router = useRouter();
const loginMode = ref("qrcode");
const dialogVisible = ref(false);
const qrTimer = ref<number | null>(null);
const countdownTimer = ref<number | null>(null);
const countdown = ref(0);

const qrData = reactive({
  qrCode: "",
  qrCodeString: "",
  token: "",
});

const browserOpening = ref(false);

const emits = defineEmits(["close"]);
const props = defineProps({
  dialogVisible: {
    type: Boolean,
    default: false,
  },
});

onMounted(() => {
  dialogVisible.value = props.dialogVisible;
  loadQrcode();
  EventsOn("login:success", onLoginSuccess);
  EventsOn("login:error", onLoginError);
});

onBeforeUnmount(() => {
  stopQrPolling();
  stopCountdown();
  EventsOff("login:success");
  EventsOff("login:error");
});

const errorMessage = (error: unknown) =>
  error instanceof Error ? error.message : String(error);

const saveLogin = (userData: services.User, cookie: string) => {
  const user = reactive(new services.User());
  Object.assign(user, userData);
  store.user = user;
  Local.set("cookies", cookie);
  // 登录成功后后台刷新大厅商品缓存（下次大厅搜索直接读内存，毫秒级）
  RefreshHallCache();
  if (!store.userList.some((item) => item.uid_hazy === user.uid_hazy)) {
    store.userList.push(user);
  }
  stopQrPolling();
  stopCountdown();
  dialogVisible.value = false;
  emits("close");
  router.push("/user/profile");
};

const loadQrcode = async () => {
  if (loginMode.value !== "qrcode") return;
  stopQrPolling();
  qrData.qrCode = "";
  try {
    const result = await GetQrcode();
    qrData.qrCode = result.qrCode;
    qrData.token = result.token;
    qrData.qrCodeString = result.qrCodeString;
    startQrPolling();
  } catch (error) {
    ElMessage.warning(errorMessage(error));
  }
};

const startQrPolling = () => {
  stopQrPolling();
  qrTimer.value = window.setInterval(async () => {
    if (!qrData.token || !qrData.qrCodeString || loginMode.value !== "qrcode") return;
    try {
      const loginResult = await CheckLogin(qrData.token, qrData.qrCodeString);
      if (loginResult.status === 1 && loginResult.user) {
        saveLogin(loginResult.user, loginResult.cookie);
      } else if (loginResult.status === 2) {
        stopQrPolling();
        ElMessage.warning("二维码已过期，正在刷新");
        loadQrcode();
      }
    } catch {
      // 轮询期间的临时网络错误不打断扫码流程。
    }
  }, 2000);
};

const stopQrPolling = () => {
  if (qrTimer.value !== null) {
    window.clearInterval(qrTimer.value);
    qrTimer.value = null;
  }
};

const handleTabChange = (name: string | number) => {
  if (name === "qrcode") {
    loadQrcode();
  } else {
    stopQrPolling();
  }
};

const startCountdown = () => {
  stopCountdown();
  countdown.value = 60;
  countdownTimer.value = window.setInterval(() => {
    countdown.value -= 1;
    if (countdown.value <= 0) stopCountdown();
  }, 1000);
};

const stopCountdown = () => {
  if (countdownTimer.value !== null) {
    window.clearInterval(countdownTimer.value);
    countdownTimer.value = null;
  }
  if (countdown.value < 0) countdown.value = 0;
};

// 打开应用内原生浏览器窗口，在 dedao.cn 同源页内完成手机号+易盾滑块登录。
const openBrowserLogin = async () => {
  browserOpening.value = true;
  try {
    await OpenLoginBrowser();
    ElMessage.info("已在应用内打开登录窗口，请完成滑块验证并登录");
  } catch (error) {
    ElMessage.warning(errorMessage(error));
  } finally {
    // 窗口是异步登录，这里仅标记本次调用结束；真正登录成功由事件回调处理。
    browserOpening.value = false;
  }
};

// 由后端 OpenLoginBrowser 在浏览器窗口登录成功后触发。
const onLoginSuccess = (data: { user: services.User; cookie: string }) => {
  if (data && data.user) {
    saveLogin(data.user, data.cookie);
    ElMessage.success("登录成功");
  }
};

// 浏览器窗口登录失败（如 Cookie 解析失败）时触发。
const onLoginError = (msg: string) => {
  ElMessage.warning(msg || "登录失败，请重试");
};

const closeDialog = () => {
  stopQrPolling();
  stopCountdown();
  dialogVisible.value = false;
  emits("close");
};
</script>

<style scoped>
.login-tabs {
  margin-top: -10px;
}

.login-container,
.phone-login {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 0 10px;
}

.phone-login {
  padding: 22px 12px 16px;
}

.phone-tip {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.6;
  margin: 0 0 24px;
  text-align: center;
  padding: 0 6px;
}

.login-header {
  text-align: center;
  margin-bottom: 24px;
}

.title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px;
}

.subtitle {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0;
}

.qr-container {
  background: var(--bg-color);
  padding: 20px;
  border-radius: 12px;
  box-shadow: var(--shadow-inner);
  margin-bottom: 24px;
}

.qr-wrapper {
  width: 180px;
  height: 180px;
  background: white;
  padding: 10px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.qr-code-img {
  width: 100%;
  height: 100%;
}

.qr-loading {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
  font-size: 24px;
}

.login-footer {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.app-logo {
  width: 26px;
  height: 26px;
}

.footer-text {
  color: var(--text-secondary);
  font-size: 14px;
}

.phone-login-button {
  width: 100%;
  height: 40px;
}

:deep(.el-dialog) {
  border-radius: 16px;
  overflow: hidden;
  background: var(--card-bg);
  box-shadow: var(--shadow-heavy);
}

:deep(.el-dialog__header) {
  margin: 0;
  padding: 0;
}

:deep(.el-dialog__body) {
  padding: 24px 30px 30px;
}
</style>
