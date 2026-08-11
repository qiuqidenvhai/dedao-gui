package app

import (
	"github.com/yann0917/dedao-gui/backend/services"
)

// AddToShelf 把商品加入书架（电子书 / 听书）。课程会返回不支持的提示。
func AddToShelf(enid string, productType int) (result *services.ShelfResult, err error) {
	result, err = getService().AddToShelf(enid, productType)
	return
}

// RemoveFromShelf 把商品移出书架。
func RemoveFromShelf(enid string, productType int) (result *services.ShelfResult, err error) {
	result, err = getService().RemoveFromShelf(enid, productType)
	return
}

// SearchProducts 全站搜索（课程 + 听书 + 电子书三路并发合并）。
func SearchProducts(keyword string, perType int) (result *services.SearchResult, err error) {
	result, err = getService().SearchProductsAll(keyword, perType)
	return
}

// SearchProductsByType 按商品类型分页搜索（2 电子书 / 13 听书 / 66 课程）。
func SearchProductsByType(keyword string, productType, page, size int) (result *services.SearchResult, err error) {
	result, err = getService().SearchProductsByType(keyword, productType, page, size)
	return
}
