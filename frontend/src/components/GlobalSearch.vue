<template>
  <div class="search-wrapper">
    <el-autocomplete
      v-model="searchKeyword"
      :fetch-suggestions="querySearch"
      placeholder="搜索课程、电子书、听书..."
      :prefix-icon="Search"
      clearable
      :debounce="300"
      :trigger-on-focus="false"
      :hide-loading="false"
      :loading="searchLoading"
      @select="handleSelect"
      @keyup.enter="handleEnter"
      class="search-autocomplete"
    />
    <!-- 课程详情弹窗 -->
    <course-info
      v-if="courseInfoVisible"
      :enid="selectedEnid"
      :dialog-visible="courseInfoVisible"
      @close="closeCourseInfo"
    />
    <!-- 电子书详情弹窗（含加入书架/移出书架） -->
    <ebook-info
      v-if="ebookInfoVisible"
      :enid="selectedEnid"
      :dialog-visible="ebookInfoVisible"
      @close="closeEbookInfo"
    />
    <!-- 听书详情弹窗 -->
    <audio-info
      v-if="audioInfoVisible"
      :enid="selectedEnid"
      :dialog-visible="audioInfoVisible"
      @close="closeAudioInfo"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
// @ts-ignore
import { SearchHot, SearchAll } from '../../wailsjs/go/backend/App'
import CourseInfo from './CourseInfo.vue'
import EbookInfo from './EbookInfo.vue'
import AudioInfo from './AudioInfo.vue'

const router = useRouter()

const searchKeyword = ref('')
const hotSearchData = ref<any[]>([])
const searchLoading = ref(false)

// 三个类型弹窗状态
const courseInfoVisible = ref(false)
const ebookInfoVisible = ref(false)
const audioInfoVisible = ref(false)
const selectedEnid = ref('')

// 加载热搜词
onMounted(async () => {
  try {
    const result = await SearchHot()
    if (result && result.hot_tab_list) {
      const allHot: any[] = []
      result.hot_tab_list.forEach((tab: any) => {
        if (tab.list && tab.list.length > 0) {
          tab.list.forEach((item: any) => {
            allHot.push({
              value: item.title || item.searchKey,
              title: item.title,
              searchKey: item.searchKey,
              subTitle: item.subTitle
            })
          })
        }
      })
      hotSearchData.value = allHot
    }
  } catch (error) {
    console.error('加载热搜词失败:', error)
    // 加载失败时使用默认热搜词
    hotSearchData.value = [
      { value: '职场', title: '职场', searchKey: '职场' },
      { value: '管理', title: '管理', searchKey: '管理' },
      { value: '商业', title: '商业', searchKey: '商业' },
      { value: '理财', title: '理财', searchKey: '理财' },
      { value: '成长', title: '成长', searchKey: '成长' },
      { value: '心理', title: '心理', searchKey: '心理' },
      { value: '育儿', title: '育儿', searchKey: '育儿' },
      { value: '健康', title: '健康', searchKey: '健康' }
    ]
  }
})

// 根据后端 type 字段分类内容类型和跳转路径
// 后端 course.go Course struct 的 Type：
//   2 = 电子书, 3 = 听书(oDOB), 66 = 课程(新 suggest), 1 = 课程(旧)
const classifyItem = (item: any) => {
  const itemType = item.type || 0
  let path = '/bought/course'
  let typeName = '课程'

  if (itemType === 2) {
    path = '/bought/ebook'
    typeName = '电子书'
  } else if (itemType === 3) {
    path = '/bought/odob'
    typeName = '听书'
  } else if (itemType === 1 || itemType === 66) {
    path = '/bought/course'
    typeName = '课程'
  } else if (itemType === 4 || item.type_name?.includes('视频')) {
    path = '/bought/video'
    typeName = '视频'
  }

  return { path, typeName, itemType }
}

// 搜索建议 - 调用统一的搜索API（已包含向日葵大厅兜底）
const querySearch = async (queryString: string, cb: (results: any[]) => void) => {
  if (!queryString || queryString.trim().length === 0) {
    // 没有输入时显示热搜词
    cb(hotSearchData.value.slice(0, 10))
    return
  }

  searchLoading.value = true
  try {
    // 调用统一的搜索API
    const result = await SearchAll(queryString.trim(), 1, 20)
    if (result && result.list && result.list.length > 0) {
      const searchResults = result.list.map((item: any) => {
        const { path, typeName, itemType } = classifyItem(item)
        return {
          value: item.title || item.name,
          title: item.title || item.name,
          enid: item.enid,
          icon: item.icon,
          type: itemType,
          typeName,
          path,
          intro: item.intro || item.description || ''
        }
      })
      cb(searchResults)
    } else {
      // 没有搜索结果时过滤热搜词
      const filtered = hotSearchData.value.filter(item => {
        const title = (item.title || '').toLowerCase()
        const searchKey = (item.searchKey || '').toLowerCase()
        const query = queryString.toLowerCase()
        return title.includes(query) || searchKey.includes(query)
      })
      cb(filtered.slice(0, 10))
    }
  } catch (error) {
    console.error('搜索失败:', error)
    // 搜索失败时过滤热搜词
    const filtered = hotSearchData.value.filter(item => {
      const title = (item.title || '').toLowerCase()
      const searchKey = (item.searchKey || '').toLowerCase()
      const query = queryString.toLowerCase()
      return title.includes(query) || searchKey.includes(query)
    })
    cb(filtered.slice(0, 10))
  } finally {
    searchLoading.value = false
  }
}

// 关闭各弹窗
const closeCourseInfo = () => {
  courseInfoVisible.value = false
  selectedEnid.value = ''
}
const closeEbookInfo = () => {
  ebookInfoVisible.value = false
  selectedEnid.value = ''
}
const closeAudioInfo = () => {
  audioInfoVisible.value = false
  selectedEnid.value = ''
}

// 处理选择 - 根据类型打开对应详情弹窗
const handleSelect = (item: any) => {
  // 如果是热搜词（没有path字段），跳转到课程搜索页面
  if (!item.path) {
    router.push({
      path: '/bought/course',
      query: { keyword: item.searchKey || item.title }
    })
  } else {
    switch (item.type) {
      case 2:
        // 电子书(type=2)：打开电子书详情弹窗，可加入书架/移出书架
        selectedEnid.value = item.enid
        ebookInfoVisible.value = true
        break
      case 3:
        // 听书(type=3)：打开听书详情弹窗
        selectedEnid.value = item.enid
        audioInfoVisible.value = true
        break
      case 1:
      case 66:
        // 课程(type=1或66)：打开课程详情弹窗
        selectedEnid.value = item.enid
        courseInfoVisible.value = true
        break
      default:
        // 其他类型：跳转到对应列表页
        router.push({
          path: item.path,
          query: { keyword: item.title }
        })
    }
  }
  searchKeyword.value = ''
}

// 处理回车 - 跳转到大厅结果页（统一的搜索结果视图，带回填关键词）
const handleEnter = () => {
  if (searchKeyword.value.trim()) {
    router.push({
      path: '/hall',
      query: { keyword: searchKeyword.value.trim() }
    })
  }
}
</script>

<style scoped>
.search-wrapper {
  width: 280px;
  margin-right: 16px;
}

.search-autocomplete {
  width: 100%;
}

.search-autocomplete :deep(.el-input__wrapper) {
  border-radius: 20px;
  background-color: var(--bg-secondary);
  box-shadow: none;
  border: 1px solid var(--border-color);
}

.search-autocomplete :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-color);
}

.search-autocomplete :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(255, 107, 0, 0.1);
}

.search-autocomplete :deep(.el-autocomplete-suggestion) {
  background-color: var(--bg-color) !important;
  border: 1px solid var(--border-color) !important;
  border-radius: 8px !important;
}

.search-autocomplete :deep(.el-autocomplete-suggestion li) {
  color: var(--text-primary) !important;
  padding: 10px 15px !important;
}

.search-autocomplete :deep(.el-autocomplete-suggestion li:hover) {
  background-color: var(--card-hover-bg) !important;
}

.search-autocomplete :deep(.el-autocomplete-suggestion__list) {
  padding: 5px 0 !important;
}
</style>
