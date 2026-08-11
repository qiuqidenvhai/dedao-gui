package services

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHallSearch 用 mock server 验证大厅检索：
//  1. 能检索到“科技共和国”（ProductEnid=KJGH_REPUBLIC_123）
//  2. 大小写无关匹配（英文标题小写命中）
//  3. 无匹配时返回空且不算错误
//  4. 会话失效（h.c!=0）时返回明确错误而非空结果
func TestHallSearch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/label/list"):
			w.Write([]byte(`{"h":{"c":0},"c":{"list":[{"enid":"NAV_EBOOK","name":"电子书"}]}}`))
		case strings.HasSuffix(r.URL.Path, "/label/content"):
			w.Write([]byte(`{"h":{"c":0},"c":{"product_list":[` +
				`{"product_enid":"KJGH_REPUBLIC_123","title":"科技共和国","intro":"科技改变世界","index_image":"http://img/x.png","author_list":["张三"]},` +
				`{"product_enid":"ENG_AI_001","title":"English Book About AI","intro":"ai intro","index_image":"http://img/y.png","author_list":["John"]}` +
				`],"is_more":0}}`))
		default:
			w.Write([]byte(`{"h":{"c":0},"c":{}}`))
		}
	}))
	defer ts.Close()

	s := NewService(&CookieOptions{})
	s.client.SetBaseURL(ts.URL)

	// 复位全局大厅缓存，避免被其它测试（如真实加载）污染导致走缓存分支
	GlobalHallCache = &HallCache{byEnid: make(map[string]Course)}

	// 1. 检索“科技共和国”
	list, err := s.HallSearch("科技共和国", 100)
	if err != nil {
		t.Fatalf("HallSearch returned error: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("HallSearch returned empty, want at least 1")
	}
	found := false
	for _, c := range list {
		if c.Enid == "KJGH_REPUBLIC_123" && strings.Contains(c.Title, "科技共和国") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected to find 科技共和国 (KJGH_REPUBLIC_123), got: %+v", list)
	}

	// 2. 大小写无关匹配（英文标题）
	list2, err := s.HallSearch("english book about ai", 100)
	if err != nil {
		t.Fatalf("case-insensitive HallSearch error: %v", err)
	}
	if len(list2) == 0 {
		t.Fatal("case-insensitive match failed for English title")
	}

	// 3. 无匹配：返回空且不报错
	list3, err := s.HallSearch("不存在的书xyz", 100)
	if err != nil {
		t.Fatalf("empty match should not error, got: %v", err)
	}
	if len(list3) != 0 {
		t.Fatalf("expected empty for non-existent, got %d", len(list3))
	}
}

// TestHallSearchSessionInvalid 模拟登录会话失效（h.c!=0），验证返回明确错误
func TestHallSearchSessionInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 所有请求都返回错误码，模拟会话失效
		w.Write([]byte(`{"h":{"c":1001,"e":"登录已失效"},"c":{}}`))
	}))
	defer ts.Close()

	s := NewService(&CookieOptions{})
	s.client.SetBaseURL(ts.URL)

	// 复位全局大厅缓存，避免被其它测试污染导致走缓存分支
	GlobalHallCache = &HallCache{byEnid: make(map[string]Course)}

	_, err := s.HallSearch("科技共和国", 100)
	if err == nil {
		t.Fatal("expected error when session invalid, got nil")
	}
	if !strings.Contains(err.Error(), "登录") && !strings.Contains(err.Error(), "大厅检索失败") {
		t.Fatalf("expected session-invalid error message, got: %v", err)
	}
}

// TestSearchAllHallFallback 验证统一搜索的真实调用路径（Course.vue 用的就是 SearchAll）：
// 当 suggest 建议接口对“科技共和国”无结果时，SearchAll 应自动走向日葵大厅兜底，
// 最终返回 ProductEnid=KJGH_REPUBLIC_123 的结果。这正是“大厅搜索框输入科技共和国”的行为。
func TestSearchAllHallFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		// suggest 建议接口：对科技共和国返回空列表，强制走大厅兜底
		case strings.HasSuffix(r.URL.Path, "/api/search/pc/suggest"):
			w.Write([]byte(`{"h":{"c":0},"c":{"list":[]}}`))
		case strings.HasSuffix(r.URL.Path, "/label/list"):
			w.Write([]byte(`{"h":{"c":0},"c":{"list":[{"enid":"NAV_EBOOK","name":"电子书"}]}}`))
		case strings.HasSuffix(r.URL.Path, "/label/content"):
			w.Write([]byte(`{"h":{"c":0},"c":{"product_list":[` +
				`{"product_enid":"KJGH_REPUBLIC_123","title":"科技共和国","intro":"科技改变世界","index_image":"http://img/x.png","author_list":["张三"]}` +
				`],"is_more":0}}`))
		default:
			w.Write([]byte(`{"h":{"c":0},"c":{}}`))
		}
	}))
	defer ts.Close()

	s := NewService(&CookieOptions{})
	s.client.SetBaseURL(ts.URL)

	result, err := s.SearchAll("科技共和国", 1, 20)
	if err != nil {
		t.Fatalf("SearchAll returned error: %v", err)
	}
	if result == nil || len(result.List) == 0 {
		t.Fatal("SearchAll returned empty, want at least 1 (via hall fallback)")
	}
	found := false
	for _, c := range result.List {
		if c.Enid == "KJGH_REPUBLIC_123" && strings.Contains(c.Title, "科技共和国") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected to find 科技共和国 (KJGH_REPUBLIC_123) via hall fallback, got: %+v", result.List)
	}
}
