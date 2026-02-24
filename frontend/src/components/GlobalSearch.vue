<template>
  <div class="search-wrapper">
    <el-autocomplete
      v-model="searchKeyword"
      :fetch-suggestions="querySearch"
      placeholder="搜索课程、听书..."
      :prefix-icon="Search"
      clearable
      :debounce="300"
      :trigger-on-focus="true"
      :hide-loading="false"
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
import { SearchHot } from '../../wailsjs/go/backend/App'

const router = useRouter()

const searchKeyword = ref('')
const hotSearchData = ref<any[]>([])

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

// 搜索建议
const querySearch = (queryString: string, cb: (results: any[]) => void) => {
  if (!queryString) {
    // 没有输入时显示热搜词
    cb(hotSearchData.value.slice(0, 10))
    return
  }
  
  // 过滤热搜词
  const filtered = hotSearchData.value.filter(item => {
    const title = (item.title || '').toLowerCase()
    const searchKey = (item.searchKey || '').toLowerCase()
    const query = queryString.toLowerCase()
    return title.includes(query) || searchKey.includes(query)
  })
  
  cb(filtered.slice(0, 20))
}

// 处理选择
const handleSelect = (item: any) => {
  const searchKey = item.searchKey || item.value || item.title
  if (searchKey) {
    router.push({
      path: '/bought/course',
      query: { keyword: searchKey }
    })
  }
  searchKeyword.value = ''
}

// 处理回车
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
