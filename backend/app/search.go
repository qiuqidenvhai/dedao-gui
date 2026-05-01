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
