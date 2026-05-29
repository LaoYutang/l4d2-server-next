package controller

import (
	"bufio"
	"l4d2-manager-next/consts"
	"l4d2-manager-next/db"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/model"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type SystemStatus struct {
	CPUPercent   float64 `json:"cpu_percent"`
	CPUMaxCore   float64 `json:"cpu_max_core_percent"`
	MemTotal     uint64  `json:"mem_total"`
	MemUsed      uint64  `json:"mem_used"`
	SwapTotal    uint64  `json:"swap_total"`
	SwapUsed     uint64  `json:"swap_used"`
	DiskTotal    uint64  `json:"disk_total"`
	DiskUsed     uint64  `json:"disk_used"`
	NetUpSpeed   uint64  `json:"net_up_speed"`   // bytes/sec
	NetDownSpeed uint64  `json:"net_down_speed"` // bytes/sec
	Timestamp    int64   `json:"timestamp"`
}

var (
	currentStatus SystemStatus
	statusMutex   sync.RWMutex
)

func StartMonitor() {
	var lastNetSent, lastNetRecv uint64

	// 初始化上次网络统计，避免首次运行时出现巨大峰值
	netIO, _ := getNetIOCounters()
	for _, io := range netIO {
		name := strings.ToLower(io.Name)
		if shouldIgnoreInterface(name) {
			continue
		}
		lastNetSent += io.BytesSent
		lastNetRecv += io.BytesRecv
	}

	// 预热 CPU (首次调用 Percent(0) 设置基准)
	cpu.Percent(0, false)
	cpu.Percent(0, true)

	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		// CPU
		// 当间隔为 0 时，计算自上次调用以来的时间
		cpuPercents, _ := cpu.Percent(0, false)
		totalCPU := 0.0
		if len(cpuPercents) > 0 {
			totalCPU = cpuPercents[0]
		}

		cpuCorePercents, _ := cpu.Percent(0, true)
		maxCore := 0.0
		for _, p := range cpuCorePercents {
			if p > maxCore {
				maxCore = p
			}
		}

		// 内存
		vMem, _ := mem.VirtualMemory()
		sMem, _ := mem.SwapMemory()

		// 硬盘
		// 获取 addons 目录所在分区的磁盘使用情况
		var totalDisk, usedDisk uint64
		usage, err := disk.Usage(consts.AddonsBasePath)
		if err == nil {
			totalDisk = usage.Total
			usedDisk = usage.Used
		}

		// 网络
		netIO, _ := getNetIOCounters()
		var currNetSent, currNetRecv uint64
		for _, io := range netIO {
			name := strings.ToLower(io.Name)
			if shouldIgnoreInterface(name) {
				continue
			}
			currNetSent += io.BytesSent
			currNetRecv += io.BytesRecv
		}

		// 计算速度
		upSpeed := uint64(0)
		downSpeed := uint64(0)

		if currNetSent >= lastNetSent {
			upSpeed = currNetSent - lastNetSent
		}
		if currNetRecv >= lastNetRecv {
			downSpeed = currNetRecv - lastNetRecv
		}

		lastNetSent = currNetSent
		lastNetRecv = currNetRecv

		statusMutex.Lock()
		currentStatus = SystemStatus{
			CPUPercent:   totalCPU,
			CPUMaxCore:   maxCore,
			MemTotal:     vMem.Total,
			MemUsed:      vMem.Used,
			SwapTotal:    sMem.Total,
			SwapUsed:     sMem.Used,
			DiskTotal:    totalDisk,
			DiskUsed:     usedDisk,
			NetUpSpeed:   upSpeed,
			NetDownSpeed: downSpeed,
			Timestamp:    time.Now().Unix(),
		}
		statusMutex.Unlock()

		// 如果启用，写入数据库
		if db.DB != nil && logic.IsMonitorHistoryEnabled() {
			metric := model.SystemMetric{
				Timestamp:    currentStatus.Timestamp,
				CPUPercent:   toFixed(currentStatus.CPUPercent, 2),
				CPUMaxCore:   toFixed(currentStatus.CPUMaxCore, 2),
				MemUsed:      toFixed(float64(currentStatus.MemUsed)/1024/1024/1024, 2),  // GB
				SwapUsed:     toFixed(float64(currentStatus.SwapUsed)/1024/1024/1024, 2), // GB
				NetUpSpeed:   toFixed(float64(currentStatus.NetUpSpeed)/1024, 2),         // KB
				NetDownSpeed: toFixed(float64(currentStatus.NetDownSpeed)/1024, 2),       // KB
				DiskUsed:     toFixed(float64(currentStatus.DiskUsed)/1024/1024/1024, 2), // GB
			}
			// 在后台协程中创建记录，避免阻塞主监控循环
			db.DB.Create(&metric)
		}
	}
}

func toFixed(val float64, precision int) float64 {
	p := 1.0
	for i := 0; i < precision; i++ {
		p *= 10
	}
	return float64(int(val*p+0.5)) / p
}

func shouldIgnoreInterface(name string) bool {
	return strings.Contains(name, "docker") ||
		strings.Contains(name, "veth") ||
		strings.Contains(name, "br-") ||
		strings.Contains(name, "loopback") ||
		name == "lo"
}

func getNetIOCounters() ([]net.IOCountersStat, error) {
	// Check if /host/proc/1/net/dev exists
	hostNetDev := "/host/proc/1/net/dev"
	if _, err := os.Stat(hostNetDev); err == nil {
		return parseNetDev(hostNetDev)
	}
	return net.IOCounters(true)
}

func parseNetDev(path string) ([]net.IOCountersStat, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var ret []net.IOCountersStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			name := strings.TrimSpace(parts[0])
			fields := strings.Fields(parts[1])
			if len(fields) < 16 {
				continue
			}

			// recv: bytes packets errs drop fifo frame compressed multicast
			// sent: bytes packets errs drop fifo colls carrier compressed

			recvBytes, _ := strconv.ParseUint(fields[0], 10, 64)
			sentBytes, _ := strconv.ParseUint(fields[8], 10, 64)

			ret = append(ret, net.IOCountersStat{
				Name:      name,
				BytesRecv: recvBytes,
				BytesSent: sentBytes,
			})
		}
	}
	return ret, nil
}

func GetMonitorStatus(c *gin.Context) {
	statusMutex.RLock()
	defer statusMutex.RUnlock()
	c.JSON(http.StatusOK, currentStatus)
}

func GetMonitorConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"history_enabled": logic.IsMonitorHistoryEnabled() && db.DB != nil,
	})
}

func SetMonitorConfig(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.String(http.StatusForbidden, "需要管理员权限")
		return
	}

	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.String(http.StatusBadRequest, "参数错误")
		return
	}

	if req.Enable && db.DB == nil {
		c.String(http.StatusBadRequest, "性能监控历史数据库未初始化")
		return
	}

	LogOp(c, req, "设置性能监控历史记录配置")
	if err := logic.SetMonitorHistoryEnable(req.Enable); err != nil {
		c.String(http.StatusInternalServerError, "保存配置失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func GetMonitorHistory(c *gin.Context) {
	// 鉴权：只有管理员才能查看历史数据
	role, _ := c.Get("role")
	if role != "admin" {
		c.String(http.StatusForbidden, "需要管理员权限才能查看历史数据")
		return
	}

	if db.DB == nil || !logic.IsMonitorHistoryEnabled() {
		c.String(http.StatusBadRequest, "历史数据记录未启用")
		return
	}

	startStr := c.PostForm("start")
	endStr := c.PostForm("end")
	start, _ := strconv.ParseInt(startStr, 10, 64)
	end, _ := strconv.ParseInt(endStr, 10, 64)

	if start == 0 || end == 0 {
		c.String(http.StatusBadRequest, "无效的开始或结束时间")
		return
	}

	// Count records first
	var count int64
	err := db.DB.Model(&model.SystemMetric{}).Where("timestamp >= ? AND timestamp <= ?", start, end).Count(&count).Error
	if err != nil {
		c.String(http.StatusInternalServerError, "查询数据库失败: %v", err)
		return
	}

	type Result struct {
		BucketTime   int64   `json:"timestamp"`
		CPUPercent   float64 `json:"cpu_percent"`
		CPUMaxCore   float64 `json:"cpu_max_core_percent"`
		MemUsed      float64 `json:"mem_used"`
		SwapUsed     float64 `json:"swap_used"`
		NetUpSpeed   float64 `json:"net_up_speed"`
		NetDownSpeed float64 `json:"net_down_speed"`
		DiskUsed     float64 `json:"disk_used"`
	}

	var results []Result

	// 如果数据量较少 (<= 2000)，直接返回原始数据
	// ECharts 可以轻松处理 2000 个点。这样可以避免在数据稀疏时进行不当的降采样
	if count <= 2000 {
		var rawData []model.SystemMetric
		err = db.DB.Where("timestamp >= ? AND timestamp <= ?", start, end).Order("timestamp ASC").Find(&rawData).Error
		if err != nil {
			c.String(http.StatusInternalServerError, "查询原始数据失败: %v", err)
			return
		}
		// 转换为 Result 格式
		for _, m := range rawData {
			results = append(results, Result{
				BucketTime:   m.Timestamp,
				CPUPercent:   m.CPUPercent,
				CPUMaxCore:   m.CPUMaxCore,
				MemUsed:      m.MemUsed,
				SwapUsed:     m.SwapUsed,
				NetUpSpeed:   m.NetUpSpeed,
				NetDownSpeed: m.NetDownSpeed,
				DiskUsed:     m.DiskUsed,
			})
		}
		c.JSON(http.StatusOK, results)
		return
	}

	// 降采样逻辑: 目标约 720 个点
	// 使用实际数据的时间范围而不是查询范围，以正确处理稀疏数据
	var minTime, maxTime int64
	type MinMax struct {
		MinT int64
		MaxT int64
	}
	var mm MinMax
	err = db.DB.Model(&model.SystemMetric{}).
		Select("MIN(timestamp) as min_t, MAX(timestamp) as max_t").
		Where("timestamp >= ? AND timestamp <= ?", start, end).
		Scan(&mm).Error

	if err != nil {
		// 如果失败则回退到查询范围
		minTime = start
		maxTime = end
	} else {
		minTime = mm.MinT
		maxTime = mm.MaxT
	}

	duration := maxTime - minTime
	if duration <= 0 {
		duration = end - start // Fallback
	}

	targetPoints := int64(720)
	bucketSize := duration / targetPoints
	if bucketSize < 1 {
		bucketSize = 1
	}

	// SQLite 聚合查询
	err = db.DB.Model(&model.SystemMetric{}).
		Select("CAST(timestamp / ? AS INTEGER) * ? as bucket_time, MAX(cpu_percent) as cpu_percent, MAX(cpu_max_core) as cpu_max_core, MAX(mem_used) as mem_used, MAX(swap_used) as swap_used, MAX(net_up_speed) as net_up_speed, MAX(net_down_speed) as net_down_speed, MAX(disk_used) as disk_used", bucketSize, bucketSize).
		Where("timestamp >= ? AND timestamp <= ?", start, end).
		Group("bucket_time").
		Order("bucket_time ASC").
		Scan(&results).Error

	if err != nil {
		c.String(http.StatusInternalServerError, "聚合查询失败: %v", err)
		return
	}

	c.JSON(http.StatusOK, results)
}
