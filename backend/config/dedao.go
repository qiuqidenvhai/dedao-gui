package config

import (
	"github.com/yann0917/dedao-gui/backend/services"
)

// User dedao user info
type User struct {
	UIDHazy string `json:"uid_hazy"`
	Name    string `json:"name"`
	Avatar  string `json:"avatar"`
}

// Dedao geek time info
type Dedao struct {
	User
	services.CookieOptions
}

// WeChatConfig 微信配置
type WeChatConfig struct {
	AppID       string `json:"app_id"`
	AppSecret   string `json:"app_secret"`
	RedirectURI string `json:"redirect_uri"`
}

// New dedao service
func (d *Dedao) New() *services.Service {
	return services.NewService(&d.CookieOptions)
}
