package services

import (
	"strings"
	"sync"
)

// HallCache 大厅（向日葵）商品缓存。
//
// 设计要点（来自用户要求）：
//   - 不要在每次搜索时联网加载大厅，而是「启动时预加载一次」全部大厅商品到内存。
//   - 搜索时直接读内存 map/slice，O(n) 子串过滤只需毫秒，根本不联网。
//   - 每次重新打开 App 时刷新一遍这个缓存即可。
//
// 底层数据源是向日葵各分类导航的内容接口（SunflowerLabelContent），
// 这个接口本身就是大厅详情能正常工作的那个「正确调用」，不是会报「服务异常」的 suggest。
type HallCache struct {
	mu     sync.RWMutex
	items  []Course         // 全量商品，用于关键字过滤
	byEnid map[string]Course // enid -> Course，O(1) 精确查询
	loaded bool
	loading bool
	size   int
}

// GlobalHallCache 全局单例，整个进程共享一份大厅商品缓存。
var GlobalHallCache = &HallCache{
	byEnid: make(map[string]Course),
}

// HallCacheState 大厅缓存状态，供前端判断是否已就绪。
type HallCacheState struct {
	Loaded bool `json:"loaded"`
	Size   int  `json:"size"`
}

// RefreshHallCache 后台刷新大厅商品缓存（App 启动 / 登录成功后各调用一次）。
// 必须在已登录状态下调用，否则向日葵接口无权限会拉不到数据。
func (s *Service) RefreshHallCache() {
	go GlobalHallCache.Load(s)
}

// HallCacheStatus 返回大厅缓存状态。
func (s *Service) HallCacheStatus() HallCacheState {
	return HallCacheState{
		Loaded: GlobalHallCache.IsLoaded(),
		Size:   GlobalHallCache.Size(),
	}
}

// Load 遍历向日葵所有分类导航的全部分页，把商品灌进内存。
// 应在后台 goroutine 调用（启动时 / 登录后各一次），不要阻塞搜索请求。
// 单分类导航最多翻 maxPagesPerNav 页（防止极端情况下无限翻页），整体不做人为延迟，
// 仅靠网络往返自然限速；个别请求失败则跳过该页继续，保证缓存优雅降级。
func (c *HallCache) Load(s *Service) {
	c.mu.Lock()
	if c.loading {
		c.mu.Unlock()
		return
	}
	c.loading = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.loading = false
		c.mu.Unlock()
	}()

	const maxPagesPerNav = 200 // 单导航上限，正常远到不了
	const pageSize = 50

	type job struct {
		enid  string
		nType int
	}
	var jobs []job

	for _, nType := range []int{4, 2, 8} { // 课程、电子书、听书
		labelList, err := s.SunflowerLabelList(nType)
		if err != nil || labelList == nil {
			continue
		}
		for _, nav := range labelList.List {
			jobs = append(jobs, job{enid: nav.Enid, nType: nType})
		}
	}

	if len(jobs) == 0 {
		return
	}

	const workers = 4
	ch := make(chan job)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allProd []Course
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				page := 0
				for page < maxPagesPerNav {
					content, err := s.SunflowerLabelContent(j.enid, j.nType, page, pageSize)
					if err != nil || content == nil {
						break
					}
					var matched []Course
					for _, prod := range content.ProductList {
						if prod.ProductEnid == "" {
							continue
						}
						var pType, classType int
						switch j.nType {
						case 2: // 电子书
							pType, classType = ProductTypeEbook, ProductTypeEbook
						case 8: // 听书
							pType, classType = ProductTypeOdob, ProductTypeOdob
						default: // 4=课程
							pType, classType = 66, 1
						}
						matched = append(matched, Course{
							ID:          prod.Id,
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
						allProd = append(allProd, matched...)
						mu.Unlock()
					}
					if content.IsMore != 1 {
						break
					}
					page++
				}
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()

	// 去重 + 建索引
	seen := make(map[string]bool, len(allProd))
	uniq := make([]Course, 0, len(allProd))
	byEnid := make(map[string]Course, len(allProd))
	for _, p := range allProd {
		if p.Enid == "" || seen[p.Enid] {
			continue
		}
		seen[p.Enid] = true
		uniq = append(uniq, p)
		byEnid[p.Enid] = p
	}

	c.mu.Lock()
	c.items = uniq
	c.byEnid = byEnid
	c.size = len(uniq)
	c.loaded = true
	c.mu.Unlock()
}

// IsLoaded 缓存是否已就绪（可被搜索使用）。
func (c *HallCache) IsLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loaded
}

// Size 当前缓存商品总数。
func (c *HallCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.size
}

// Search 在大厅缓存中做大小写无关的子串匹配（标题/简介/作者），毫秒级。
// keyword 为空返回 nil。
func (c *HallCache) Search(keyword string, limit int) []Course {
	c.mu.RLock()
	items := c.items
	c.mu.RUnlock()
	if len(items) == 0 {
		return nil
	}
	kw := strings.ToLower(strings.TrimSpace(keyword))
	if kw == "" {
		return nil
	}
	if limit <= 0 {
		limit = 100
	}
	res := make([]Course, 0, limit)
	for _, it := range items {
		if len(res) >= limit {
			break
		}
		title := strings.ToLower(it.Title)
		intro := strings.ToLower(it.Intro)
		author := strings.ToLower(it.Author)
		if strings.Contains(title, kw) || strings.Contains(intro, kw) || strings.Contains(author, kw) {
			res = append(res, it)
		}
	}
	return res
}

// Get 按 enid 精确查询（O(1)），用于详情/书架等场景。
func (c *HallCache) Get(enid string) (Course, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.byEnid[enid]
	return v, ok
}
