package backend

import (
	"github.com/yann0917/dedao-gui/backend/config"
	"github.com/yann0917/dedao-gui/backend/services"
)

// sunflower.go 的所有方法都改走 config.Instance.ActiveUserService()，
// 而非 init() 时一次性建好的 backend.Instance。理由：
// backend.Instance 是启动时用空 cookie 创建的匿名会话，登录后从未刷新，
// 它返回的 GetHomeInitialState().isLogin 永远是 false，
// 直接导致 Home.vue 第 321 行的 getUserInfo() 不触发 → 首页右上用户卡全程空白。
// 走 ActiveUserService() 与 app.getService() 同路径，
// login 流程会 c.service=nil 重置缓存，setActiveUser 时拿到的是登录后的真实 cookie。

func (a *App) GetHomeInitialState() (state services.HomeInitState, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	state, err = config.Instance.ActiveUserService().GetHomeInitialState()
	return
}

func (a *App) SearchHot() (list *services.SearchTot, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	list, err = config.Instance.ActiveUserService().SearchHot()
	return
}

// SunflowerLabelList 首页导航标签列表
// 2-好看又好查的电子书, 4-精选课程
func (a *App) SunflowerLabelList(nType int) (list *services.SunflowerLabelList, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	list, err = config.Instance.ActiveUserService().SunflowerLabelList(nType)
	return
}

func (a *App) SunflowerLabelContent(enID string, nType, page, pageSize int) (list *services.SunflowerContent, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	list, err = config.Instance.ActiveUserService().SunflowerLabelContent(enID, nType, page, pageSize)
	return
}

func (a *App) SunflowerResourceList() (list *services.SunflowerResourceList, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	list, err = config.Instance.ActiveUserService().SunflowerResourceList()
	return
}

func (a *App) AlgoFilter(param services.AlgoFilterParam) (resp *services.AlgoFilterResp, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	resp, err = config.Instance.ActiveUserService().AlgoFilter(param)
	return
}

func (a *App) AlgoProduct(param services.AlgoFilterParam) (resp *services.AlgoProductResp, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	resp, err = config.Instance.ActiveUserService().AlgoProduct(param)
	return
}
