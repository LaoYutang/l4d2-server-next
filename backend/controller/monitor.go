package controller

import (
	"bufio"
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

func init() {
	go startMonitor()
}

func startMonitor() {
	var lastNetSent, lastNetRecv uint64

	// Initialize last net stats to avoid huge spike on first tick
	netIO, _ := getNetIOCounters()
	for _, io := range netIO {
		name := strings.ToLower(io.Name)
		if shouldIgnoreInterface(name) {
			continue
		}
		lastNetSent += io.BytesSent
		lastNetRecv += io.BytesRecv
	}

	// Warm up CPU (first call to Percent(0) sets baseline)
	cpu.Percent(0, false)
	cpu.Percent(0, true)

	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		// CPU
		// When interval is 0, it calculates time since last call
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

		// Memory
		vMem, _ := mem.VirtualMemory()
		sMem, _ := mem.SwapMemory()

		// Disk
		// Get usage of the partition containing the current working directory
		var totalDisk, usedDisk uint64
		cwd, err := os.Getwd()
		if err == nil {
			usage, err := disk.Usage(cwd)
			if err == nil {
				totalDisk = usage.Total
				usedDisk = usage.Used
			}
		}

		// Network
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

		// Calculate Speed
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
	}
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
