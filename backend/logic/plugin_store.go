package logic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type GitHubTreeResponse struct {
	Tree []GitHubTreeItem `json:"tree"`
}

type GitHubTreeItem struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

type StorePlugin struct {
	Name      string `json:"name"`
	FileCount int    `json:"file_count"`
	Size      int    `json:"size"`
	Installed bool   `json:"installed"`
}

var (
	storeCache     []StorePlugin
	storeCacheTime time.Time
	storeCacheMut  sync.Mutex
)

// FetchStorePlugins fetches the plugin list from GitHub repository
func FetchStorePlugins(forceRefresh bool, proxyUrl string) ([]StorePlugin, error) {
	storeCacheMut.Lock()
	defer storeCacheMut.Unlock()

	// Cache for 10 minutes, unless forceRefresh is true
	// Installed 字段每次都实时计算，不依赖缓存
	if !forceRefresh && time.Since(storeCacheTime) < 10*time.Minute && storeCache != nil {
		return markInstalledPlugins(storeCache), nil
	}

	client := &http.Client{Timeout: 10 * time.Second}

	rawUrl := "https://api.github.com/repos/LaoYutang/l4d2-plugins-store/git/trees/master?recursive=1"
	fetchUrl := rawUrl
	if proxyUrl != "" {
		proxyUrl = strings.TrimSuffix(proxyUrl, "/")
		fetchUrl = proxyUrl + "/" + rawUrl
	}

	req, err := http.NewRequest("GET", fetchUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API 返回状态码 %d", resp.StatusCode)
	}

	var treeResp GitHubTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&treeResp); err != nil {
		return nil, fmt.Errorf("解析 GitHub 数据失败: %v", err)
	}

	pluginMap := make(map[string]*StorePlugin)

	for _, item := range treeResp.Tree {
		if !strings.HasPrefix(item.Path, "plugins/") {
			continue
		}
		parts := strings.Split(item.Path, "/")
		if len(parts) < 2 {
			continue
		}
		pluginName := parts[1]
		if pluginName == "" {
			continue
		}

		if _, exists := pluginMap[pluginName]; !exists {
			pluginMap[pluginName] = &StorePlugin{Name: pluginName}
		}

		if item.Type == "blob" {
			pluginMap[pluginName].FileCount++
			pluginMap[pluginName].Size += item.Size
		}
	}

	var plugins []StorePlugin
	for _, p := range pluginMap {
		if p.FileCount > 0 { // Only include plugins with files
			plugins = append(plugins, *p)
		}
	}

	storeCache = plugins
	storeCacheTime = time.Now()

	return markInstalledPlugins(plugins), nil
}

// markInstalledPlugins 根据本地目录判断哪些商店插件已安装，每次调用都实时重新计算
func markInstalledPlugins(plugins []StorePlugin) []StorePlugin {
	storePath := getStorePath()
	installedSet := make(map[string]bool)
	if entries, err := os.ReadDir(storePath); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				installedSet[e.Name()] = true
			}
		}
	}

	result := make([]StorePlugin, len(plugins))
	copy(result, plugins)
	for i := range result {
		result[i].Installed = installedSet[result[i].Name]
	}
	return result
}

// DownloadStorePlugin downloads a plugin from the store
func DownloadStorePlugin(pluginName, proxyUrl string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/LaoYutang/l4d2-plugins-store/git/trees/master?recursive=1")
	if err != nil {
		return fmt.Errorf("请求 GitHub API 失败: %v", err)
	}
	defer resp.Body.Close()

	var treeResp GitHubTreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&treeResp); err != nil {
		return fmt.Errorf("解析 GitHub 数据失败: %v", err)
	}

	var filesToDownload []string
	prefix := "plugins/" + pluginName + "/"
	for _, item := range treeResp.Tree {
		if item.Type == "blob" && strings.HasPrefix(item.Path, prefix) {
			filesToDownload = append(filesToDownload, item.Path)
		}
	}

	if len(filesToDownload) == 0 {
		return fmt.Errorf("未找到插件或插件为空")
	}

	storePath := getStorePath()
	destDir := filepath.Join(storePath, pluginName)
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		return fmt.Errorf("插件 %s 已存在，请先删除", pluginName)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("创建插件目录失败: %v", err)
	}

	success := false
	defer func() {
		if !success {
			os.RemoveAll(destDir)
		}
	}()

	var wg sync.WaitGroup
	errChan := make(chan error, len(filesToDownload))

	for _, file := range filesToDownload {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()

			relPath := strings.TrimPrefix(path, prefix)
			localPath := filepath.Join(destDir, relPath)

			if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
				errChan <- fmt.Errorf("创建目录失败: %v", err)
				return
			}

			parts := strings.Split(path, "/")
			for i, p := range parts {
				parts[i] = url.PathEscape(p)
			}
			encodedPath := strings.Join(parts, "/")

			rawUrl := "https://raw.githubusercontent.com/LaoYutang/l4d2-plugins-store/master/" + encodedPath
			downloadUrl := rawUrl
			if proxyUrl != "" {
				proxyUrl = strings.TrimSuffix(proxyUrl, "/")
				downloadUrl = proxyUrl + "/" + rawUrl
			}

			if err := downloadFileWithRetry(downloadUrl, localPath, 3); err != nil {
				errChan <- fmt.Errorf("下载文件 %s 失败: %v", relPath, err)
				return
			}
		}(file)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	success = true
	writePluginSource(pluginName, "store")
	return nil
}

func downloadFileWithRetry(url, filepath string, retries int) error {
	var err error
	for i := 0; i < retries; i++ {
		err = downloadFile(url, filepath)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

func downloadFile(urlStr, filepath string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	// Add user agent to prevent some proxies from blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP 状态码 %d", resp.StatusCode)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
