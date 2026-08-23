<template>
    <div class="hall-container">
        <!-- 搜索框已统一到顶部全局搜索：回车/选择即跳转本页并带入关键词，
             本页只负责展示结果网格与类型筛选，避免两套搜索重复。 -->

        <!-- 搜索结果头部（只显示结果数，不显示关键字） -->
        <div v-if="!initLoading" class="hall-header">
            <span class="hall-count">共 {{ results.length }} 个结果</span>
        </div>

        <!-- 类型筛选（课程 / 听书 / 电子书 分别有多少条，一目了然） -->
        <div v-if="searchKeyword && !initLoading && results.length > 0" class="hall-filter">
            <el-radio-group v-model="activeType" size="small">
                <el-radio-button :value="0">全部 {{ results.length }}</el-radio-button>
                <el-radio-button v-if="countOf(66)" :value="66">课程 {{ countOf(66) }}</el-radio-button>
                <el-radio-button v-if="countOf(13)" :value="13">听书 {{ countOf(13) }}</el-radio-button>
                <el-radio-button v-if="countOf(2)" :value="2">电子书 {{ countOf(2) }}</el-radio-button>
            </el-radio-group>
        </div>

        <div v-loading="initLoading" class="hall-grid-container">
            <div v-if="filteredResults.length > 0" class="hall-grid">
                <div
                    v-for="item in filteredResults"
                    :key="item.enid"
                    class="hall-card"
                    @click="openDetail(item)"
                >
                    <!-- 悬停遮罩层（仅非分组显示操作） -->
                    <div v-if="!item.is_group" class="hall-card-overlay" @click.stop>
                        <div class="hall-overlay-actions">
                            <el-button
                                v-if="canDownload(item)"
                                circle
                                type="success"
                                :icon="Download"
                                title="下载"
                                @click.stop="openDownloadDialog(item)"
                            />
                        </div>
                    </div>
                    <div class="hall-card-cover">
                        <el-image
                            v-if="item.icon"
                            :src="item.icon"
                            fit="cover"
                            loading="lazy"
                        >
                            <template #placeholder>
                                <div class="hall-image-placeholder">
                                    <el-icon><Picture /></el-icon>
                                </div>
                            </template>
                            <template #error>
                                <div class="hall-image-placeholder">
                                    <el-icon><Picture /></el-icon>
                                </div>
                            </template>
                        </el-image>
                        <div v-else class="hall-no-cover">
                            <el-icon><Picture /></el-icon>
                        </div>
                        <div class="hall-card-type" v-if="typeName(item)">{{ typeName(item) }}</div>
                    </div>
                    <div class="hall-card-body">
                        <h3 class="hall-card-title" :title="item.title">{{ item.title }}</h3>
                        <div class="hall-card-author" v-if="item.author">{{ item.author }}</div>
                        <p class="hall-card-intro" v-if="item.intro">{{ item.intro }}</p>
                        <div class="hall-card-price" v-if="item.price && item.price !== '0.00'">
                            <span class="price-now">¥{{ item.price }}</span>
                            <span class="price-origin" v-if="item.product_price && item.product_price > 0">
                                原价 ¥{{ item.product_price }}
                            </span>
                        </div>
                    </div>
                </div>
            </div>
            <el-empty v-else-if="!initLoading" :description="searchKeyword ? '大厅中未找到相关内容' : '输入关键词开始大厅搜索'" />
        </div>

        <!-- 详情弹窗（按 enid 打开，大厅结果无 id 也能看；带上类型让书架按钮走对接口） -->
        <course-info
            v-if="detailVisible"
            :enid="detailEnid"
            :product-type="detailType"
            :dialog-visible="detailVisible"
            @close="closeDetail"
        />
    </div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, onBeforeRouteUpdate } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Picture, View, Download } from '@element-plus/icons-vue'
import { HallSearch, BatchCourseDownload, BatchEbookDownload, BatchOdobDownload, AudioDetailAlias } from '../../wailsjs/go/backend/App'
import { app, services } from '../../wailsjs/go/models'
import CourseInfo from '../components/CourseInfo.vue'

const route = useRoute()
const searchKeyword = ref('')
const results = ref<services.Course[]>([])
const loading = ref(false)
const initLoading = ref(true)
const detailVisible = ref(false)
const detailEnid = ref('')
const detailType = ref(0)
const activeType = ref(0)

// 商品类型（服务端真实取值）：2 电子书 / 13 听书 / 66 课程
const typeName = (item: services.Course): string => {
    switch (item.type) {
        case 2: return '电子书'
        case 3:
        case 13: return '听书'
        case 4: return '视频'
        case 1:
        case 66: return '课程'
        default: return ''
    }
}


// 归一化类型，供筛选统计用（听书历史上有 3/13 两种取值）
const normType = (t: number): number => {
    if (t === 3 || t === 13) return 13
    if (t === 2) return 2
    return 66
}

const countOf = (t: number): number =>
    results.value.filter(i => normType(Number(i.type)) === t).length

const canDownload = (item: services.Course): boolean => {
    return !!item.enid && !item.is_group
}

const filteredResults = computed(() => {
    if (!activeType.value) return results.value
    return results.value.filter(i => normType(Number(i.type)) === activeType.value)
})

const loadData = async () => {
    const kw = (searchKeyword.value || '').trim()
    loading.value = true
    initLoading.value = true
    if (!kw) {
        results.value = []
        loading.value = false
        initLoading.value = false
        return
    }
    try {
        // 走后端 HallSearch：内部并发调用得到官方的课程/听书/电子书三个分页搜索接口，
        // 再叠加本地大厅缓存补充，结果完整（不会只出听书）
        const list = await HallSearch(kw, 240)
        results.value = list || []
        activeType.value = 0
    } catch (error) {
        console.error('大厅搜索失败:', error)
        ElMessage({ message: '大厅搜索失败: ' + String(error), type: 'warning' })
        results.value = []
    } finally {
        loading.value = false
        initLoading.value = false
    }
}

// 搜索已由顶部全局搜索统一驱动：回车/选择会跳转到本页并带入 keyword，
// 由 onBeforeRouteUpdate / onMounted 触发刷新。
onMounted(() => {
    const kw = (route.query.keyword as string) || ''
    searchKeyword.value = kw
    loadData()
})

// 顶部搜索跳转过来（query 变化）或点击历史回退时刷新
onBeforeRouteUpdate((to, _from) => {
    const kw = (to.query.keyword as string) || ''
    if (kw === searchKeyword.value) return
    searchKeyword.value = kw
    results.value = []
    loadData()
})

const openDetail = (item: services.Course) => {
    if (!item.enid) return
    detailEnid.value = item.enid
    detailType.value = normType(Number(item.type))
    detailVisible.value = true
}
const closeDetail = () => {
    detailVisible.value = false
    detailEnid.value = ''
    detailType.value = 0
}

const openDownloadDialog = async (item: services.Course) => {
    if (!item.enid) {
        ElMessage.warning('无法获取课程信息，下载失败')
        return
    }
    try {
        // 根据类型选择下载函数
        if (item.type === 2) {
            // 电子书
            await BatchEbookDownload([{ id: item.id, enid: item.enid, title: item.title }], 1)
        } else if (item.type === 13 || item.type === 3) {
            // 听书 - 需要先获取音频详情
            try {
                const audioDetail = await AudioDetailAlias(item.enid)
                const odobItem = new app.OdobItem({
                    ID: item.id,
                    Enid: item.enid,
                    Title: item.title,
                    AudioDetail: audioDetail
                })
                await BatchOdobDownload([odobItem], 1)
            } catch (audioErr) {
                // 如果获取音频详情失败，仍然尝试下载
                const odobItem = new app.OdobItem({
                    ID: item.id,
                    Enid: item.enid,
                    Title: item.title
                })
                await BatchOdobDownload([odobItem], 1)
            }
        } else {
            // 课程类型 (66, 4, 1)
            await BatchCourseDownload([{ id: 0, aid: 0, enid: item.enid, title: item.title }], 1)
        }
        ElMessage.success('已添加到下载队列')
    } catch (error) {
        ElMessage.error('下载失败: ' + String(error))
    }
}
</script>

<style scoped>
.hall-container {
    padding: 20px 24px;
    min-height: 100%;
}
.hall-search-bar {
    max-width: 720px;
    margin: 0 auto 20px;
}
.hall-input :deep(.el-input__wrapper) {
    border-radius: 20px;
    box-shadow: none;
    border: 1px solid var(--border-color);
}
.hall-input :deep(.el-input__wrapper.is-focus) {
    border-color: var(--accent-color);
    box-shadow: 0 0 0 2px rgba(255, 107, 0, 0.1);
}
.hall-header {
    max-width: 1200px;
    margin: 0 auto 16px;
    font-size: 15px;
    color: var(--text-primary);
}
.hall-keyword {
    color: var(--accent-color);
    font-weight: 600;
}
.hall-count {
    margin-left: 12px;
    color: var(--text-secondary, #909399);
    font-size: 13px;
}
.hall-filter {
    max-width: 1200px;
    margin: 0 auto 14px;
}
.hall-grid-container {
    max-width: 1200px;
    margin: 0 auto;
    min-height: 300px;
}
.hall-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 18px;
}
.hall-card {
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    overflow: hidden;
    cursor: pointer;
    transition: transform .15s ease, box-shadow .15s ease;
    display: flex;
    flex-direction: column;
}
.hall-card:hover {
    transform: translateY(-4px);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
    border-color: var(--accent-color);
}
.hall-card-cover {
    position: relative;
    width: 100%;
    aspect-ratio: 1 / 1;
    background: var(--bg-secondary, #f5f7fa);
}
.hall-card-cover .el-image {
    width: 100%;
    height: 100%;
}
.hall-image-placeholder,
.hall-no-cover {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #c0c4cc;
    font-size: 40px;
}
.hall-card-type {
    position: absolute;
    top: 8px;
    left: 8px;
    background: rgba(0, 0, 0, 0.6);
    color: #fff;
    font-size: 12px;
    padding: 2px 8px;
    border-radius: 4px;
}
.hall-card-body {
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1;
}
.hall-card-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.4;
}
.hall-card-author {
    font-size: 12px;
    color: var(--text-secondary, #909399);
}
.hall-card-intro {
    font-size: 12px;
    color: var(--text-secondary, #909399);
    margin: 0;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.4;
}
.hall-card-price {
    margin-top: auto;
    display: flex;
    align-items: baseline;
    gap: 8px;
}
.price-now {
    color: var(--accent-color);
    font-size: 15px;
    font-weight: 600;
}
.price-origin {
    color: #c0c4cc;
    font-size: 12px;
    text-decoration: line-through;
}
.hall-card-tag {
    align-self: flex-start;
    font-size: 11px;
    color: var(--accent-color);
    border: 1px solid var(--accent-color);
    border-radius: 4px;
    padding: 1px 6px;
}
</style>
