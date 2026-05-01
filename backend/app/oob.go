package app

import (
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yann0917/dedao-gui/backend/services"
)

type OdobItem struct {
	ID           int
	Enid         string
	Title        string
	AudioDetail  *services.Audio
}

// BatchOdobDownload 批量下载听书
func BatchOdobDownload(ctx context.Context, items []OdobItem, downloadType int) error {
	total := len(items)
	for i, item := range items {
		data := &services.Course{
			ID:    item.ID,
			Enid:  item.Enid,
			Title: item.Title,
		}
		if item.AudioDetail != nil {
			data.AudioDetail = *item.AudioDetail
		}
		download := &OdobDownload{
			Ctx:          ctx,
			DownloadType: downloadType,
			ID:           item.ID,
			Data:         data,
		}
		var progress Progress
		progress.ID = item.ID
		progress.Total = total
		progress.Current = i + 1
		progress.Pct = (i + 1) * 100 / total
		progress.Value = item.Title
		runtime.EventsEmit(ctx, "batchOdobDownload", progress)
		if err := download.Download(); err != nil {
			return err
		}
	}
	return nil
}

// AudioDetail 听书音频简介
func AudioDetail(id string) (detail *services.AudioInfoResp, err error) {
	detail, err = getService().AudioDetail(id)
	if err != nil {
		return
	}
	return
}

func AudioDetailAlias(aliasID string) (detail *services.Audio, err error) {
	detail, err = getService().AudioDetailAlias(aliasID)
	if err != nil {
		return
	}
	return
}

// OdobShelfAdd 听书加入书架
func OdobShelfAdd(enIDs []string) (resp *services.EbookShelfAddResp, err error) {
	resp, err = getService().OdobShelfAdd(enIDs)
	return
}
