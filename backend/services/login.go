package services

import (
	"fmt"
	"strings"
)

// QrCodeResp 扫码登录响应
type QrCodeResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		QrCode       string `json:"qrcode"`
		QrCodeString string `json:"qrCodeString"`
	} `json:"data"`
}

// CheckLoginResp 扫码登录检查结果
type CheckLoginResp struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
	Data    struct {
		Status int `json:"status"` // 1-扫码成功,2-过期
	} `json:"data"`
}

// LoginAccessToken get login access token
func (s *Service) LoginAccessToken() (token string, err error) {
	token, err = s.reqGetLoginAccessToken(CsrfToken)
	if err != nil {
		return
	}
	return
}

func (s *Service) GetQrcode(token string) (resp *QrCodeResp, err error) {
	resp, err = s.reqGetQrcode(token)
	if err != nil {
		return
	}
	return
}

func (s *Service) CheckLogin(token, qrcode string) (check *CheckLoginResp, cookie string, err error) {
	check, cookie, err = s.reqCheckLogin(token, qrcode)
	if err != nil {
		return
	}
	return
}

// PhoneSendCodeResp 发送验证码结果
type PhoneSendCodeResp struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	NeedCaptcha bool   `json:"needCaptcha"`
	CaptchaID   string `json:"captchaId,omitempty"`
}

// PhoneLoginResp 手机号登录结果
type PhoneLoginResp struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Cookie  string `json:"cookie"`
	User    *User  `json:"user"`
}

// isValidPhone 校验中国大陆手机号格式
func isValidPhone(phone string) bool {
	if len(phone) != 11 || phone[0] != '1' {
		return false
	}
	for _, char := range phone {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// SendSMSCode 向指定手机号发送短信验证码（真实调用 dedao 手机号登录 API）
// captcha 为易盾滑块 validate token；首次发送传空串。needCaptcha=true 时短信不会发出，需滑完滑块后用 validate 重发。
func SendSMSCode(service *Service, token, phone, captcha string) (result *PhoneSendCodeResp, err error) {
	phone = strings.TrimSpace(phone)
	captcha = strings.TrimSpace(captcha)
	if service == nil {
		return nil, fmt.Errorf("登录服务未初始化")
	}
	if !isValidPhone(phone) {
		return nil, fmt.Errorf("请输入有效的11位手机号")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("登录令牌为空")
	}

	resp, err := service.reqSendSMSCode(token, phone, captcha)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("发送验证码未返回结果")
	}

	// needCaptcha=true 表示需要滑块验证，短信尚未真正发出，不能视为成功
	if resp.Data.NeedCaptcha {
		return &PhoneSendCodeResp{
			Success:     false,
			Message:     "请完成滑块验证",
			NeedCaptcha: true,
			CaptchaID:   resp.Data.CaptchaID,
		}, nil
	}

	message := resp.ErrMsg
	if message == "" && resp.ErrCode == 0 {
		message = "验证码已发送"
	}
	result = &PhoneSendCodeResp{
		Success:     resp.ErrCode == 0,
		Message:     message,
		NeedCaptcha: resp.Data.NeedCaptcha,
		CaptchaID:   resp.Data.CaptchaID,
	}
	return
}

// SendSMSCodeWithCaptcha 发送短信验证码（带易盾 validate token 重发）
func SendSMSCodeWithCaptcha(service *Service, token, phone, captchaToken string) (result *PhoneSendCodeResp, err error) {
	phone = strings.TrimSpace(phone)
	captchaToken = strings.TrimSpace(captchaToken)
	if service == nil {
		return nil, fmt.Errorf("登录服务未初始化")
	}
	if !isValidPhone(phone) {
		return nil, fmt.Errorf("请输入有效的11位手机号")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("登录令牌为空")
	}

	resp, err := service.reqSendSMSCodeWithCaptcha(token, phone, captchaToken)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("发送验证码未返回结果")
	}

	if resp.Data.NeedCaptcha {
		return &PhoneSendCodeResp{
			Success:     false,
			Message:     "请完成滑块验证",
			NeedCaptcha: true,
			CaptchaID:   resp.Data.CaptchaID,
		}, nil
	}

	message := resp.ErrMsg
	if message == "" && resp.ErrCode == 0 {
		message = "验证码已发送"
	}
	result = &PhoneSendCodeResp{
		Success:     resp.ErrCode == 0,
		Message:     message,
		NeedCaptcha: resp.Data.NeedCaptcha,
		CaptchaID:   resp.Data.CaptchaID,
	}
	return
}

// PhoneLoginWithCode 手机号+验证码登录（真实调用 dedao 手机号登录 API）
func PhoneLoginWithCode(service *Service, token, phone, code string) (result *PhoneLoginResp, err error) {
	phone = strings.TrimSpace(phone)
	code = strings.TrimSpace(code)
	if service == nil {
		return nil, fmt.Errorf("登录服务未初始化")
	}
	if !isValidPhone(phone) {
		return nil, fmt.Errorf("请输入有效的11位手机号")
	}
	if len(code) != 6 {
		return nil, fmt.Errorf("验证码应为6位数字")
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return nil, fmt.Errorf("验证码应为6位数字")
		}
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("登录令牌为空，请重新发送验证码")
	}

	cookie, err := service.reqPhoneLogin(token, code)
	if err != nil {
		return nil, err
	}

	result = &PhoneLoginResp{
		Success: true,
		Message: "登录成功",
		Cookie:  cookie,
	}
	return
}
