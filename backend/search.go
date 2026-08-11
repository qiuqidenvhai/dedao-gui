package backend

import (
	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/services"
)

// SearchCourse 搜索课程
func (a *App) SearchCourse(keyword string, page, pageSize int) (result *services.SearchResult, err error) {
	result, err = app.SearchCourse(keyword, page, pageSize)
	return
}

// SearchAll 统一搜索（课程、电子书、听书等）
func (a *App) SearchAll(keyword string, page, pageSize int) (result *services.SearchResult, err error) {
	result, err = app.SearchAll(keyword, page, pageSize)
	return
}

// HallSearch 从向日葵大厅按关键字检索（全量遍历 + 客户端匹配）
func (a *App) HallSearch(keyword string, limit int) (list []services.Course, err error) {
	list, err = app.HallSearch(keyword, limit)
	return
}

// RefreshHallCache 后台刷新大厅商品缓存（启动 / 登录后各一次）
func (a *App) RefreshHallCache() {
	app.RefreshHallCache()
}

// HallCacheStatus 返回大厅缓存状态，供前端判断是否已就绪
func (a *App) HallCacheStatus() services.HallCacheState {
	return app.HallCacheStatus()
}

// SearchProducts 全站搜索（课程 + 听书 + 电子书三路并发合并）
// perType 表示每类最多取多少条（0=默认 30，最大 100）
func (a *App) SearchProducts(keyword string, perType int) (result *services.SearchResult, err error) {
	result, err = app.SearchProducts(keyword, perType)
	return
}

// SearchProductsByType 按商品类型分页搜索（2 电子书 / 13 听书 / 66 课程）
func (a *App) SearchProductsByType(keyword string, productType, page, size int) (result *services.SearchResult, err error) {
	result, err = app.SearchProductsByType(keyword, productType, page, size)
	return
}

// AddToShelf 加入书架（电子书 / 听书）
func (a *App) AddToShelf(enid string, productType int) (result *services.ShelfResult, err error) {
	result, err = app.AddToShelf(enid, productType)
	return
}

// RemoveFromShelf 移出书架
func (a *App) RemoveFromShelf(enid string, productType int) (result *services.ShelfResult, err error) {
	result, err = app.RemoveFromShelf(enid, productType)
	return
}
