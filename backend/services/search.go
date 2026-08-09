package services

import (
	"encoding/json"
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

// CourseListResp 课程列表响应（用于解析产品列表API）
type CourseListResp struct {
	List   []Course `json:"list"`
	Total  int      `json:"total"`
	IsMore int      `json:"is_more"`
}

// SuggestItem 单个搜索建议项
type SuggestItem struct {
	ID      int    `json:"id"`
	Type    int    `json:"type"`
	Tname   string `json:"tname"`
	Title   string `json:"title"`
	Author  string `json:"author"`
	Content string `json:"content"`
	Extra   struct {
		Enid  string `json:"enid"`
		Image string `json:"image"`
		Press string `json:"press"`
	} `json:"extra"`
}

// SuggestList suggest API 返回的列表结构
type SuggestList struct {
	Type      int           `json:"type"`
	TabType   int           `json:"tab_type"`
	TrackName string        `json:"track_name"`
	Total     int           `json:"total"`
	List      []SuggestItem `json:"list"`
}

// reqSuggest 请求搜索建议API
func (s *Service) reqSuggest(keyword string, searchType int) (io.ReadCloser, error) {
	resp, err := s.client.R().
		SetBody(map[string]interface{}{
			"query":      keyword,
			"searchType": searchType,
		}).
		Post("/api/search/pc/suggest")
	return handleHTTPResponse(resp, err)
}

// SearchSuggest 搜索建议（使用新的 suggest API）
func (s *Service) SearchSuggest(keyword string, page, pageSize int) (result *SearchResult, err error) {
	if keyword == "" {
		return &SearchResult{List: []Course{}, Total: 0}, nil
	}

	body, err := s.reqSuggest(keyword, 0)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	// 解析 suggest API 的响应
	var rawResp struct {
		H struct {
			C int `json:"c"`
		} `json:"h"`
		C json.RawMessage `json:"c"`
	}

	err = utils.UnmarshalReader(body, &rawResp)
	if err != nil {
		fmt.Printf("SearchSuggest parse error: %s\n", err.Error())
		return nil, err
	}

	// c.list 是一个数组，每个元素有 list 字段
	type SuggestOuterList struct {
		Type      int           `json:"type"`
		TabType   int           `json:"tab_type"`
		TrackName string        `json:"track_name"`
		Total     int           `json:"total"`
		List      []SuggestItem `json:"list"`
	}
	type SuggestOuter struct {
		List []SuggestOuterList `json:"list"`
	}

	var outer SuggestOuter
	err = json.Unmarshal(rawResp.C, &outer)
	if err != nil {
		// 尝试另一种格式
		var singleOuter SuggestOuterList
		err = json.Unmarshal(rawResp.C, &singleOuter)
		if err != nil {
			fmt.Printf("SearchSuggest unmarshal error: %s\n", err.Error())
			return nil, err
		}
		outer.List = []SuggestOuterList{singleOuter}
	}

	// 转换 SuggestList 到 Course 列表
	var courses []Course
	for _, sl := range outer.List {
		for _, item := range sl.List {
			// 确定产品类型：2=电子书，3=听书，其他=课程
			var classType int
			var pType int
			switch item.Type {
			case 2:
				classType = 13 // 电子书类型
				pType = 2
			case 3:
				classType = 14 // 听书类型
				pType = 3
			default:
				classType = 66 // 课程
				pType = 66
			}

			// 去掉 title 中的 <hl> 标签
			cleanTitle := strings.ReplaceAll(item.Title, "<hl>", "")
			cleanTitle = strings.ReplaceAll(cleanTitle, "</hl>", "")

			// 去掉 content 中的 <hl> 标签
			cleanIntro := strings.ReplaceAll(item.Content, "<hl>", "")
			cleanIntro = strings.ReplaceAll(cleanIntro, "</hl>", "")

			course := Course{
				ID:          item.ID,
				Enid:        item.Extra.Enid,
				Type:        pType,
				ClassType:   classType,
				Title:       cleanTitle,
				Intro:       cleanIntro,
				Author:      item.Author,
				Icon:        item.Extra.Image, // 设置封面图片
				IsCollected: false,
			}
			courses = append(courses, course)
		}
	}

	result = &SearchResult{
		List:  courses,
		Total: len(courses),
	}

	return result, nil
}

// SearchCourse 搜索课程
func (s *Service) SearchCourse(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用新的 suggest API
	return s.SearchSuggest(keyword, page, pageSize)
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

// SearchAll 统一搜索（课程、电子书、听书等）+ 向日葵大厅数据兜底
func (s *Service) SearchAll(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 1. 优先调用 suggest API（通用搜索建议）
	result, err = s.SearchSuggest(keyword, page, pageSize)
	if err != nil || result == nil || len(result.List) == 0 {
		// 2. suggest 没有结果或失败，用向日葵大厅数据兜底搜索
		var hallResult []Course
		hallResult, err = s.HallSearch(keyword, pageSize)
		if err == nil && len(hallResult) > 0 {
			result = &SearchResult{List: hallResult, Total: len(hallResult)}
		} else {
			// 都失败时返回空列表而不是报错
			if result == nil {
				result = &SearchResult{List: []Course{}, Total: 0}
			}
			err = nil
		}
	}
	return
}

// HallSearch 从向日葵大厅搜索产品，作为搜索兜底源
// nType: 2-电子书、4-课程、8-听书
func (s *Service) HallSearch(keyword string, limit int) ([]Course, error) {
	nTypes := []int{2, 4, 8}
	var allResults []Course

	for _, nType := range nTypes {
		if len(allResults) >= limit {
			break
		}
		labelList, err := s.SunflowerLabelList(nType)
		if err != nil {
			continue
		}
		for _, nav := range labelList.List {
			if len(allResults) >= limit {
				break
			}
			content, err := s.SunflowerLabelContent(nav.Enid, nType, 0, 10)
			if err != nil {
				continue
			}
			for _, prod := range content.ProductList {
				if len(allResults) >= limit {
					break
				}
				if strings.Contains(prod.Title, keyword) || strings.Contains(prod.Intro, keyword) {
					// pType 和 classType 映射：2=电子书, 4=课程, 8=听书
					var pType, classType int
					switch nType {
					case 2:
						pType = 2; classType = 13
					case 8:
						pType = 3; classType = 14
					default: // 4=课程
						pType = 66; classType = 1
					}
					allResults = append(allResults, Course{
						ID:          0,
						Enid:        prod.ProductEnid,
						Type:        pType,
						ClassType:   classType,
						Title:       prod.Title,
						Intro:       prod.Intro,
						Author:      strings.Join(prod.AuthorList, ","),
						Icon:        prod.IndexImage,
						IsCollected: false,
					})
				}
			}
		}
	}
	return allResults, nil
}

// SearchEbook 搜索电子书
func (s *Service) SearchEbook(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用新的 suggest API，searchType=2 只返回电子书
	if keyword == "" {
		return &SearchResult{List: []Course{}, Total: 0}, nil
	}

	body, err := s.reqSuggest(keyword, 2)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	// 解析响应
	var rawResp struct {
		H struct {
			C int `json:"c"`
		} `json:"h"`
		C json.RawMessage `json:"c"`
	}

	err = utils.UnmarshalReader(body, &rawResp)
	if err != nil {
		return nil, err
	}

	// c.list 是一个数组，每个元素有 list 字段
	type SuggestOuterList struct {
		Type      int           `json:"type"`
		TabType   int           `json:"tab_type"`
		TrackName string        `json:"track_name"`
		Total     int           `json:"total"`
		List      []SuggestItem `json:"list"`
	}
	type SuggestOuter struct {
		List []SuggestOuterList `json:"list"`
	}

	var outer SuggestOuter
	err = json.Unmarshal(rawResp.C, &outer)
	if err != nil {
		var singleOuter SuggestOuterList
		err = json.Unmarshal(rawResp.C, &singleOuter)
		if err != nil {
			return nil, err
		}
		outer.List = []SuggestOuterList{singleOuter}
	}

	var courses []Course
	for _, sl := range outer.List {
		for _, item := range sl.List {
			cleanTitle := strings.ReplaceAll(item.Title, "<hl>", "")
			cleanTitle = strings.ReplaceAll(cleanTitle, "</hl>", "")
			cleanIntro := strings.ReplaceAll(item.Content, "<hl>", "")
			cleanIntro = strings.ReplaceAll(cleanIntro, "</hl>", "")

			course := Course{
				ID:          item.ID,
				Enid:        item.Extra.Enid,
				Type:        2,
				ClassType:   13,
				Title:       cleanTitle,
				Intro:       cleanIntro,
				Author:      item.Author,
				Icon:        item.Extra.Image, // 设置封面图片
				IsCollected: false,
			}
			courses = append(courses, course)
		}
	}

	result = &SearchResult{
		List:  courses,
		Total: len(courses),
	}

	return result, nil
}

// SearchOdob 搜索听书
func (s *Service) SearchOdob(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用新的 suggest API，searchType=3 只返回听书
	if keyword == "" {
		return &SearchResult{List: []Course{}, Total: 0}, nil
	}

	body, err := s.reqSuggest(keyword, 3)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	// 解析响应
	var rawResp struct {
		H struct {
			C int `json:"c"`
		} `json:"h"`
		C json.RawMessage `json:"c"`
	}

	err = utils.UnmarshalReader(body, &rawResp)
	if err != nil {
		return nil, err
	}

	// c.list 是一个数组，每个元素有 list 字段
	type SuggestOuterList struct {
		Type      int           `json:"type"`
		TabType   int           `json:"tab_type"`
		TrackName string        `json:"track_name"`
		Total     int           `json:"total"`
		List      []SuggestItem `json:"list"`
	}
	type SuggestOuter struct {
		List []SuggestOuterList `json:"list"`
	}

	var outer SuggestOuter
	err = json.Unmarshal(rawResp.C, &outer)
	if err != nil {
		var singleOuter SuggestOuterList
		err = json.Unmarshal(rawResp.C, &singleOuter)
		if err != nil {
			return nil, err
		}
		outer.List = []SuggestOuterList{singleOuter}
	}

	var courses []Course
	for _, sl := range outer.List {
		for _, item := range sl.List {
			cleanTitle := strings.ReplaceAll(item.Title, "<hl>", "")
			cleanTitle = strings.ReplaceAll(cleanTitle, "</hl>", "")
			cleanIntro := strings.ReplaceAll(item.Content, "<hl>", "")
			cleanIntro = strings.ReplaceAll(cleanIntro, "</hl>", "")

			course := Course{
				ID:          item.ID,
				Enid:        item.Extra.Enid,
				Type:        3,
				ClassType:   14,
				Title:       cleanTitle,
				Intro:       cleanIntro,
				Author:      item.Author,
				Icon:        item.Extra.Image, // 设置封面图片
				IsCollected: false,
			}
			courses = append(courses, course)
		}
	}

	result = &SearchResult{
		List:  courses,
		Total: len(courses),
	}

	return result, nil
}
