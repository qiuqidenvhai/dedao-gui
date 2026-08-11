package services

import (
	"encoding/json"
	"errors"
	"strings"
)

// ShelfResult 加入/移出书架的统一结果
type ShelfResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	// Kind: ebook / odob / course
	Kind string `json:"kind"`
}

// shelfPost 直接发请求并解析得到统一的 {h:{c,e}} 头，把服务端的业务提示原样带回来。
func (s *Service) shelfPost(path string, body map[string]interface{}) (int, string, error) {
	resp, err := s.client.R().SetBody(body).Post(path)
	if err != nil {
		return -1, "", err
	}
	var raw struct {
		H struct {
			C int    `json:"c"`
			E string `json:"e"`
		} `json:"h"`
		C json.RawMessage `json:"c"`
	}
	if err := json.Unmarshal(resp.Body(), &raw); err != nil {
		return -1, "", err
	}
	return raw.H.C, raw.H.E, nil
}

// guessProductType 根据 enid 长度猜商品类型（电子书 enid 是 64 位，课程/听书是 30~40 位）
// 仅在前端没传类型时兜底。
func guessProductType(enid string) int {
	if len(enid) >= 60 {
		return ProductTypeEbook
	}
	return 0
}

// AddToShelf 把商品加入书架。
//
//	productType = 2  电子书 -> pc/ebook2/v1/bookshelf/add
//	productType = 13 听书   -> pc/odob/v2/bookrack/pc/add
//	productType = 66 课程   -> 得到没有「加入我的课程」接口，课程购买后自动进入我的课程
//
// productType 传 0 时按 enid 长度猜；猜不出来就电子书、听书都试一遍。
func (s *Service) AddToShelf(enid string, productType int) (*ShelfResult, error) {
	enid = strings.TrimSpace(enid)
	if enid == "" {
		return nil, errors.New("enid 为空")
	}
	if productType == 0 {
		productType = guessProductType(enid)
	}

	tryEbook := func() (*ShelfResult, error) {
		code, msg, err := s.shelfPost("pc/ebook2/v1/bookshelf/add", map[string]interface{}{"book_enids": []string{enid}})
		if err != nil {
			return nil, err
		}
		if code == 0 {
			return &ShelfResult{OK: true, Kind: "ebook", Message: "已加入电子书书架"}, nil
		}
		return &ShelfResult{OK: false, Kind: "ebook", Message: humanShelfErr(msg)}, nil
	}
	tryOdob := func() (*ShelfResult, error) {
		code, msg, err := s.shelfPost("pc/odob/v2/bookrack/pc/add", map[string]interface{}{"book_enids": []string{enid}})
		if err != nil {
			return nil, err
		}
		if code == 0 {
			return &ShelfResult{OK: true, Kind: "odob", Message: "已加入听书书架"}, nil
		}
		return &ShelfResult{OK: false, Kind: "odob", Message: humanShelfErr(msg)}, nil
	}

	switch productType {
	case ProductTypeEbook:
		return tryEbook()
	case ProductTypeOdob, 1013, 3:
		return tryOdob()
	case ProductTypeCourse, 4:
		return &ShelfResult{
			OK:      false,
			Kind:    "course",
			Message: "课程不支持加入书架：得到的课程在购买后会自动出现在「我的课程」里",
		}, nil
	}

	// 类型未知：先电子书后听书
	if r, err := tryEbook(); err == nil && r.OK {
		return r, nil
	}
	return tryOdob()
}

// RemoveFromShelf 从书架移出
func (s *Service) RemoveFromShelf(enid string, productType int) (*ShelfResult, error) {
	enid = strings.TrimSpace(enid)
	if enid == "" {
		return nil, errors.New("enid 为空")
	}
	if productType == 0 {
		productType = guessProductType(enid)
	}
	switch productType {
	case ProductTypeOdob, 1013, 3:
		// 注意：/pc/odob/v2/bookrack/remove 实测返回 104000 参数错误；
		// 必须用统一商品接口 /pc/hades/v1/product/remove（pids + ptype:13）。
		code, msg, err := s.shelfPost("/pc/hades/v1/product/remove", map[string]interface{}{"pids": []string{enid}, "ptype": 13})
		if err != nil {
			return nil, err
		}
		if code == 0 {
			return &ShelfResult{OK: true, Kind: "odob", Message: "已移出听书书架"}, nil
		}
		return &ShelfResult{OK: false, Kind: "odob", Message: humanShelfErr(msg)}, nil
	default:
		code, msg, err := s.shelfPost("pc/ebook2/v1/bookshelf/remove", map[string]interface{}{"book_enids": []string{enid}})
		if err != nil {
			return nil, err
		}
		if code == 0 {
			return &ShelfResult{OK: true, Kind: "ebook", Message: "已移出电子书书架"}, nil
		}
		return &ShelfResult{OK: false, Kind: "ebook", Message: humanShelfErr(msg)}, nil
	}
}

// humanShelfErr 把服务端英文/机器提示翻成人话
func humanShelfErr(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "neither vip or limit free"):
		return "这本听书需要电子书会员或已购买后才能加入书架"
	case strings.Contains(m, "already"):
		return "已经在书架里了"
	case msg == "":
		return "加入书架失败"
	}
	return msg
}
