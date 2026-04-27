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

	"github.com/google/uuid"
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
	treeCache     = make(map[string]*GitHubTreeResponse)
	treeCacheTime = make(map[string]time.Time)
	treeCacheMut  sync.Mutex
)

func getTreeData(forceRefresh bool, proxyUrl, githubToken, repo string) (*GitHubTreeResponse, error) {
	if repo == "" {
		repo = "LaoYutang/l4d2-plugins-store"
	}

	treeCacheMut.Lock()
	defer treeCacheMut.Unlock()

	if !forceRefresh && time.Since(treeCacheTime[repo]) < 10*time.Minute && treeCache[repo] != nil {
		return treeCache[repo], nil
	}

	client := &http.Client{Timeout: 10 * time.Second}

	rawUrl := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/master?recursive=1", repo)
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
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}

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

	treeCache[repo] = &treeResp
	treeCacheTime[repo] = time.Now()
	return treeCache[repo], nil
}

func FetchStorePlugins(forceRefresh bool, proxyUrl, githubToken, repo string) ([]StorePlugin, error) {
	tree, err := getTreeData(forceRefresh, proxyUrl, githubToken, repo)
	if err != nil {
		return nil, err
	}

	pluginMap := make(map[string]*StorePlugin)

	for _, item := range tree.Tree {
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
		if p.FileCount > 0 {
			plugins = append(plugins, *p)
		}
	}

	return markInstalledPlugins(plugins), nil
}

func markInstalledPlugins(plugins []StorePlugin) []StorePlugin {
	storePath := getStorePath()
	installedSet := make(map[string]bool)
	if entries, err := os.ReadDir(storePath); err == nil {
		for _, e := range entries {
			if e.IsDir() && e.Name() != DownloadTempDir {
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

func getDownloadTempPath(id string) string {
	return filepath.Join(getStorePath(), DownloadTempDir, id)
}

// CleanDownloadTemp 启动时整体清空 .download_temp/，删除上次运行残留。
// 调用点位于 main.go 的 router.Run 之前，此时 HTTP 服务尚未对外提供，
// 不可能存在正在进行的下载，整体 RemoveAll 安全。
func CleanDownloadTemp() {
	os.RemoveAll(filepath.Join(getStorePath(), DownloadTempDir))
}

func DownloadStorePlugin(pluginName, proxyUrl, githubToken, repo string) error {
	if repo == "" {
		repo = "LaoYutang/l4d2-plugins-store"
	}

	tree, err := getTreeData(false, proxyUrl, githubToken, repo)
	if err != nil {
		return err
	}

	var filesToDownload []string
	prefix := "plugins/" + pluginName + "/"
	for _, item := range tree.Tree {
		if item.Type == "blob" && strings.HasPrefix(item.Path, prefix) {
			filesToDownload = append(filesToDownload, item.Path)
		}
	}

	if len(filesToDownload) == 0 {
		return fmt.Errorf("未找到插件或插件为空")
	}

	storePath := getStorePath()
	finalDir := filepath.Join(storePath, pluginName)
	if _, err := os.Stat(finalDir); !os.IsNotExist(err) {
		return fmt.Errorf("插件 %s 已存在，请先删除", pluginName)
	}

	tempDir := getDownloadTempPath(uuid.New().String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	var wg sync.WaitGroup
	errChan := make(chan error, len(filesToDownload))

	for _, file := range filesToDownload {
		wg.Add(1)
		go func(path string, token string) {
			defer wg.Done()

			relPath := strings.TrimPrefix(path, prefix)
			localPath := filepath.Join(tempDir, relPath)

			if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
				errChan <- fmt.Errorf("创建目录失败: %v", err)
				return
			}

			parts := strings.Split(path, "/")
			for i, p := range parts {
				parts[i] = url.PathEscape(p)
			}
			encodedPath := strings.Join(parts, "/")

			rawUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/master/%s", repo, encodedPath)
			downloadUrl := rawUrl
			if proxyUrl != "" {
				proxyUrl = strings.TrimSuffix(proxyUrl, "/")
				downloadUrl = proxyUrl + "/" + rawUrl
			}

			if err := downloadFileWithRetry(downloadUrl, localPath, 3, token); err != nil {
				errChan <- fmt.Errorf("下载文件 %s 失败: %v", relPath, err)
				return
			}
		}(file, githubToken)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(finalDir); !os.IsNotExist(err) {
		return fmt.Errorf("插件 %s 已存在，请先删除", pluginName)
	}
	if err := os.Rename(tempDir, finalDir); err != nil {
		return fmt.Errorf("提交插件目录失败: %v", err)
	}

	writePluginSource(pluginName, "store")
	return nil
}

func downloadFileWithRetry(url, filepath string, retries int, githubToken string) error {
	var err error
	for i := 0; i < retries; i++ {
		err = downloadFile(url, filepath, githubToken)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

func downloadFile(urlStr, filepath string, githubToken string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}

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