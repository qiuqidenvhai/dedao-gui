package downloader

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// TaskStatus 表示下载任务的状态
type TaskStatus int

const (
	TaskPending   TaskStatus = iota // 等待中
	TaskRunning                    // 下载中
	TaskPaused                     // 已暂停
	TaskCompleted                  // 已完成
	TaskFailed                     // 失败
	TaskCancelled                  // 已取消
)

// TaskProgress 单个文件的精细进度
type TaskProgress struct {
	TaskID     string  `json:"task_id"`      // 任务唯一ID
	Title      string  `json:"title"`         // 文件标题
	Status     TaskStatus `json:"status"`       // 当前状态
	TotalBytes int64   `json:"total_bytes"`   // 文件总大小（字节）
	DoneBytes  int64   `json:"done_bytes"`    // 已下载字节数
	SpeedBps   float64 `json:"speed_bps"`     // 当前速度（字节/秒）
	Pct        int     `json:"pct"`           // 百分比 0-100
	ETA        string  `json:"eta"`           // 预计剩余时间
	Error      string  `json:"error,omitempty"` // 错误信息
}

// Task 一个下载任务
type Task struct {
	ID       string        // 唯一标识
	Title    string        // 显示名称
	Fn       func(ctx context.Context, report func(TaskProgress)) error // 下载函数
	progress atomic.Int64  // 已完成字节数
	total    atomic.Int64  // 总字节数
	status   atomic.Value  // TaskStatus
	cancel   context.CancelFunc
	mu       sync.Mutex
	speedCalc *speedCalculator
}

func newTask(id, title string, fn func(ctx context.Context, report func(TaskProgress)) error) *Task {
	t := &Task{
		ID:        id,
		Title:     title,
		Fn:        fn,
		speedCalc: newSpeedCalculator(),
	}
	t.status.Store(TaskPending)
	return t
}

func (t *Task) Status() TaskStatus { return t.status.Load().(TaskStatus) }
func (t *Task) SetStatus(s TaskStatus) { t.status.Store(s) }
func (t *Task) DoneBytes() int64 { return t.progress.Load() }
func (t *Task) TotalBytes() int64 { return t.total.Load() }
func (t *Task) addDone(n int64) {
	old := t.progress.Add(n)
	t.speedCalc.Record(old + n)
}
func (t *Task) setTotal(n int64) { t.total.Store(n) }

// DownloadManager 并行下载管理器
type DownloadManager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wailsCtx   context.Context // Wails 事件上下文
	taskChan   chan *Task      // 任务通道
	workers    int             // 工作线程数
	tasks      map[string]*Task // 所有任务
	taskMu     sync.RWMutex
	wg         sync.WaitGroup
	paused     atomic.Bool
	globalCancel context.CancelFunc

	// 统计
	totalTasks atomic.Int32
	doneTasks  atomic.Int32
	failTasks  atomic.Int32

	// 进度回调
	onProgress func(TaskProgress)
	onTaskDone func(*Task)
}

// NewDownloadManager 创建下载管理器
//   - wailsCtx: Wails runtime context，用于发送事件
//   - workers: 并发工作线程数（建议 5-10）
func NewDownloadManager(wailsCtx context.Context, workers int) *DownloadManager {
	if workers < 1 {
		workers = 5
	}
	if workers > 20 {
		workers = 20
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, gcancel := context.WithCancel(context.Background())
	m := &DownloadManager{
		ctx:          ctx,
		cancel:       cancel,
		wailsCtx:     wailsCtx,
		taskChan:     make(chan *Task, 256),
		workers:      workers,
		tasks:        make(map[string]*Task),
		globalCancel: gcancel,
	}
	// 启动工作池
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}
	return m
}

// AddTask 添加下载任务到队列
func (m *DownloadManager) AddTask(id, title string, fn func(ctx context.Context, report func(TaskProgress)) error) {
	t := newTask(id, title, fn)
	m.taskMu.Lock()
	m.tasks[id] = t
	m.taskMu.Unlock()
	m.totalTasks.Add(1)
	select {
	case m.taskChan <- t:
	default:
		t.SetStatus(TaskFailed)
		t.Fn = nil // 队列满，丢弃
	}
}

// Start 启动管理器（已由 NewDownloadManager 自动启动工作池）
func (m *DownloadManager) Start() {}

// Pause 全局暂停：停止接受新任务，当前任务跑完后暂停
func (m *DownloadManager) Pause() {
	m.paused.Store(true)
}

// Resume 恢复下载
func (m *DownloadManager) Resume() {
	m.paused.Store(false)
}

// Cancel 取消所有任务并关闭管理器
func (m *DownloadManager) Cancel() {
	m.globalCancel()
	m.cancel()
	m.taskMu.Lock()
	for _, t := range m.tasks {
		if t.Status() == TaskRunning || t.Status() == TaskPending {
			t.SetStatus(TaskCancelled)
			if t.cancel != nil {
				t.cancel()
			}
		}
	}
	m.taskMu.Unlock()
	close(m.taskChan) // 关闭通道让 worker 退出
}

// CancelTask 取消单个任务
func (m *DownloadManager) CancelTask(id string) bool {
	m.taskMu.RLock()
	t, ok := m.tasks[id]
	m.taskMu.RUnlock()
	if !ok {
		return false
	}
	if t.Status() == TaskRunning || t.Status() == TaskPending {
		t.SetStatus(TaskCancelled)
		if t.cancel != nil {
			t.cancel()
		}
		return true
	}
	return false
}

// GetTask 获取任务状态
func (m *DownloadManager) GetTask(id string) (*Task, bool) {
	m.taskMu.RLock()
	defer m.taskMu.RUnlock()
	t, ok := m.tasks[id]
	return t, ok
}

// GetAllTasks 获取所有任务快照
func (m *DownloadManager) GetAllTasks() []TaskProgress {
	m.taskMu.RLock()
	defer m.taskMu.RUnlock()
	result := make([]TaskProgress, 0, len(m.tasks))
	for _, t := range m.tasks {
		p := m.buildProgress(t)
		result = append(result, p)
	}
	return result
}

// Stats 返回总体统计
func (m *DownloadManager) Stats() (total, done, failed, running int) {
	total = int(m.totalTasks.Load())
	done = int(m.doneTasks.Load())
	failed = int(m.failTasks.Load())
	m.taskMu.RLock()
	for _, t := range m.tasks {
		if t.Status() == TaskRunning {
			running++
		}
	}
	m.taskMu.RUnlock()
	return
}

// SetProgressCallback 设置进度回调
func (m *DownloadManager) SetProgressCallback(fn func(TaskProgress)) {
	m.onProgress = fn
}

// SetTaskDoneCallback 设置任务完成回调
func (m *DownloadManager) SetTaskDoneCallback(fn func(*Task)) {
	m.onTaskDone = fn
}

// Wait 等待所有任务完成
func (m *DownloadManager) Wait() {
	m.wg.Wait()
}

// ---- 内部方法 ----

func (m *DownloadManager) worker(id int) {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case task, ok := <-m.taskChan:
			if !ok {
				return
			}
			m.executeTask(task)
		}
	}
}

func (m *DownloadManager) executeTask(t *Task) {
	// 检查全局暂停
	for m.paused.Load() {
		select {
		case <-m.ctx.Done():
			t.SetStatus(TaskCancelled)
			return
		case <-time.After(500 * time.Millisecond):
			continue
		}
	}

	// 检查是否已取消
	if t.Status() == TaskCancelled {
		return
	}

	t.SetStatus(TaskRunning)
	tctx, tcancel := context.WithCancel(m.ctx)
	t.cancel = tcancel

	report := func(p TaskProgress) {
		if m.onProgress != nil {
			m.onProgress(p)
		}
		// 通过 Wails 事件推送到前端
		if m.wailsCtx != nil {
			runtime.EventsEmit(m.wailsCtx, "download:progress", p)
		}
	}

	err := t.Fn(tctx, report)

	if err != nil {
		if t.Status() != TaskCancelled {
			t.SetStatus(TaskFailed)
			m.failTasks.Add(1)
			// 报告失败进度
			p := m.buildProgress(t)
			p.Error = err.Error()
			report(p)
		}
	} else if t.Status() != TaskCancelled {
		t.SetStatus(TaskCompleted)
		m.doneTasks.Add(1)
		p := m.buildProgress(t)
		p.Pct = 100
		report(p)
	}

	if m.onTaskDone != nil {
		m.onTaskDone(t)
	}
}

func (m *DownloadManager) buildProgress(t *Task) TaskProgress {
	status := t.Status()
	done := t.DoneBytes()
	total := t.TotalBytes()
	pct := 0
	if total > 0 {
		pct = int(float64(done) / float64(total) * 100)
		if pct > 100 {
			pct = 100
		}
	}
	var eta string
	speed := t.speedCalc.Speed()
	if speed > 0 && total > done {
		remainingSec := float64(total-done) / speed
		eta = formatDuration(time.Duration(remainingSec) * time.Second)
	}
	return TaskProgress{
		TaskID:     t.ID,
		Title:      t.Title,
		Status:     status,
		TotalBytes: total,
		DoneBytes:  done,
		SpeedBps:   speed,
		Pct:        pct,
		ETA:        eta,
	}
}

// ---- 速度计算器 ----

type speedCalculator struct {
	mu      sync.Mutex
	samples []sample
}

type sample struct {
	bytes int64
	time  time.Time
}

func newSpeedCalculator() *speedCalculator {
	return &speedCalculator{
		samples: make([]sample, 0, 10),
	}
}

func (sc *speedCalculator) Record(bytes int64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	now := time.Now()
	sc.samples = append(sc.samples, sample{bytes: bytes, time: now})
	// 只保留最近 10 秒的样本
	cutoff := now.Add(-10 * time.Second)
	for len(sc.samples) > 1 && sc.samples[0].time.Before(cutoff) {
		sc.samples = sc.samples[1:]
	}
}

func (sc *speedCalculator) Speed() float64 {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if len(sc.samples) < 2 {
		return 0
	}
	first := sc.samples[0]
	last := sc.samples[len(sc.samples)-1]
	delta := last.time.Sub(first.time).Seconds()
	if delta <= 0 {
		return 0
	}
	return float64(last.bytes-first.bytes) / delta
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", d/time.Second%60)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", d/time.Minute%60, d/time.Second%60)
	}
	return fmt.Sprintf("%dh%dm", d/time.Hour, d/time.Minute%60)
}
