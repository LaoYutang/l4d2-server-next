package controller

import (
	"fmt"
	"io"
	"l4d2-manager-next/consts"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/disk"
)

type DOWNLOAD_STATUS = uint8

const (
	DOWNLOAD_STATUS_PENDING     DOWNLOAD_STATUS = iota // 等待下载
	DOWNLOAD_STATUS_IN_PROGRESS                        // 下载中
	DOWNLOAD_STATUS_COMPLETED                          // 下载完成
	DOWNLOAD_STATUS_FAILED                             // 下载失败
)

type downloadTask struct {
	url              string          // 下载链接
	status           DOWNLOAD_STATUS // 状态
	message          string          // 错误消息
	progress         float64         // 进度
	cancel           chan struct{}   // 取消信号通道
	cancelled        bool            // 标记是否已取消
	downloadSpeed    float64         // 下载速度 (bytes/second)
	startTime        time.Time       // 下载开始时间
	lastUpdate       time.Time       // 上次更新时间
	downloadedBytes  int64           // 已下载字节数
	lastSecondBytes  int64           // 上一秒的下载字节数
	speedUpdateTimer *time.Ticker    // 速度更新定时器
	mu               sync.RWMutex    // 读写锁，保护并发访问
	semaphore        chan struct{}   // 并发控制信号量
	totalSize        int64           // 文件总大小
	filename         string          // 文件名
}

// 新增下载任务
func NewDownloadTask(url string, semaphore chan struct{}) *downloadTask {
	res := &downloadTask{
		url:              url,
		status:           DOWNLOAD_STATUS_PENDING,
		cancel:           make(chan struct{}),
		cancelled:        false,
		downloadSpeed:    0,
		startTime:        time.Now(),
		lastUpdate:       time.Now(),
		downloadedBytes:  0,
		lastSecondBytes:  0,
		speedUpdateTimer: time.NewTicker(5 * time.Second),
		semaphore:        semaphore,
		totalSize:        0, // 初始化文件总大小
		filename:         "",
	}

	// 启动协程下载文件
	go res.download()

	return res
}

// 添加取消方法
func (dt *downloadTask) Cancel() {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	if !dt.cancelled {
		dt.cancelled = true
		close(dt.cancel)
	}

	if dt.speedUpdateTimer != nil {
		dt.speedUpdateTimer.Stop()
	}
}

// 定期更新下载速度
func (dt *downloadTask) updateSpeedPeriodically() {
	defer func() {
		if dt.speedUpdateTimer != nil {
			dt.speedUpdateTimer.Stop()
		}
	}()

	for {
		select {
		case <-dt.cancel:
			return
		case <-dt.speedUpdateTimer.C:
			dt.mu.Lock()
			currentBytes := dt.downloadedBytes
			bytesInPeriod := currentBytes - dt.lastSecondBytes
			// 计算每秒平均速度 (5秒内下载的字节数 / 5)
			dt.downloadSpeed = float64(bytesInPeriod) / 5.0
			dt.lastSecondBytes = currentBytes
			dt.mu.Unlock()
		}
	}
}

// 执行实际的文件下载
func (dt *downloadTask) download() {
	select {
	case <-dt.cancel:
		dt.message = "下载已取消"
		dt.status = DOWNLOAD_STATUS_FAILED
		return
	case dt.semaphore <- struct{}{}:
	}

	defer func() { <-dt.semaphore }() // 释放信号量

	dt.status = DOWNLOAD_STATUS_IN_PROGRESS
	dt.startTime = time.Now()
	dt.lastUpdate = time.Now()

	// 启动速度计算协程
	go dt.updateSpeedPeriodically()

	// 发送HTTP请求
	resp, err := http.Get(dt.url)
	if err != nil {
		dt.message = fmt.Sprintf("下载失败: %v", err)
		dt.status = DOWNLOAD_STATUS_FAILED
		return
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		dt.message = fmt.Sprintf("HTTP错误: %d", resp.StatusCode)
		dt.status = DOWNLOAD_STATUS_FAILED
		return
	}

	// 从URL中提取文件名
	fileName := dt.determineFileName(resp)
	if fileName == "" {
		fileName = "downloaded_file"
	}
	dt.mu.Lock()
	dt.filename = fileName
	dt.mu.Unlock()

	// 创建本地文件
	filePath := filepath.Join(consts.AddonsBasePath, "temp", fileName)
	err = os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		dt.message = fmt.Sprintf("创建目录失败: %v", err)
		dt.status = DOWNLOAD_STATUS_FAILED
		return
	}

	file, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("创建文件失败: %v\n", err)
		dt.status = DOWNLOAD_STATUS_FAILED
		return
	}
	defer func() {
		file.Close()
		os.Remove(filePath) // 清理临时文件
	}()

	// 获取文件总大小
	totalSize := resp.ContentLength
	var downloaded int64 = 0

	// 保存文件总大小到任务结构体中
	dt.mu.Lock()
	dt.totalSize = totalSize
	dt.mu.Unlock()

	// 创建一个带缓冲的读取器
	buffer := make([]byte, 8192)

	for {
		// 检查是否收到取消信号
		select {
		case <-dt.cancel:
			dt.message = "下载已取消"
			dt.status = DOWNLOAD_STATUS_FAILED
			return
		default:
		}

		n, err := resp.Body.Read(buffer)
		if n > 0 {
			// 写入文件
			_, writeErr := file.Write(buffer[:n])
			if writeErr != nil {
				fmt.Printf("写入文件失败: %v\n", writeErr)
				dt.status = DOWNLOAD_STATUS_FAILED
				return
			}

			downloaded += int64(n)

			// 线程安全地更新下载字节数
			dt.mu.Lock()
			dt.downloadedBytes = downloaded
			dt.mu.Unlock()

			// 更新进度
			if totalSize > 0 {
				dt.progress = float64(downloaded) / float64(totalSize) * 100.0
			}

			dt.lastUpdate = time.Now()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("读取数据失败: %v\n", err)
			dt.status = DOWNLOAD_STATUS_FAILED
			return
		}
	}

	dt.progress = 100.0
	dt.status = DOWNLOAD_STATUS_COMPLETED

	// 停止速度更新定时器
	if dt.speedUpdateTimer != nil {
		dt.speedUpdateTimer.Stop()
	}

	// 显式关闭文件以进行检查
	file.Close()

	// 检查是否为VPK文件（魔数检查）
	if f, err := os.Open(filePath); err == nil {
		magic := make([]byte, 4)
		if _, err := io.ReadFull(f, magic); err == nil {
			// VPK Signature: 0x55aa1234 (Little Endian: 34 12 aa 55)
			if magic[0] == 0x34 && magic[1] == 0x12 && magic[2] == 0xaa && magic[3] == 0x55 {
				f.Close() // 关闭以便重命名
				// 如果不是以.vpk结束，则添加后缀
				if filepath.Ext(filePath) != ".vpk" {
					newPath := filePath + ".vpk"
					if err := os.Rename(filePath, newPath); err == nil {
						filePath = newPath
						dt.mu.Lock()
						dt.filename = filepath.Base(newPath)
						dt.mu.Unlock()
					}
				}
			} else {
				f.Close()
			}
		} else {
			f.Close()
		}
	}

	// 下载完成后处理文件
	if err := ProcessFile(filePath); err != nil {
		dt.message = fmt.Sprintf("文件处理失败: %v", err)
		dt.status = DOWNLOAD_STATUS_FAILED
		return
	}
}

// 确定文件名
func (dt *downloadTask) determineFileName(resp *http.Response) string {
	// 1. 尝试从Content-Disposition获取
	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition != "" {
		_, params, err := mime.ParseMediaType(contentDisposition)
		if err == nil {
			if filename, ok := params["filename"]; ok && filename != "" {
				return filename
			}
		}
	}

	// 2. 从URL中提取文件名
	// 移除查询参数
	url := strings.Split(dt.url, "?")[0]
	// 提取最后一部分作为文件名
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if name != "" {
			return name
		}
	}

	return ""
}

// 获取下载状态
func (dt *downloadTask) GetStatus() DOWNLOAD_STATUS {
	return dt.status
}

// 获取下载进度 (0-100)
func (dt *downloadTask) GetProgress() float64 {
	return dt.progress
}

// 获取错误消息
func (dt *downloadTask) GetMessage() string {
	return dt.message
}

// 获取下载速度 (bytes/second)
func (dt *downloadTask) GetDownloadSpeed() float64 {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return dt.downloadSpeed
}

// 获取文件总大小 (bytes)
func (dt *downloadTask) GetTotalSize() int64 {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return dt.totalSize
}

// 获取文件名
func (dt *downloadTask) GetFilename() string {
	dt.mu.RLock()
	defer dt.mu.RUnlock()
	return dt.filename
}

// 格式化下载速度为可读字符串
func (dt *downloadTask) GetFormattedSpeed() string {
	speed := dt.GetDownloadSpeed()
	if speed < 1024 {
		return fmt.Sprintf("%.2f B/s", speed)
	} else if speed < 1024*1024 {
		return fmt.Sprintf("%.2f KB/s", speed/1024)
	} else if speed < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB/s", speed/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB/s", speed/(1024*1024*1024))
	}
}

// 格式化文件大小为可读字符串
func (dt *downloadTask) GetFormattedSize() string {
	size := dt.GetTotalSize()
	if size <= 0 {
		return "未知大小"
	}
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(size)/(1024*1024*1024))
	}
}

func splitURLString(urlString string) []string {
	urls := make([]string, 0)

	// 使用正则表达式匹配http和https开头的URL
	re := regexp.MustCompile(`(https?://[^\s\n\r]+)`)
	matches := re.FindAllString(urlString, -1)

	for _, match := range matches {
		// 清理URL，移除可能的尾随空白字符
		cleanURL := strings.TrimSpace(match)
		if cleanURL != "" {
			urls = append(urls, cleanURL)
		}
	}

	return urls
}

type downloader struct {
	tasks     []*downloadTask
	semaphore chan struct{} // 控制最大并发下载数的信号量
}

func NewDownloader() *downloader {
	return &downloader{
		tasks:     make([]*downloadTask, 0),
		semaphore: make(chan struct{}, 3), // 允许最多3个并发下载
	}
}

func (d *downloader) AddTask(url string) {
	task := NewDownloadTask(url, d.semaphore)
	d.tasks = append(d.tasks, task)
}

func (d *downloader) GetTasksInfo() []map[string]any {
	tasksInfo := make([]map[string]any, 0)
	for _, task := range d.tasks {
		tasksInfo = append(tasksInfo, map[string]any{
			"url":            task.url,
			"status":         task.GetStatus(),
			"progress":       task.GetProgress(),
			"message":        task.GetMessage(),
			"downloadSpeed":  task.GetDownloadSpeed(),
			"formattedSpeed": task.GetFormattedSpeed(),
			"totalSize":      task.GetTotalSize(),
			"formattedSize":  task.GetFormattedSize(),
			"filename":       task.GetFilename(),
		})
	}
	return tasksInfo
}

var Downloader *downloader

func init() {
	Downloader = NewDownloader()
}

func AddDownloadTask(c *gin.Context) {
	if stat, err := disk.Usage(consts.AddonsBasePath); err != nil {
		c.String(http.StatusInternalServerError, "获取磁盘使用信息失败: %v", err)
	} else if stat.UsedPercent > 90 {
		c.String(http.StatusInsufficientStorage, "磁盘空间不足，当前使用率超过90%")
		return
	}

	url := c.PostForm("url")
	if url == "" {
		c.String(http.StatusBadRequest, "下载链接不能为空")
		return
	}

	// 识别切分多个http连接
	urls := splitURLString(url)
	for _, singleURL := range urls {
		Downloader.AddTask(singleURL)
	}
	c.String(http.StatusOK, "下载任务已添加")
}

func CancelDownloadTask(c *gin.Context) {
	indexStr := c.PostForm("index")
	if indexStr == "" {
		c.String(http.StatusBadRequest, "任务索引不能为空")
		return
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.String(http.StatusBadRequest, "任务索引格式错误")
		return
	}

	if index < 0 || index >= len(Downloader.tasks) {
		c.String(http.StatusBadRequest, "任务索引超出范围")
		return
	}

	// 取消指定索引的下载任务
	Downloader.tasks[index].Cancel()
	c.String(http.StatusOK, "下载任务已取消")
}

func ClearTasks(c *gin.Context) {
	tasks := make([]*downloadTask, 0)
	for _, task := range Downloader.tasks {
		if task.GetStatus() == DOWNLOAD_STATUS_IN_PROGRESS || task.GetStatus() == DOWNLOAD_STATUS_PENDING {
			tasks = append(tasks, task)
		}
	}
	Downloader.tasks = tasks
	c.String(http.StatusOK, "下载任务已清空")
}

func GetDownloadTasksInfo(c *gin.Context) {
	c.JSON(http.StatusOK, Downloader.GetTasksInfo())
}

func RestartDownloadTask(c *gin.Context) {
	indexStr := c.PostForm("index")
	if indexStr == "" {
		c.String(http.StatusBadRequest, "任务索引不能为空")
		return
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.String(http.StatusBadRequest, "任务索引格式错误")
		return
	}

	if index < 0 || index >= len(Downloader.tasks) {
		c.String(http.StatusBadRequest, "任务索引超出范围")
		return
	}

	// 获取原任务的URL
	originalTask := Downloader.tasks[index]
	taskURL := originalTask.url

	// 取消原任务
	originalTask.Cancel()

	// 创建新的下载任务
	newTask := NewDownloadTask(taskURL, Downloader.semaphore)

	// 替换原任务
	Downloader.tasks[index] = newTask

	c.String(http.StatusOK, "下载任务已重新开始")
}
