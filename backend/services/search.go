package services

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

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
			// 确定产品类型（服务端真实取值：电子书 2 / 听书 13 / 课程 66）
			// 注意 suggest 返回的 item.Type 里听书是 3，需要映射成真实的 13，
			// 否则详情、加书架都会走错接口。
			var classType int
			var pType int
			switch item.Type {
			case 2:
				pType, classType = ProductTypeEbook, ProductTypeEbook
			case 3, 13:
				pType, classType = ProductTypeOdob, ProductTypeOdob
			default:
				pType, classType = ProductTypeCourse, ProductTypeCourse
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

// SearchCourse 搜索课程。
// 走正式分页搜索接口 /api/search/v2/pc/searchclass（suggest 每 tab 只回 2 条，不能用）。
func (s *Service) SearchCourse(keyword string, page, pageSize int) (result *SearchResult, err error) {
	if pageSize <= 0 {
		pageSize = 30
	}
	result, err = s.SearchCourseV2(keyword, page, pageSize)
	if err == nil && result != nil {
		return result, nil
	}
	// 正式接口异常时退回 suggest，至少还能出两条
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

// SearchAll 统一搜索（课程 + 听书 + 电子书三路并发合并）。
//
// 2026-08-10 重写：原来走 /api/search/pc/suggest（搜索框联想接口），
// 每个 tab 最多只回 2 条 —— 这就是「一个关键词只搜出一两条 / 只有听书」的根因。
// 现在走 dedao.cn 搜索结果页真正用的三个分页接口（见 search_v2.go），
// 结果完整、带真实 total、enid 一定有效（能正常预览）。
func (s *Service) SearchAll(keyword string, page, pageSize int) (result *SearchResult, err error) {
	kw := strings.TrimSpace(keyword)
	if kw == "" {
		return &SearchResult{List: []Course{}, Total: 0}, nil
	}
	if pageSize <= 0 {
		pageSize = 30
	}

	// 1. 主路径：真·全站搜索（三接口并发，支持翻页）
	result, err = s.SearchProductsAllPaged(kw, page, pageSize)
	if err == nil && result != nil && len(result.List) > 0 {
		return result, nil
	}

	// 翻到第 2 页以后没数据就是真到底了，不要再用 suggest / 缓存兜底，
	// 否则前端「加载更多」会把第 1 页的东西又追加一遍。
	if page > 1 {
		return &SearchResult{List: []Course{}, Total: 0}, nil
	}

	// 2. 兜底一：suggest（虽只有 2 条，但总比空好）
	if r2, e2 := s.SearchSuggest(kw, page, pageSize); e2 == nil && r2 != nil && len(r2.List) > 0 {
		return r2, nil
	}

	// 3. 兜底二：本地大厅缓存
	if GlobalHallCache.IsLoaded() {
		if res := GlobalHallCache.Search(kw, pageSize); len(res) > 0 {
			return &SearchResult{List: res, Total: len(res)}, nil
		}
	}

	return &SearchResult{List: []Course{}, Total: 0}, nil
}

// HallSearch 大厅检索（全站商品）。
//
// 2026-08-10 重写：以前靠「预加载向日葵目录到内存 + 子串匹配」，
// 只能覆盖向日葵精选目录（约 7k 条），大量商品搜不到，而且类型判定也不准。
// 现在直接用得到官方搜索结果页的三个分页接口（课程 / 听书 / 电子书，见 search_v2.go），
// 一次并发拿齐三类，条数由 limit 决定 —— 结果完整，不会「只找到听书」。
// 本地大厅缓存降级为补充：把官方搜索没覆盖到的本地命中项追加在后面。
func (s *Service) HallSearch(keyword string, limit int) ([]Course, error) {
	kw := strings.TrimSpace(keyword)
	if kw == "" {
		return []Course{}, nil
	}
	if limit <= 0 {
		limit = 150
	}

	// 每类取 limit/3（官方单页上限 100）
	perType := limit / 3
	if perType < 20 {
		perType = 20
	}
	if perType > 100 {
		perType = 100
	}

	var (
		merged []Course
		seen   = map[string]bool{}
	)
	appendUniq := func(list []Course) {
		for _, c := range list {
			if c.Enid == "" || seen[c.Enid] {
				continue
			}
			seen[c.Enid] = true
			merged = append(merged, c)
		}
	}

	// 1. 主路径：官方全站搜索（课程 + 听书 + 电子书三路并发）
	result, err := s.SearchProductsAll(kw, perType)
	if err == nil && result != nil {
		appendUniq(result.List)
	}

	// 2. 补充：本地大厅缓存里命中但官方搜索没返回的（向日葵专享位等）
	if GlobalHallCache.IsLoaded() {
		appendUniq(GlobalHallCache.Search(kw, limit))
	} else {
		// 缓存还没建好：只有在官方搜索也没结果时才现场扫一次向日葵，
		// 同时触发后台预加载，下次就有缓存了。
		if len(merged) == 0 {
			if list, serr := s.hallScan(kw, limit); serr == nil {
				appendUniq(list)
			}
		}
		go GlobalHallCache.Load(s)
	}

	// 3. 都为空才认为失败
	if len(merged) == 0 {
		if err != nil {
			return nil, err
		}
		return []Course{}, nil
	}
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

// hallScan 受控扫描向日葵大厅各分类导航的首页内容，在客户端做大小写无关包含匹配。
// 仅作为 suggest 无结果时的兜底：并发 4 路、每个导航只取 1 页（50 条）、整体 8 秒超时。
func (s *Service) hallScan(keyword string, limit int) ([]Course, error) {
	kw := strings.ToLower(keyword)
	deadline := time.Now().Add(8 * time.Second)

	type job struct {
		enid  string
		nType int
	}
	var jobs []job
	var listErr error
	for _, nType := range []int{4, 2, 8} {
		if time.Now().After(deadline) {
			break
		}
		labelList, err := s.SunflowerLabelList(nType)
		if err != nil {
			listErr = err
			continue
		}
		for _, nav := range labelList.List {
			jobs = append(jobs, job{enid: nav.Enid, nType: nType})
		}
	}
	if len(jobs) == 0 {
		return nil, listErr
	}

	const workers = 4
	const pageSize = 50
	var (
		mu      sync.Mutex
		results []Course
		wg      sync.WaitGroup
	)
	ch := make(chan job)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				if time.Now().After(deadline) {
					continue // 排空剩余任务，避免死锁
				}
				mu.Lock()
				enough := len(results) >= limit
				mu.Unlock()
				if enough {
					continue
				}
				content, err := s.SunflowerLabelContent(j.enid, j.nType, 0, pageSize)
				if err != nil || content == nil {
					continue
				}
				var matched []Course
				for _, prod := range content.ProductList {
					title := strings.ToLower(prod.Title)
					intro := strings.ToLower(prod.Intro + " " + prod.Introduction)
					if !strings.Contains(title, kw) && !strings.Contains(intro, kw) {
						continue
					}
					var pType, classType int
					switch j.nType {
					case 2:
						pType, classType = ProductTypeEbook, ProductTypeEbook
					case 8:
						pType, classType = ProductTypeOdob, ProductTypeOdob
					default: // 4=课程
						pType, classType = 66, 1
					}
					matched = append(matched, Course{
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
				if len(matched) > 0 {
					mu.Lock()
					results = append(results, matched...)
					mu.Unlock()
				}
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()

	// 按 enid 去重
	seen := make(map[string]bool, len(results))
	var uniq []Course
	for _, c := range results {
		if c.Enid == "" || seen[c.Enid] {
			continue
		}
		seen[c.Enid] = true
		uniq = append(uniq, c)
		if len(uniq) >= limit {
			break
		}
	}
	return uniq, nil
}

// SearchEbook 搜索电子书。走正式分页接口，异常时退回 suggest。
func (s *Service) SearchEbook(keyword string, page, pageSize int) (result *SearchResult, err error) {
	if pageSize <= 0 {
		pageSize = 30
	}
	result, err = s.SearchEbookV2(keyword, page, pageSize)
	if err == nil && result != nil {
		return result, nil
	}
	return s.searchEbookSuggest(keyword, page, pageSize)
}

// SearchOdob 搜索听书。走正式分页接口，异常时退回 suggest。
func (s *Service) SearchOdob(keyword string, page, pageSize int) (result *SearchResult, err error) {
	if pageSize <= 0 {
		pageSize = 30
	}
	result, err = s.SearchOdobV2(keyword, page, pageSize)
	if err == nil && result != nil {
		return result, nil
	}
	return s.searchOdobSuggest(keyword, page, pageSize)
}

// searchEbookSuggest 旧的 suggest 版电子书搜索（每 tab 仅 2 条），仅作兜底
func (s *Service) searchEbookSuggest(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用 suggest API，searchType=2 只返回电子书
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

// searchOdobSuggest 旧的 suggest 版听书搜索（每 tab 仅 2 条），仅作兜底
func (s *Service) searchOdobSuggest(keyword string, page, pageSize int) (result *SearchResult, err error) {
	// 使用 suggest API，searchType=3 只返回听书
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
				Type:        ProductTypeOdob,
				ClassType:   ProductTypeOdob,
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
