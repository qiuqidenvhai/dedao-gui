package services

import (
	"fmt"

	"github.com/yann0917/dedao-gui/backend/utils"
)

// SearchResult 搜索结果
type SearchResult struct {
	List  []Course `json:"list"`
	Total int      `json:"total"`
}

// SearchCourse 搜索课程
func (s *Service) SearchCourse(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用得到搜索API
	resp, err := s.client.R().
		SetBody(map[string]interface{}{
			"keyword":   keyword,
			"page":      page,
			"page_size": pageSize,
		}).
		Post("/pc/search/v1/course")

	if err != nil {
		return nil, err
	}

	// 使用通用响应处理
	result = &SearchResult{}
	
	// 直接解析响应体
	err = utils.UnmarshalJSON(resp.Body(), result)
	if err != nil {
		fmt.Printf("SearchCourse unmarshal error: %s\n", err.Error())
		// 打印响应内容用于调试
		fmt.Printf("SearchCourse response: %s\n", string(resp.Body()))
		return nil, err
	}

	return result, nil
}
