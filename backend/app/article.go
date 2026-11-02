package app

import (
	"fmt"
	"math"
	"sync"

	"github.com/yann0917/dedao-gui/backend/services"
)

// ArticleList 已购课程文章列表分页
func ArticleList(enid, chapterID string, count, maxID int, reverse bool) (list *services.ArticleList, err error) {
	list, err = getService().ArticleList(enid, chapterID, count, maxID, reverse)
	if err != nil {
		return
	}
	return
}

// ArticleListAll 已购课程文章列表
func ArticleListAll(id int, chapterID string) (list *services.ArticleList, err error) {
	info, err := CourseDetail(CateCourse, id)
	if err != nil {
		return
	}
	enid := info.Enid
	count := info.PublishNum
	return ArticleListAllByInfo(enid, count, chapterID)

}

// 在途去重：同一门课的「拉全量文章」在并发时只真正发一次请求。
// 批量下载时 5 个 worker 会同时调用这里，不去重的话每次都要翻 3 页
// （133 篇 ≈ 3 次请求），白白把接口和内存都拖慢。
// 这里只共享「正在进行中的那一次」，请求结束立刻从 map 移除并释放引用，
// 不做常驻缓存，所以不会占额外内存、也不会有数据过期问题。
type articleAllCall struct {
	wg   sync.WaitGroup
	list *services.ArticleList
	err  error
}

var (
	articleAllInflightMu sync.Mutex
	articleAllInflight   = map[string]*articleAllCall{}
)

// ArticleListAllByInfo 根据 Enid 和数量获取文章列表
func ArticleListAllByInfo(enid string, count int, chapterID string) (list *services.ArticleList, err error) {
	key := enid + "|" + chapterID

	articleAllInflightMu.Lock()
	if c, ok := articleAllInflight[key]; ok {
		articleAllInflightMu.Unlock()
		c.wg.Wait()
		return c.list, c.err
	}
	c := &articleAllCall{}
	c.wg.Add(1)
	articleAllInflight[key] = c
	articleAllInflightMu.Unlock()

	defer func() {
		c.list, c.err = list, err
		articleAllInflightMu.Lock()
		delete(articleAllInflight, key)
		articleAllInflightMu.Unlock()
		c.wg.Done()
	}()

	// 单页 50 条：得到接口实测 count=50 正常、52 会报 h.c=104000「服务异常」，
	// 所以 50 是安全上限。133 篇从 5 次请求降到 3 次，返回数据量不变（不额外占内存）。
	pageSize := 50
	page := int(math.Ceil(float64(count) / float64(pageSize)))
	maxID := 0
	var lists []services.ArticleIntro
	for i := 0; i < page; i++ {
		list, err = getService().ArticleList(enid, chapterID, pageSize, maxID, false)
		if err != nil {
			return
		}
		if len(list.List) > 0 {
			maxID = list.List[len(list.List)-1].ID
			lists = append(lists, list.List...)
		}
	}
	// 如果 count 为 0 或者 page 为 0，list 可能为 nil
	if list == nil {
		list = &services.ArticleList{}
	}
	list.List = lists
	return
}

// ArticleDetail article detail
func ArticleDetail(enid string) (detail *services.ArticleDetail, err error) {
	info, err := getService().ArticleInfo(enid, 1)
	if err != nil {
		return
	}
	token := info.DdArticleToken
	appid := "1632426125495894021"
	detail, err = getService().ArticleDetail(token, enid, appid)
	if err != nil {
		return
	}
	return

}

// ArticleCommentList article comment list
func ArticleCommentList(enId, sort string, page, limit, sType int) (list *services.ArticleCommentList, err error) {
	list, err = getService().ArticleCommentList(enId, sort, page, limit, sType)
	if err != nil {
		return
	}

	return

}

// OdobArticleInfo article info
// get article token, audio token, media security token etc.
func OdobArticleInfo(aEnid string) (info *services.ArticleInfo, err error) {
	info, err = getService().ArticleInfo(aEnid, 2)
	if err != nil {
		return
	}
	return
}

// OdobArticleDetail odob article detail
func OdobArticleDetail(aEnid string) (detail *services.ArticleDetail, err error) {
	info, err := OdobArticleInfo(aEnid)
	if err != nil {
		return
	}

	token := info.DdArticleToken
	appid := "1632426125495894021"
	detail, err = getService().ArticleDetail(token, aEnid, appid)
	if err != nil {
		fmt.Printf("err:%#v\n", err)
		return
	}
	return

}

// ArticleDetailByEnid odob article detail
// enid article enid  or odob audioAliasID, aType 1-course article, 2-odob article
func ArticleDetailByEnid(aType int, aEnid string) (detail *services.ArticleDetail, err error) {
	info, err := getService().ArticleInfo(aEnid, aType)
	if err != nil {
		return
	}

	token := info.DdArticleToken
	appid := "1632426125495894021"
	detail, err = getService().ArticleDetail(token, aEnid, appid)
	if err != nil {
		fmt.Printf("err:%#v\n", err)
		return
	}
	return

}
