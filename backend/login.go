package backend

import (
	"errors"

	"strings"

	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/config"
	"github.com/yann0917/dedao-gui/backend/services"
)

type QrCodeResp struct {
	Token        string `json:"token"`
	QrCode       string `json:"qrCode"`
	QrCodeString string `json:"qrCodeString"`
}

type LoginResult struct {
	Status int            `json:"status"` // 1-登录成功,2-二维码过期
	Cookie string         `json:"cookie"` // cookies string
	User   *services.User `json:"user"`
}

type PhoneCodeResult struct {
	Token       string `json:"token"`
	Message     string `json:"message"`
	NeedCaptcha bool   `json:"needCaptcha"`
	CaptchaID   string `json:"captchaId,omitempty"`
}

var Instance *services.Service

func init() {
	Instance = config.Instance.ActiveUserService()
}
func (a *App) GetQrcode() (qrCode QrCodeResp, err error) {
	if Instance == nil {
		Instance = config.Instance.ActiveUserService()
	}
	if services.CsrfToken == "" {
		if _, err = Instance.GetHomeInitialState(); err != nil {
			return
		}
	}
	token, err := Instance.LoginAccessToken()
	if err != nil {
		return
	}
	if strings.Contains(token, "invalid csrf token") {
		app.Logout()
		services.CsrfToken = ""
		_, _ = Instance.GetHomeInitialState()
		token, err = Instance.LoginAccessToken()
		if err != nil {
			return
		}
	}
	code, err := Instance.GetQrcode(token)
	if err != nil {
		return
	}
	qrCode.Token = token
	if code != nil {
		qrCode.QrCode = code.Data.QrCode
		qrCode.QrCodeString = code.Data.QrCodeString
	}
	return
}

func (a *App) CheckLogin(token, qrCodeString string) (result LoginResult, err error) {
	check, cookie, err := Instance.CheckLogin(token, qrCodeString)
	if err != nil {
		return
	}
	result.Cookie = cookie
	if check != nil {
		if check.Data.Status == 1 {
			result.User, err = app.LoginByCookie(cookie)
			if err != nil {
				return
			}

			// fmt.Println("扫码成功")
		} else if check.Data.Status == 2 {
			err = errors.New("登录失败，二维码已过期")
			return
		}
		result.Status = check.Data.Status
	}
	return
}

// SendPhoneCode 复用扫码登录的匿名会话获取 token 并发送手机验证码。
// 返回的 token 必须原样传给 PhoneLogin，不能重新申请。
func (a *App) SendPhoneCode(phone string) (result PhoneCodeResult, err error) {
	if Instance == nil {
		Instance = config.Instance.ActiveUserService()
	}
	if Instance == nil {
		return result, errors.New("登录服务初始化失败")
	}

	if services.CsrfToken == "" {
		if _, err = Instance.GetHomeInitialState(); err != nil {
			return result, err
		}
	}
	token, err := Instance.LoginAccessToken()
	if err != nil {
		return result, err
	}
	if strings.Contains(token, "invalid csrf token") || strings.TrimSpace(token) == "" {
		services.CsrfToken = ""
		if _, err = Instance.GetHomeInitialState(); err != nil {
			return result, err
		}
		token, err = Instance.LoginAccessToken()
		if err != nil {
			return result, err
		}
	}

	sendResult, err := services.SendSMSCode(Instance, token, phone, "")
	if err != nil {
		return result, err
	}
	if !sendResult.Success {
		if sendResult.Message == "" {
			return result, errors.New("发送验证码失败")
		}
		return result, errors.New(sendResult.Message)
	}

	result.Token = token
	result.Message = sendResult.Message
	result.NeedCaptcha = sendResult.NeedCaptcha
	result.CaptchaID = sendResult.CaptchaID
	return result, nil
}

// SendPhoneCodeWithCaptcha 发送手机验证码（带验证码token）
func (a *App) SendPhoneCodeWithCaptcha(phone, captchaToken string) (result PhoneCodeResult, err error) {
	if Instance == nil {
		Instance = config.Instance.ActiveUserService()
	}
	if Instance == nil {
		return result, errors.New("登录服务初始化失败")
	}

	if services.CsrfToken == "" {
		if _, err = Instance.GetHomeInitialState(); err != nil {
			return result, err
		}
	}
	token, err := Instance.LoginAccessToken()
	if err != nil {
		return result, err
	}
	if strings.Contains(token, "invalid csrf token") || strings.TrimSpace(token) == "" {
		services.CsrfToken = ""
		if _, err = Instance.GetHomeInitialState(); err != nil {
			return result, err
		}
		token, err = Instance.LoginAccessToken()
		if err != nil {
			return result, err
		}
	}

	sendResult, err := services.SendSMSCodeWithCaptcha(Instance, token, phone, captchaToken)
	if err != nil {
		return result, err
	}
	if !sendResult.Success {
		if sendResult.Message == "" {
			return result, errors.New("发送验证码失败")
		}
		return result, errors.New(sendResult.Message)
	}

	result.Token = token
	result.Message = sendResult.Message
	result.NeedCaptcha = sendResult.NeedCaptcha
	result.CaptchaID = sendResult.CaptchaID
	return result, nil
}

// PhoneLogin 校验手机验证码，写入与扫码登录完全相同的本地登录态。
func (a *App) PhoneLogin(token, phone, code string) (result services.PhoneLoginResp, err error) {
	if Instance == nil {
		return result, errors.New("登录会话已失效，请重新发送验证码")
	}

	phoneResult, err := services.PhoneLoginWithCode(Instance, token, phone, code)
	if err != nil {
		return result, err
	}
	user, err := app.LoginByCookie(phoneResult.Cookie)
	if err != nil {
		return result, err
	}

	phoneResult.User = user
	return *phoneResult, nil
}

func (a *App) Logout() (err error) {
	// 先删除持久化配置；即使文件已经不存在也视为退出成功。
	if err = config.Instance.DeleteConfigFile(); err != nil {
		return err
	}
	services.ClearServiceState()
	config.Instance.Reset()
	// 退出后立即重建匿名会话，保证扫码和手机号登录共用一套干净的 cookie jar。
	Instance = config.Instance.ActiveUserService()
	if Instance == nil {
		return errors.New("登录服务重新初始化失败")
	}
	_, err = Instance.GetHomeInitialState()
	return err
}

// EnsureInstance 确保 backend.Instance 可用，不可用时返回错误
func EnsureInstance() error {
	if Instance == nil {
		Instance = config.Instance.ActiveUserService()
		if Instance == nil {
			return errors.New("未登录")
		}
	}
	return nil
}

func (a *App) UserInfo() (user *services.User, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	// backend.Instance 是 init() 一次性创建的旧引用（在登录前用空 cookie），
	// 走 config.Instance.ActiveUserService()：后者在 setActiveUser 里被 c.service=nil
	// 重置过，能拿到新 cookie 的 service（与 app.getService() 同一路径，下载/ebook 已验证）。
	user, err = config.Instance.ActiveUserService().User()
	return
}

func (a *App) EbookUserInfo() (user *services.EbookVIPInfo, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	user, err = config.Instance.ActiveUserService().EbookUserInfo()
	return
}

func (a *App) OdobUserInfo() (user *services.OdobVip, err error) {
	if err = EnsureInstance(); err != nil {
		return
	}
	user, err = config.Instance.ActiveUserService().OdobUserInfo()
	return
}
