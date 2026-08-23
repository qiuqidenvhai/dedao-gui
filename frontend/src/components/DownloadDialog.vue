<template>
    <el-dialog 
        v-model="dialogVisible" 
        title="下载选项" 
        align-center 
        center 
        width="560px" 
        :before-close="closeDialog"
        class="custom-download-dialog"
    >
        <div class="download-container">
            <div class="format-selector">
                <div class="section-label">
                    选择导出格式
                    <span v-if="isBatchMode" class="batch-info">(批量下载 {{ batchCount }} 个)</span>
                </div>

                <div class="format-options">
                    <div 
                        v-for="item in props.downloadTypeOptions" 
                        :key="item.value" 
                        class="format-option"
                        :class="downloadType === item.value ? 'active' : ''"
                        @click="downloadType = item.value"
                    >
                        <span class="format-text">{{ item.label }}</span>
                        <el-icon v-if="downloadType === item.value" class="selected-icon"><Check /></el-icon>
                    </div>
                </div>
            </div>

            <!-- 总体进度 -->
            <div v-if="percentage > 0 || isDownloading" class="download-status">
                <div class="status-header">
                    <span class="status-text">{{ content }}</span>
                    <span class="status-percent">{{ percentage }}%</span>
                </div>
                <el-progress 
                    :percentage="percentage"
                    :stroke-width="8"
                    :show-text="false"
                    status="success"
                    class="custom-progress"
                />
                <div v-if="speedText" class="speed-info">
                    <span>{{ speedText }}</span>
                    <span v-if="etaText" class="eta">剩余 {{ etaText }}</span>
                </div>
            </div>

            <!-- 任务列表（批量模式） -->
            <div v-if="isBatchMode && taskList.length > 0" class="task-list">
                <div class="section-label" style="margin-top:16px;">下载任务</div>
                <div class="task-list-inner">
                    <div 
                        v-for="task in taskList" 
                        :key="task.task_id" 
                        class="task-item"
                        :class="{ 'task-done': task.status === 2, 'task-error': task.status === 4 }"
                    >
                        <div class="task-info">
                            <el-icon class="task-icon" :class="statusIconClass(task.status)">
                                <component :is="statusIcon(task.status)" />
                            </el-icon>
                            <span class="task-title" :title="task.title">{{ task.title }}</span>
                        </div>
                        <div class="task-progress-area">
                            <el-progress 
                                :percentage="task.pct" 
                                :stroke-width="4" 
                                :show-text="false"
                                :status="progressStatus(task.status)"
                                style="width:120px;"
                            />
                            <span class="task-pct">{{ task.pct }}%</span>
                            <span v-if="task.speed_bps > 0" class="task-speed">{{ formatSpeed(task.speed_bps) }}</span>
                            <el-button 
                                v-if="task.status === 1 || task.status === 0" 
                                size="small" 
                                text 
                                type="danger"
                                @click="cancelTask(task.task_id)"
                            >取消</el-button>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <template #footer>
            <div class="dialog-footer">
                <el-button @click="closeDialog" :disabled="isDownloading && !allowClose">取消</el-button>
                <el-button type="primary" @click="download()" :loading="isDownloading"> 
                    {{ isDownloading ? '下载中...' : '开始下载' }}
                </el-button>
            </div>
        </template>
    </el-dialog>
</template>

<script lang="ts" setup>
import {onMounted, ref, computed, PropType, onUnmounted} from "vue";
import {EbookDownload, CourseDownload, OdobDownload, BatchCourseDownload, BatchEbookDownload, BatchOdobDownload} from "../../wailsjs/go/backend/App";
import {ElMessage} from "element-plus";
import { EventsOn, EventsOff} from "../../wailsjs/runtime/runtime";
import { Check, Loading, CircleCheckFilled, CircleCloseFilled, Warning } from '@element-plus/icons-vue'

// 状态常量（对应后端 TaskStatus）
const STATUS = {
    PENDING: 0,
    RUNNING: 1,
    PAUSED: 2,
    COMPLETED: 3,
    FAILED: 4,
    CANCELLED: 5,
}

let percentage = ref(0)
let content = ref('')
const isDownloading = ref(false)
const speedText = ref('')
const etaText = ref('')
const allowClose = ref(true)
const taskList = ref<any[]>([])

const dialogVisible = ref(false)
const downloadType = ref(1)

const props = defineProps({
    downloadId:{
        type:Number,
        required:true,
        default:0,
    },
    enId:{
        type:String,
        default:'',
    },
    prodType:{
        type:Number,
        required:true,
        default:0,
    },
    articleId:{
        type:Number,
        default:0,
    },
    dialogVisible: {
        type: Boolean,
        default: false,
    },
    downloadTypeOptions:{
        type: Array as PropType<Array<{value: number, label: string}>>,
        required:true,
        default:() => []
    },
    downloadData: {
        type: [Object, Array],
        default: () => ({})
    }
});
const emits = defineEmits(["close"]);

const isBatchMode = computed(() => {
    return Array.isArray(props.downloadData) && props.downloadData.length > 0
})

const batchCount = computed(() => {
    if (Array.isArray(props.downloadData)) {
        return props.downloadData.length
    }
    return 0
})

onMounted(() => {
    openDialog();
    // 监听并行下载的精细进度事件
    EventsOn("download:progress", onDownloadProgress);
});

onUnmounted(() => {
    EventsOff("download:progress");
});

const openDialog = () => {
    dialogVisible.value = props.dialogVisible
    if (props.downloadTypeOptions && props.downloadTypeOptions.length > 0) {
        downloadType.value = props.downloadTypeOptions[0].value
    }
}

const closeDialog = () => {
    if (isDownloading.value && !allowClose.value) {
        return
    }
    EventsOff("courseDownload", "ebookDownload", "odobDownload", "batchOdobDownload", "download:progress")
    percentage.value = 0
    content.value = ''
    isDownloading.value = false
    speedText.value = ''
    etaText.value = ''
    taskList.value = []
    emits("close")
}

// 处理精细进度事件
const onDownloadProgress = (data: any) => {
    if (!data) return
    
    // 更新总体进度（取第一个活跃任务或平均）
    if (data.pct !== undefined) {
        // 在批量模式下，更新任务列表
        if (data.task_id) {
            const idx = taskList.value.findIndex(t => t.task_id === data.task_id)
            if (idx >= 0) {
                taskList.value[idx] = { ...taskList.value[idx], ...data }
            } else {
                taskList.value.push({ ...data })
            }
        }
        
        // 更新主进度条（显示最快/最新任务的进度）
        if (data.status === STATUS.RUNNING) {
            percentage.value = data.pct
            content.value = data.value || data.title || '下载中...'
            
            // 速度和剩余时间
            if (data.speed_bps > 0) {
                speedText.value = formatSpeed(data.speed_bps)
            }
            if (data.eta) {
                etaText.value = data.eta
            }
        }
        
        // 完成状态
        if (data.status === STATUS.COMPLETED) {
            const allDone = taskList.value.every(t => 
                t.status === STATUS.COMPLETED || t.status === STATUS.CANCELLED || t.status === STATUS.FAILED
            )
            if (allDone && taskList.value.length > 0) {
                percentage.value = 100
                content.value = '全部完成'
                isDownloading.value = false
                allowClose.value = true
            }
        }
    }
}

// 格式化速度显示
const formatSpeed = (bps: number): string => {
    if (!bps || bps <= 0) return ''
    if (bps < 1024) return `${bps.toFixed(0)} B/s`
    if (bps < 1024 * 1024) return `${(bps / 1024).toFixed(1)} KB/s`
    if (bps < 1024 * 1024 * 1024) return `${(bps / 1024 / 1024).toFixed(1)} MB/s`
    return `${(bps / 1024 / 1024 / 1024).toFixed(1)} GB/s`
}

// 状态图标
const statusIcon = (status: number) => {
    switch (status) {
        case STATUS.PENDING: return 'Loading'
        case STATUS.RUNNING: return 'Loading'
        case STATUS.COMPLETED: return 'CircleCheckFilled'
        case STATUS.FAILED: return 'CircleCloseFilled'
        case STATUS.CANCELLED: return 'Warning'
        default: return 'Loading'
    }
}

const statusIconClass = (status: number) => {
    switch (status) {
        case STATUS.COMPLETED: return 'icon-success'
        case STATUS.FAILED: return 'icon-error'
        case STATUS.CANCELLED: return 'icon-warn'
        default: return ''
    }
}

const progressStatus = (status: number) => {
    switch (status) {
        case STATUS.COMPLETED: return 'success' as const
        case STATUS.FAILED: return 'exception' as const
        case STATUS.CANCELLED: return 'warning' as const
        default: return undefined
    }
}

// 取消单个任务（通过 Wails 调用后端）
const cancelTask = async (taskId: string) => {
    // TODO: 调用后端 CancelTask API
    console.log('Cancel task:', taskId)
}

const download = async () => {
    isDownloading.value = true
    allowClose.value = false
    content.value = '准备下载...'
    percentage.value = 0
    taskList.value = []
    
    try {
        if (isBatchMode.value) {
            if (props.prodType === 2) {
                EventsOn("ebookDownload", data => {
                    if (data) {
                        percentage.value = data.pct
                        content.value = data.value + ' 下载中...'
                    }
                })
                
                const ebooks = props.downloadData.map((item: any) => ({
                    id: item.id,
                    enid: item.enid,
                    title: item.title
                }))
                
                await BatchEbookDownload(ebooks, downloadType.value)
                
                ElMessage({ message: '已添加到下载队列', type: 'success' })
                
            } else if (props.prodType === 3) {
                EventsOn("batchOdobDownload", data => {
                    if (data) {
                        percentage.value = data.pct
                        content.value = data.value + ' 下载中...'
                    }
                })
                
                const odobs = props.downloadData.map((item: any) => ({
                    id: item.id,
                    enid: item.enid,
                    title: item.title,
                    audio_detail: item.audio_detail
                }))
                
                await BatchOdobDownload(odobs, downloadType.value)
                
                ElMessage({ message: '已添加到下载队列', type: 'success' })
                
            } else {
                EventsOn("courseDownload", data => {
                    if (data) {
                        percentage.value = data.pct
                        content.value = data.value + ' 下载中...'
                    }
                })
                
                const articles = props.downloadData.map((item: any) => ({
                    id: item.id,
                    aid: item.aid,
                    enid: item.enid,
                    title: item.title
                }))
                
                await BatchCourseDownload(articles, downloadType.value)
                
                ElMessage({ message: '已添加到下载队列', type: 'success' })
            }
            
            // 批量模式：不立即关闭，等待进度更新
            isDownloading.value = false
            allowClose.value = true
            return
        }
        
        // 单个下载模式
        switch (props.prodType) {
            case 2:
                EventsOn("ebookDownload", data => {
                    if (data) {
                        percentage.value = data.pct
                        content.value = data.value + ' 下载中...'
                    }
                })
                await EbookDownload(props.downloadId, downloadType.value, props.enId)
                break;

            case 66:
                EventsOn("courseDownload", data => {
                    if (data) {
                        percentage.value = data.pct
                        content.value = data.value + ' 下载中...'
                    }
                })
                await CourseDownload(props.downloadId, props.articleId, downloadType.value, props.enId)
                break;
            case 3:
                EventsOn("odobDownload", data => {
                    if (data) {
                        percentage.value = data.pct
                        content.value = data.value + ' 下载中...'
                    }
                })
                await OdobDownload(props.downloadId, downloadType.value, props.downloadData as any)
                break;
        }
    } catch (error) {
        ElMessage({ message: String(error), type: 'warning' })
    } finally {
        isDownloading.value = false
        allowClose.value = true
        closeDialog()
    }
}
</script>

<style scoped>
.download-container {
    padding: 10px 20px;
}

.format-selector {
    margin-bottom: 24px;
}

.section-label {
    font-size: 14px;
    color: var(--text-secondary, #606266);
    margin-bottom: 12px;
    font-weight: 500;
}

.batch-info {
    color: var(--accent-color, #ff6b00);
    font-weight: normal;
    margin-left: 8px;
}

.format-options {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
}

.format-option {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    height: 40px;
    border: 1px solid var(--border-color, #dcdfe6);
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;
    background-color: var(--fill-color-light, #f5f7fa);
    font-size: 14px;
    color: var(--text-primary, #303133);
}

.format-option:hover {
    border-color: var(--primary-color, #409eff);
    color: var(--primary-color, #409eff);
    background-color: var(--primary-color-light-9, #ecf5ff);
}

.format-option.active {
    border-color: var(--primary-color, #409eff);
    background-color: var(--primary-color, #409eff);
    color: white;
    font-weight: 500;
}

.selected-icon {
    position: absolute;
    right: 4px;
    top: 4px;
    font-size: 12px;
}

.download-status {
    margin-top: 20px;
    background: var(--fill-color-lighter, #fafafa);
    padding: 16px;
    border-radius: 8px;
    border: 1px solid var(--border-color-lighter, #ebeef5);
}

.status-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 8px;
    font-size: 13px;
}

.status-text {
    color: var(--text-regular, #606266);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 80%;
}

.status-percent {
    color: var(--primary-color, #409eff);
    font-weight: 600;
}

.speed-info {
    display: flex;
    justify-content: space-between;
    margin-top: 6px;
    font-size: 12px;
    color: var(--text-secondary, #909399);
}

.eta {
    color: var(--text-placeholder, #c0c4cc);
}

/* 任务列表 */
.task-list {
    margin-top: 16px;
}

.task-list-inner {
    max-height: 240px;
    overflow-y: auto;
    border: 1px solid var(--border-color-lighter, #ebeef5);
    border-radius: 8px;
    background: var(--fill-color-light, #f5f7fa);
}

.task-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color-lighter, #ebeef5);
    transition: background 0.15s;
}

.task-item:last-child {
    border-bottom: none;
}

.task-item:hover {
    background: var(--fill-color, #fff);
}

.task-item.task-done {
    opacity: 0.7;
}

.task-item.task-error {
    opacity: 0.8;
}

.task-info {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex: 1;
}

.task-icon {
    font-size: 16px;
    flex-shrink: 0;
}

.task-icon.icon-success { color: #67c23a; }
.task-icon.icon-error { color: #f56c6c; }
.task-icon.icon-warn { color: #e6a23c; }

.task-title {
    font-size: 13px;
    color: var(--text-primary, #303133);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.task-progress-area {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
}

.task-pct {
    font-size: 12px;
    color: var(--text-secondary, #909399);
    min-width: 36px;
    text-align: right;
}

.task-speed {
    font-size: 11px;
    color: var(--text-placeholder, #c0c4cc);
    min-width: 60px;
}

.dialog-footer {
    display: flex;
    justify-content: center;
    gap: 16px;
    padding-bottom: 8px;
}
</style>
