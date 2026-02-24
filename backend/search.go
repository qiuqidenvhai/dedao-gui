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
