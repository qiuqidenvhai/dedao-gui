package app

import (
	"github.com/yann0917/dedao-gui/backend/services"
)

// SearchCourse 搜索课程
func SearchCourse(keyword string, page, pageSize int) (result *services.SearchResult, err error) {
	result, err = getService().SearchCourse(keyword, page, pageSize)
	return
}
