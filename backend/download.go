package backend

import (
	"fmt"
	"os"
	"runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yann0917/dedao-gui/backend/app"
	"github.com/yann0917/dedao-gui/backend/services"
	"github.com/yann0917/dedao-gui/backend/utils"
)

func (a *App) OpenDirectoryDialog(title string) (dir string, err error) {
	home, _ := os.LookupEnv("HOME")
	dialogOptions := wailsruntime.OpenDialogOptions{
		DefaultDirectory:           home,
		Title:                      title,
		ShowHiddenFiles:            false,
		CanCreateDirectories:       true,
		ResolvesAliases:            false,
		TreatPackagesAsDirectories: false,
	}
	dir, err = wailsruntime.OpenDirectoryDialog(a.Ctx, dialogOptions)
	app.SetOutputDir(dir)
	return
}

func (a *App) OpenFileDialog(title string) (file string, err error) {
	home, _ := os.LookupEnv("HOME")
	dialogOptions := wailsruntime.OpenDialogOptions{
		DefaultDirectory:           home,
		Title:                      title,
		ShowHiddenFiles:            false,
		CanCreateDirectories:       false,
		ResolvesAliases:            false,
		TreatPackagesAsDirectories: false,
	}
	file, err = wailsruntime.OpenFileDialog(a.Ctx, dialogOptions)
	return
}

func (a *App) SetDir(dir []string) (err error) {
	if len(dir) > 0 {
		app.OutputDir = dir[0]
	}
	if len(dir) > 1 {
		utils.FfmpegDir = dir[1]
	}
	if len(dir) > 2 {
		utils.WkToPdfDir = dir[2]
	}
	if len(dir) > 1 {
		if err = validateExecutablePath(utils.FfmpegDir, "ffmpeg"); err != nil {
			return err
		}
	}
	if len(dir) > 2 {
		if err = validateExecutablePath(utils.WkToPdfDir, "wkhtmltopdf"); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) CourseDownload(id, aid, dType int, enid string) (err error) {
	var d app.CourseDownload
	d.Ctx = a.Ctx
	d.ID = id
	d.AID = aid
	d.EnId = enid
	d.DownloadType = dType
	err = d.Download()
	return
}

func (a *App) OdobDownload(id, dType int, data *services.Course) (err error) {
	var d app.OdobDownload
	d.Ctx = a.Ctx
	d.ID = id
	d.DownloadType = dType
	d.Data = data
	err = d.Download()
	return
}

func (a *App) EbookDownload(id, dType int, enid string) (err error) {
	var d app.EBookDownload
	d.Ctx = a.Ctx
	d.ID = id
	d.DownloadType = dType
	d.EnID = enid
	err = d.Download()
	return
}

// BatchEbookDownload 批量下载电子书
type BatchEbookDownload struct {
	ID    int    `json:"id"`
	EnID  string `json:"enid"`
	Title string `json:"title"`
}

func (a *App) BatchEbookDownload(ebooks []BatchEbookDownload, dType int) (err error) {
	if len(ebooks) == 0 {
		return fmt.Errorf("没有选择要下载的电子书")
	}

	// 逐个下载，使用goroutine在后台下载
	go func() {
		for _, ebook := range ebooks {
			var d app.EBookDownload
			d.Ctx = a.Ctx
			d.ID = ebook.ID
			d.DownloadType = dType
			d.EnID = ebook.EnID

			err := d.Download()
			if err != nil {
				fmt.Printf("下载 %s 失败: %v\n", ebook.Title, err)
			}
		}
	}()

	return nil
}

// BatchArticleDownload 批量下载课程文章
type BatchArticleDownload struct {
	ID    int    `json:"id"`
	AID   int    `json:"aid"`
	EnID  string `json:"enid"`
	Title string `json:"title"`
}

func (a *App) BatchCourseDownload(articles []BatchArticleDownload, dType int) (err error) {
	if len(articles) == 0 {
		return fmt.Errorf("没有选择要下载的文章")
	}

	// 逐个下载，使用goroutine在后台下载
	go func() {
		for _, article := range articles {
			var d app.CourseDownload
			d.Ctx = a.Ctx
			d.ID = article.ID
			d.AID = article.AID
			d.EnId = article.EnID
			d.DownloadType = dType

			err := d.Download()
			if err != nil {
				fmt.Printf("下载 %s 失败: %v\n", article.Title, err)
			}
		}
	}()

	return nil
}

func validateExecutablePath(path string, label string) error {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s 路径无效", label)
	}
	if info.IsDir() {
		return fmt.Errorf("%s 必须是可执行文件", label)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("%s 没有执行权限", label)
	}
	return nil
}
