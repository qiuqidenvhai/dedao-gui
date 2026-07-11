package services

import (
	"fmt"
	"io"

	"github.com/yann0917/dedao-gui/backend/utils"
)

// SearchResult 搜索结果
type SearchResult struct {
	List  []Course `json:"list"`
	Total int      `json:"total"`
}

// CourseListResp 课程列表响应（用于解析产品列表API）
type CourseListResp struct {
	List   []Course `json:"list"`
	Total  int      `json:"total"`
	IsMore int      `json:"is_more"`
}

// SearchCourse 搜索课程
func (s *Service) SearchCourse(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用得到搜索API - /pc/search/v1/course
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

	// 直接解析响应体
	result = &SearchResult{}

	err = utils.UnmarshalJSON(resp.Body(), result)
	if err != nil {
		fmt.Printf("SearchCourse unmarshal error: %s\n", err.Error())
		fmt.Printf("SearchCourse response: %s\n", string(resp.Body()))
		return nil, err
	}

	return result, nil
}

// reqProductList 请求产品列表（用于搜索电子书和听书）
func (s *Service) reqProductList(keyword string, productType int, page, limit int) (io.ReadCloser, error) {
	body := map[string]interface{}{
		"display_group":   false,
		"filter":          "",
		"filter_complete": 0,
		"page":            page,
		"page_size":       limit,
		"sort_type":       "desc",
	}

	// 如果有搜索关键词
	if keyword != "" {
		body["keyword"] = keyword
	}

	// 如果有产品类型
	if productType > 0 {
		body["product_type"] = productType
	}

	resp, err := s.client.R().
		SetBody(body).
		Post("/api/hades/v2/product/list")

	return handleHTTPResponse(resp, err)
}

// SearchProducts 搜索产品（支持指定类型）
func (s *Service) SearchProducts(keyword string, productType int, page, pageSize int) (result *SearchResult, err error) {
	body, err := s.reqProductList(keyword, productType, page, pageSize)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var resp CourseListResp
	err = handleJSONParse(body, &resp)
	if err != nil {
		fmt.Printf("SearchProducts error: %s\n", err.Error())
		return nil, err
	}

	result = &SearchResult{
		List:  resp.List,
		Total: resp.Total,
	}

	return result, nil
}

// SearchAll 统一搜索（课程、电子书、听书等）
func (s *Service) SearchAll(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用课程搜索API
	return s.SearchCourse(keyword, page, pageSize)
}

// SearchEbook 搜索电子书
func (s *Service) SearchEbook(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用产品列表API搜索电子书 (product_type=2)
	return s.SearchProducts(keyword, 2, page, pageSize)
}

// SearchOdob 搜索听书
func (s *Service) SearchOdob(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用产品列表API搜索听书 (product_type=3)
	return s.SearchProducts(keyword, 3, page, pageSize)
}
