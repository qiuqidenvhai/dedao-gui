package app

import (
	"github.com/yann0917/dedao-gui/backend/services"
)

// SearchCourse 搜索课程
func SearchCourse(keyword string, page, pageSize int) (result *services.SearchResult, err error) {
	result, err = getService().SearchCourse(keyword, page, pageSize)
	return
}

// SearchAll 统一搜索（课程、电子书、听书等）
func SearchAll(keyword string, page, pageSize int) (result *services.SearchResult, err error) {
	result, err = getService().SearchAll(keyword, page, pageSize)
	return
}

// HallSearch 从向日葵大厅按关键字检索（全量遍历 + 客户端匹配），返回 Course 列表
func HallSearch(keyword string, limit int) (list []services.Course, err error) {
	list, err = getService().HallSearch(keyword, limit)
	return
}

// RefreshHallCache 后台刷新大厅商品缓存（App 启动 / 登录成功后各调用一次）。
// 必须在已登录状态下调用，否则向日葵接口无权限。
func RefreshHallCache() {
	getService().RefreshHallCache()
}

// HallCacheStatus 返回大厅缓存状态，供前端判断是否已就绪。
func HallCacheStatus() services.HallCacheState {
	return getService().HallCacheStatus()
}

// SearchEbook 搜索电子书
func SearchEbook(keyword string, page, pageSize int) (result *services.SearchResult, err error) {
	result, err = getService().SearchEbook(keyword, page, pageSize)
	return
}

// SearchOdob 搜索听书
func SearchOdob(keyword string, page, pageSize int) (result *services.SearchResult, err error) {
	result, err = getService().SearchOdob(keyword, page, pageSize)
	return
}
