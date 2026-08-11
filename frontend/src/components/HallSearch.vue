<template>
  <div class="hall-search-wrapper">
    <el-input
      v-model="searchKeyword"
      placeholder="大厅搜索（可找未上架/专享内容）"
      :prefix-icon="Search"
      clearable
      @keyup.enter="onEnter"
      class="hall-search-input"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'

const router = useRouter()
const searchKeyword = ref('')

// 大厅搜索框行为与普通搜索一致：只回车跳转到搜索结果页。
// 不在前端做 debounce、不显示 loading、下拉面板——
// 由 /bought/course 页统一用 SearchAll（suggest 无结果时自动 HallSearch 兜底）展示。
const onEnter = () => {
  const kw = searchKeyword.value.trim()
  if (!kw) return
  // 跳转到独立的大厅搜索结果页（专属「大厅」格式，区别于「我的课程」普通搜索）
  // 不清空输入框：保留已输入内容，避免用户觉得「框被清空、像没反应/假的」
  router.push({
    path: '/hall',
    query: { keyword: kw }
  })
}
</script>

<style scoped>
.hall-search-wrapper {
  width: 280px;
  margin-right: 0;
}
.hall-search-input {
  width: 100%;
}
.hall-search-input :deep(.el-input__wrapper) {
  border-radius: 20px;
  background-color: var(--bg-secondary);
  box-shadow: none;
  border: 1px solid var(--border-color);
}
.hall-search-input :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-color);
}
.hall-search-input :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 2px rgba(255, 107, 0, 0.1);
}
</style>
