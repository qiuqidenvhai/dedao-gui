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
  </div>
</template>

<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
// @ts-ignore
import { SearchHot, SearchAll } from '../../wailsjs/go/backend/App'
// @ts-ignore
import { EbookInfo as GetEbookInfo } from '../../wailsjs/go/backend/App'
import { useEbookStore } from '../stores/ebook'

const router = useRouter()
const ebookStore = useEbookStore()

const searchKeyword = ref('')
const hotSearchData = ref<any[]>([])
const searchLoading = ref(false)

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

// 搜索建议 - 调用统一的搜索API
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
        // 根据类型决定跳转路径
        let path = '/bought/course'
        let typeName = '课程'
        
        // 判断内容类型 - 根据type字段或数据特征判断
        const itemType = item.type || item.product_type || 0
        
        // 常见的类型判断
        // 1: 课程, 2: 电子书, 3: 听书, 4: 视频等
        if (itemType === 2 || item.type_name?.includes('电子书') || item.type_name?.includes('ebook')) {
          path = '/bought/ebook'
          typeName = '电子书'
        } else if (itemType === 3 || item.type_name?.includes('听书') || item.type_name?.includes('odob')) {
          path = '/bought/odob'
          typeName = '听书'
        } else if (itemType === 4 || item.type_name?.includes('视频')) {
          path = '/bought/video'
          typeName = '视频'
        }
        
        return {
          value: item.title || item.name,
          title: item.title || item.name,
          enid: item.enid,
          icon: item.icon,
          type: itemType,
          typeName: typeName,
          path: path,
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

// 标准化 ENID 格式
const normalizeEnid = (enid: string): string => {
  if (!enid) return enid
  // 如果已经以 S 开头或长度超过 20，认为是完整格式
  if (enid.startsWith('S') || enid.length > 20) {
    return enid
  }
  // 否则添加 S 前缀
  return 'S' + enid
}

// 处理选择 - 根据类型跳转到对应页面或直接打开电子书详情
const handleSelect = async (item: any) => {
  searchKeyword.value = ''
  
  // 如果是热搜词（没有 path 字段），跳转到课程搜索页面
  if (!item.path && !item.enid) {
    router.push({
      path: '/bought/course',
      query: { keyword: item.searchKey || item.title }
    })
    return
  }
  
  // 如果是电子书 (type=2) 或听书 (type=3)，直接打开详情弹窗
  if (item.type === 2 || item.type === 3) {
    try {
      // 标准化 ENID 格式
      const normalizedEnid = normalizeEnid(item.enid)
      // 调用 API 获取书籍详情
      const detail = await GetEbookInfo(normalizedEnid)
      if (detail) {
        // 打开 EbookInfo 弹窗
        ebookStore.showEbookInfo(detail)
        return
      }
    } catch (error) {
      console.error('获取电子书详情失败:', error)
    }
  }
  
  // 其他类型跳转到对应页面
  router.push({
    path: item.path || '/bought/course',
    query: { keyword: item.title }
  })
}

// 处理回车 - 跳转到课程搜索页面（带回填关键词）
const handleEnter = () => {
  if (searchKeyword.value.trim()) {
    router.push({
      path: '/bought/course',
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
