package services

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ===========================================================================
// 真·全站搜索（2026-08-10 抓 dedao.cn PC 前端 bundle 得到的正式分页搜索接口）
//
// 之前用的 /api/search/pc/suggest 是「搜索框联想」接口，每个 tab 最多只回 2 条，
// 这就是「一个关键词只搜出一两条、只有听书」的根因。
// 正式的搜索结果页用的是下面三个接口，全部支持 page/size 分页且返回 total：
//
//	课程   POST /api/search/v2/pc/searchclass       JSON  -> extra.token 即 enid
//	听书   POST /api/search/v2/pc/searchaudio       JSON  -> extra.token 即 enid
//	电子书 POST /api/search/pc/open/buytab/common   FORM  -> detail.enid 即 enid
//	       （type=2 & is_buy=0 表示「全站电子书」而不是「我已购的」）
//
// 商品类型常量（服务端真实取值，务必全链路统一）：
//
//	ProductTypeEbook  = 2   电子书
//	ProductTypeOdob   = 13  听书（历史代码里写成 3 是错的，会导致详情/加书架失败）
//	ProductTypeCourse = 66  课程
// ===========================================================================

const (
	// ProductTypeEbook 电子书
	ProductTypeEbook = 2
	// ProductTypeOdob 听书（每天听本书）
	ProductTypeOdob = 13
	// ProductTypeCourse 课程
	ProductTypeCourse = 66
)

// stripHL 去掉搜索结果里的 <hl> 高亮标签
func stripHL(s string) string {
	s = strings.ReplaceAll(s, "<hl>", "")
	s = strings.ReplaceAll(s, "</hl>", "")
	return s
}

// searchV2Resp 课程/听书搜索的响应结构
type searchV2Resp struct {
	Page   int `json:"page"`
	Size   int `json:"size"`
	Total  int `json:"total"`
	IsMore int `json:"is_more"`
	Type   int `json:"type"`
	List   []struct {
		ID      int    `json:"id"`
		Type    int    `json:"type"`
		Tname   string `json:"tname"`
		Title   string `json:"title"`
		Author  string `json:"author"`
		Content string `json:"content"`
		Image   string `json:"image"`
		Extra   struct {
			Token string `json:"token"`
			Enid  string `json:"enid"`
			Image string `json:"image"`
			Press string `json:"press"`
		} `json:"extra"`
		Detail struct {
			Enid  string `json:"enid"`
			IdOut string `json:"id_out"`
			IsBuy bool   `json:"is_buy"`
		} `json:"detail"`
	} `json:"list"`
}

// searchV2 统一发起一次分页搜索请求并归一化成 []Course
// path: 接口路径；form: 是否用 form 表单（电子书接口要求 x-www-form-urlencoded）
func (s *Service) searchV2(path string, form bool, keyword string, productType, page, size int) (*SearchResult, error) {
	if strings.TrimSpace(keyword) == "" {
		return &SearchResult{List: []Course{}, Total: 0}, nil
	}
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	req := s.client.R()
	if form {
		req = req.SetFormData(map[string]string{
			"content":    keyword,
			"type":       strconv.Itoa(productType),
			"size":       strconv.Itoa(size),
			"page":       strconv.Itoa(page),
			"request_id": "",
			"hl_num":     "1",
			"is_buy":     "0",
		})
	} else {
		req = req.SetBody(map[string]interface{}{
			"content":    keyword,
			"hl_num":     1,
			"page":       page,
			"request_id": "",
			"size":       size,
			"type":       0,
		})
	}

	resp, err := req.Post(path)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errors.New("搜索接口无响应")
	}

	var raw struct {
		H struct {
			C int    `json:"c"`
			E string `json:"e"`
		} `json:"h"`
		C searchV2Resp `json:"c"`
	}
	if err := json.Unmarshal(resp.Body(), &raw); err != nil {
		return nil, err
	}
	if raw.H.C != 0 && raw.H.E != "" {
		return nil, errors.New(raw.H.E)
	}

	list := make([]Course, 0, len(raw.C.List))
	for _, it := range raw.C.List {
		enid := it.Extra.Token
		if enid == "" {
			enid = it.Extra.Enid
		}
		if enid == "" {
			enid = it.Detail.Enid
		}
		if enid == "" {
			enid = it.Detail.IdOut
		}
		if enid == "" {
			// 没有 enid 就无法打开详情，跳过（避免点开报「服务器异常」）
			continue
		}
		pType := it.Type
		if pType == 0 {
			pType = productType
		}
		icon := it.Image
		if icon == "" {
			icon = it.Extra.Image
		}
		author := it.Author
		if author == "" && it.Extra.Press != "" {
			author = it.Extra.Press
		}
		list = append(list, Course{
			ID:          it.ID,
			ClassID:     it.ID, // 搜索结果的 ID 就是 class_id，用于下载
			Enid:        enid,
			Type:        pType,
			ClassType:   pType,
			Title:       stripHL(it.Title),
			Intro:       stripHL(it.Content),
			Author:      stripHL(author),
			Icon:        icon,
			LogType:     it.Tname,
			IsCollected: false,
			IsBuy:       it.Detail.IsBuy,
		})
	}

	total := raw.C.Total
	if total == 0 {
		total = len(list)
	}
	return &SearchResult{List: list, Total: total}, nil
}

// SearchCourseV2 全站搜索「课程」（分页，支持翻页到底）
func (s *Service) SearchCourseV2(keyword string, page, size int) (*SearchResult, error) {
	return s.searchV2("/api/search/v2/pc/searchclass", false, keyword, ProductTypeCourse, page, size)
}

// SearchOdobV2 全站搜索「听书」（分页）
func (s *Service) SearchOdobV2(keyword string, page, size int) (*SearchResult, error) {
	return s.searchV2("/api/search/v2/pc/searchaudio", false, keyword, ProductTypeOdob, page, size)
}

// SearchEbookV2 全站搜索「电子书」（分页）
func (s *Service) SearchEbookV2(keyword string, page, size int) (*SearchResult, error) {
	return s.searchV2("/api/search/pc/open/buytab/common", true, keyword, ProductTypeEbook, page, size)
}

// SearchProductsAll 全站搜索：课程 + 听书 + 电子书 三路并发，合并返回（第 1 页）。
func (s *Service) SearchProductsAll(keyword string, perType int) (*SearchResult, error) {
	return s.SearchProductsAllPaged(keyword, 1, perType)
}

// SearchProductsAllPaged 全站搜索（带分页）：课程 + 听书 + 电子书 三路并发，合并返回。
//
// perType 表示每一类每页最多取多少条（0 表示默认 30，上限 100 —— 服务端单页上限）。
// 三类各取第 page 页，结果按「课程 → 听书 → 电子书」排列，同一 enid 去重。
// total 返回三类 total 之和，让前端知道总共命中了多少。
func (s *Service) SearchProductsAllPaged(keyword string, page, perType int) (*SearchResult, error) {
	if strings.TrimSpace(keyword) == "" {
		return &SearchResult{List: []Course{}, Total: 0}, nil
	}
	if perType <= 0 {
		perType = 30
	}
	if perType > 100 {
		perType = 100
	}
	if page <= 0 {
		page = 1
	}

	type part struct {
		order int
		res   *SearchResult
		err   error
	}
	results := make([]part, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		r, err := s.SearchCourseV2(keyword, page, perType)
		results[0] = part{0, r, err}
	}()
	go func() {
		defer wg.Done()
		r, err := s.SearchOdobV2(keyword, page, perType)
		results[1] = part{1, r, err}
	}()
	go func() {
		defer wg.Done()
		r, err := s.SearchEbookV2(keyword, page, perType)
		results[2] = part{2, r, err}
	}()
	wg.Wait()

	sort.SliceStable(results, func(i, j int) bool { return results[i].order < results[j].order })

	var (
		merged  []Course
		total   int
		seen    = map[string]bool{}
		lastErr error
		anyOK   bool
	)
	for _, p := range results {
		if p.err != nil {
			lastErr = p.err
			continue
		}
		if p.res == nil {
			continue
		}
		anyOK = true
		total += p.res.Total
		for _, c := range p.res.List {
			if c.Enid == "" || seen[c.Enid] {
				continue
			}
			seen[c.Enid] = true
			merged = append(merged, c)
		}
	}
	if !anyOK && lastErr != nil {
		return nil, lastErr
	}
	if merged == nil {
		merged = []Course{}
	}
	return &SearchResult{List: merged, Total: total}, nil
}

// SearchProductsByType 按商品类型分页搜索（前端「课程/听书/电子书」分 tab 时用）
func (s *Service) SearchProductsByType(keyword string, productType, page, size int) (*SearchResult, error) {
	switch productType {
	case ProductTypeEbook:
		return s.SearchEbookV2(keyword, page, size)
	case ProductTypeOdob:
		return s.SearchOdobV2(keyword, page, size)
	case ProductTypeCourse:
		return s.SearchCourseV2(keyword, page, size)
	default:
		return s.SearchProductsAllPaged(keyword, page, size)
	}
}
