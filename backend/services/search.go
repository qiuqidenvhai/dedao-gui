package services

import (
"fmt"
"io"
"strings"

"github.com/yann0917/dedao-gui/backend/utils"
)

// SearchResult 搜索结果
type SearchResult struct {
List  []Course `json:"list"`
Total int      `json:"total"`
}

// CourseListResp 课程列表响应（用于解析产品列表 API）
type CourseListResp struct {
List   []Course `json:"list"`
Total  int      `json:"total"`
IsMore int      `json:"is_more"`
}

// ProductListResp 产品列表响应（用于解析产品列表 API）
type ProductListResp struct {
List   []ProductSimple `json:"list"`
Total  int             `json:"total"`
IsMore int             `json:"is_more"`
}

// convertProductToCourse 将 ProductSimple 转换为 Course
func convertProductToCourse(item ProductSimple) Course {
// 根据 product_type 设置 type 和 class_type
// 2=电子书，3=听书
classType := 13 // 默认电子书
if item.ProductType == 3 {
classType = 14 // 听书
}

// 拼接作者列表
author := ""
if len(item.AuthorList) > 0 {
author = item.AuthorList[0]
}

return Course{
Enid:        item.ProductEnid,
ID:          0, // 产品列表中没有 ID
Type:        item.ProductType,
ClassType:   classType,
Title:       item.Title,
Intro:       item.Intro,
Author:      author,
Icon:        item.IndexImage,
IsCollected: false,
}
}

// SearchCourse 搜索课程
func (s *Service) SearchCourse(keyword string, page, pageSize int) (result *SearchResult, err error) {
// 使用得到搜索 API - /pc/search/v1/course
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

var resp ProductListResp
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

// SearchAll 统一搜索（课程、电子书、听书等）+ 大厅精选并列显示
func (s *Service) SearchAll(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 1. 先获取普通搜索结果（稳定，保留原有逻辑）
	normalResult, err := s.SearchCourse(keyword, page, pageSize)
	if err != nil {
		normalResult = &SearchResult{List: []Course{}, Total: 0}
	}

	// 2. 获取大厅精选内容（遍历所有标签，按关键词匹配，数据最全但稍慢）
	hallResults := []Course{}
	if keyword != "" {
		// 使用 SearchHall 从大厅全量数据中搜索匹配项
		hallData, err := s.SearchHall(keyword, pageSize)
		if err == nil && len(hallData) > 0 {
			hallResults = hallData
		}
	}

	// 合并结果：普通搜索在前，大厅精选在后
	allList := append(normalResult.List, hallResults...)

	result = &SearchResult{
		List:  allList,
		Total: normalResult.Total + len(hallResults),
	}

	return result, nil
}

// SearchEbook 搜索电子书
func (s *Service) SearchEbook(keyword string, page, pageSize int) (result *SearchResult, err error) {
// 使用产品列表 API 搜索电子书 (product_type=2)
return s.SearchProducts(keyword, 2, page, pageSize)
}

// SearchOdob 搜索听书
func (s *Service) SearchOdob(keyword string, page, pageSize int) (result *SearchResult, err error) {
// 使用产品列表 API 搜索听书 (product_type=3)
return s.SearchProducts(keyword, 3, page, pageSize)
}
