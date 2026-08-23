package config

import (
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/yann0917/dedao-gui/backend/services"
)

const (
	// EnvConfigDir 配置路径环境变量
	EnvConfigDir = "DEDAO_GO_CONFIG_DIR"
	// Name 配置文件名
	Name = "config.json"
)

var (
	configFilePath = filepath.Join(GetConfigDir(), Name)

	// Instance 配置信息 全局调用
	Instance *ConfigsData
)

func init() {
	Instance = new(ConfigsData)
	Instance.configFilePath = configFilePath
	if err := Instance.init(); err != nil {
		// 配置加载失败（文件不可读/被占用/缺失）时不再致命退出，
		// 改为使用空配置继续启动，保证程序一定能打开（显示登录界面）。
		log.Println("warn: config init failed, continue with empty config:", err)
	}
}

// DedaoUsers user
type DedaoUsers []*Dedao

// ConfigsData Configs data
type ConfigsData struct {
	AcitveUID      string
	DownloadPath   string
	Users          DedaoUsers
	Settings       SettingsData
	activeUser     *Dedao
	configFilePath string
	configFile     *os.File
	fileMu         sync.Mutex
	service        *services.Service
}

// SettingsData 持久化的界面/工具设置。
// 早期版本仅依赖前端 localStorage，但主窗口 WebView2 数据目录改为每进程临时目录后，
// localStorage 每次启动都是空的，导致主题/字体/工具路径等设置丢失。
// 这里把所有需要持久化的设置统一落到后端 config.json（%APPDATA%/dedao-gui）。
type SettingsData struct {
	Theme          string `json:"theme"`
	Color          string `json:"color"`
	FfmpegDir      string `json:"ffmpegDir"`
	WkhtmltopdfDir string `json:"wkhtmltopdfDir"`
	FontFamily     string `json:"fontFamily"`
}

type configJSONExport struct {
	AcitveUID    string
	DownloadPath string
	Users        DedaoUsers
	Settings     SettingsData
}

// Init 初始化配置
func (c *ConfigsData) init() error {
	if c.configFilePath == "" {
		return errors.New("配置文件未找到")
	}

	// 从配置文件中加载配置
	err := c.loadConfigFromFile()
	if err != nil {
		return err
	}

	// 初始化登陆用户信息
	err = c.initActiveUser()
	if err != nil {
		return nil
	}

	if c.activeUser != nil {
		c.service = c.activeUser.New()
	}

	return nil
}

func (c *ConfigsData) initActiveUser() error {
	// 如果已经初始化过，则跳过
	if c.AcitveUID != "" && c.activeUser != nil && c.activeUser.UIDHazy == c.AcitveUID {
		return nil
	}

	if c.AcitveUID == "" && c.activeUser != nil {
		c.AcitveUID = c.activeUser.UIDHazy
		return nil
	}

	if c.AcitveUID != "" {
		for _, user := range c.Users {
			if user.UIDHazy == c.AcitveUID {
				c.activeUser = user
				return nil
			}
		}
	}

	if c.AcitveUID == "" && len(c.Users) == 0 {
		c.activeUser = new(Dedao)
	}

	if len(c.Users) > 0 {
		return errors.New("存在登录的用户，可以进行切换登录用户")
	}

	return errors.New("未登陆")
}

// Save 保存配置
func (c *ConfigsData) Save() error {
	err := c.lazyOpenConfigFile()
	if err != nil {
		return err
	}

	c.fileMu.Lock()
	defer c.fileMu.Unlock()

	// 保存配置的数据
	conf := configJSONExport{
		AcitveUID:    c.AcitveUID,
		DownloadPath: c.DownloadPath,
		Users:        c.Users,
		Settings:     c.Settings,
	}

	data, err := jsoniter.MarshalIndent(conf, "", " ")

	if err != nil {
		panic(err)
	}

	// 减掉多余的部分
	err = c.configFile.Truncate(int64(len(data)))
	if err != nil {
		// fmt.Println(err)
		return err
	}

	_, err = c.configFile.Seek(0, io.SeekStart)
	if err != nil {
		// fmt.Println(err)
		return err
	}

	_, err = c.configFile.Write(data)
	if err != nil {
		// fmt.Println(err)
		return err
	}

	return nil
}

func (c *ConfigsData) loadConfigFromFile() error {
	err := c.lazyOpenConfigFile()
	if err != nil {
		return err
	}

	info, err := c.configFile.Stat()
	if err != nil {
		return err
	}

	if info.Size() == 0 {
		return c.Save()
	}

	c.fileMu.Lock()
	defer c.fileMu.Unlock()

	_, err = c.configFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil
	}

	// 从配置文件中加载配置
	decoder := jsoniter.NewDecoder(c.configFile)
	var conf configJSONExport
	decoder.Decode(&conf)

	c.AcitveUID = conf.AcitveUID
	c.DownloadPath = conf.DownloadPath
	c.Users = conf.Users
	c.Settings = conf.Settings
	return nil
}

func (c *ConfigsData) lazyOpenConfigFile() (err error) {
	if c.configFile != nil {
		return nil
	}
	c.fileMu.Lock()
	os.MkdirAll(filepath.Dir(c.configFilePath), 0700)
	c.configFile, err = os.OpenFile(c.configFilePath, os.O_CREATE|os.O_RDWR, 0600)
	c.fileMu.Unlock()

	if err != nil {
		if os.IsPermission(err) {
			return
		}
		if os.IsExist(err) {
			return
		}
		return
	}
	return
}

func (c *ConfigsData) DeleteConfigFile() (err error) {
	if c.configFilePath == "" {
		return nil
	}
	// 先关闭文件句柄，否则 Windows 上无法删除
	c.fileMu.Lock()
	if c.configFile != nil {
		c.configFile.Close()
		c.configFile = nil
	}
	c.fileMu.Unlock()
	err = os.Remove(c.configFilePath)
	if os.IsNotExist(err) {
		return nil
	}
	return
}

// Reset 清除所有登录状态（内存数据）
func (c *ConfigsData) Reset() {
	c.AcitveUID = ""
	c.Users = nil
	c.activeUser = nil
	c.service = nil
}

// New config
func New(configFilePath string) *ConfigsData {
	c := &ConfigsData{
		configFilePath: configFilePath,
	}

	return c
}

// GetConfigDir config file dir
// 优先使用 DEDAO_GO_CONFIG_DIR 环境变量（绝对路径）；否则使用各平台的持久化用户配置目录：
// Windows -> %APPDATA%/dedao-gui，macOS/Linux -> $XDG_CONFIG_HOME 或 ~/.config/dedao-gui。
// 关键修复：原实现在 Windows（HOME 通常未设置）会回退到 /tmp/dedao，该路径易失且不可靠，
// 导致配置（登录态、设置）每次关闭都被丢弃。改用 os.UserConfigDir() 指向系统持久化目录。
func GetConfigDir() string {
	configDir, ok := os.LookupEnv(EnvConfigDir)
	if ok && filepath.IsAbs(configDir) {
		return configDir
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "dedao-gui")
	}
	if home, ok := os.LookupEnv("HOME"); ok {
		return filepath.Join(home, ".config", "dedao")
	}
	return filepath.Join(os.TempDir(), "dedao")
}

// GetDownloadPath 返回已保存的下载目录（未设置时为空字符串）
func (c *ConfigsData) GetDownloadPath() string {
	return c.DownloadPath
}

// SetDownloadPath 更新并持久化下载目录
func (c *ConfigsData) SetDownloadPath(p string) error {
	c.DownloadPath = p
	return c.Save()
}

// GetSettings 返回已保存的界面/工具设置
func (c *ConfigsData) GetSettings() SettingsData {
	return c.Settings
}

// SaveSettings 更新并持久化界面/工具设置
func (c *ConfigsData) SaveSettings(s SettingsData) error {
	c.Settings = s
	return c.Save()
}

// ActiveUserService user
func (c *ConfigsData) ActiveUserService() *services.Service {
	if c.service == nil {
		// 如果没有 activeUser，创建一个空的（用于获取登录页等公开接口）
		if c.activeUser == nil {
			c.activeUser = new(Dedao)
		}
		c.service = c.activeUser.New()
	}
	return c.service
}

// SetUser set user
func (c *ConfigsData) SetUser(u *Dedao) (*Dedao, *services.User, error) {
	ser := services.NewService(&u.CookieOptions)
	user, err := ser.User()
	if err != nil {
		return nil, nil, err
	}

	c.DeleteUser(&User{UIDHazy: user.UIDHazy})

	dedao := &Dedao{
		User: User{
			UIDHazy: user.UIDHazy,
			Name:    user.Nickname,
			Avatar:  user.Avatar,
		},
		CookieOptions: u.CookieOptions,
	}
	c.Users = append(c.Users, dedao)
	c.setActiveUser(dedao)
	return dedao, user, nil
}

// DeleteUser delete
func (c *ConfigsData) DeleteUser(u *User) {
	for k, user := range c.Users {
		if user.UIDHazy == u.UIDHazy {
			c.Users = append(c.Users[:k], c.Users[k+1:]...)
			break
		}
	}
}

// ActiveUser active user
func (c *ConfigsData) ActiveUser() *Dedao {
	return c.activeUser
}

func (c *ConfigsData) setActiveUser(u *Dedao) {
	c.AcitveUID = u.UIDHazy
	c.activeUser = u
	// 关键：切换/设置活跃用户时，重置缓存的 service。
	// 否则 getService() 仍返回登录前创建的旧实例（空 cookie），
	// 导致登录成功后 UserInfo/EbookUserInfo 等接口继续用旧 cookie 请求，
	// 返回空数据或 401（表现为主程序“登录了但资料全空”）。
	c.service = nil
}

// LoginUserCount 登录用户数量
func (c *ConfigsData) LoginUserCount() int {
	return len(c.Users)
}

// SwitchUser switch user
func (c *ConfigsData) SwitchUser(u *User) error {
	for _, user := range c.Users {
		if user.UIDHazy == u.UIDHazy {
			c.setActiveUser(user)
			err := c.Save()
			return err
		}
	}
	return errors.New("用户不存在")
}
