package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultPluginStoreRepo         = "LaoYutang/l4d2-plugins-store"
	StorePluginDownloadConcurrency = 3
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

type StorePluginDownloadStatus string

const (
	StorePluginDownloadStatusPending     StorePluginDownloadStatus = "pending"
	StorePluginDownloadStatusDownloading StorePluginDownloadStatus = "downloading"
	StorePluginDownloadStatusCompleted   StorePluginDownloadStatus = "completed"
	StorePluginDownloadStatusFailed      StorePluginDownloadStatus = "failed"
	StorePluginDownloadStatusCancelled   StorePluginDownloadStatus = "cancelled"
)

type StorePluginDownloadProgress struct {
	Name       string                    `json:"name"`
	Repo       string                    `json:"repo"`
	Status     StorePluginDownloadStatus `json:"status"`
	Downloaded int                       `json:"downloaded"`
	Total      int                       `json:"total"`
	Message    string                    `json:"message"`
}

type storePluginDownloadTask struct {
	name        string
	repo        string
	proxyUrl    string
	githubToken string
	prefix      string
	files       []string
	tempDir     string
	finalDir    string
	ctx         context.Context
	cancel      context.CancelFunc

	mu         sync.RWMutex
	status     StorePluginDownloadStatus
	downloaded int
	total      int
	message    string
}

type gitLFSPointer struct {
	OID  string
	Size int64
}

type gitLFSBatchRequest struct {
	Operation string                `json:"operation"`
	Transfers []string              `json:"transfers,omitempty"`
	Ref       gitLFSRef             `json:"ref"`
	Objects   []gitLFSObjectRequest `json:"objects"`
}

type gitLFSRef struct {
	Name string `json:"name"`
}

type gitLFSObjectRequest struct {
	OID  string `json:"oid"`
	Size int64  `json:"size"`
}

type gitLFSBatchResponse struct {
	Objects []gitLFSObjectResponse `json:"objects"`
}

type gitLFSObjectResponse struct {
	OID     string             `json:"oid"`
	Size    int64              `json:"size"`
	Actions gitLFSActions      `json:"actions"`
	Error   *gitLFSObjectError `json:"error,omitempty"`
}

type gitLFSActions struct {
	Download *gitLFSAction `json:"download"`
}

type gitLFSAction struct {
	Href   string            `json:"href"`
	Header map[string]string `json:"header,omitempty"`
}

type gitLFSObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	treeCache              = make(map[string]*GitHubTreeResponse)
	treeCacheTime          = make(map[string]time.Time)
	treeCacheMut           sync.Mutex
	storeDownloadTaskMut   sync.Mutex
	storeDownloadTasks     = make(map[string]*storePluginDownloadTask)
	storeDownloadSemaphore = make(chan struct{}, StorePluginDownloadConcurrency)
)

func normalizeStoreRepo(repo string) string {
	if repo == "" {
		return DefaultPluginStoreRepo
	}
	return repo
}

func getStoreDownloadTaskKey(repo, pluginName string) string {
	return normalizeStoreRepo(repo) + "\x00" + pluginName
}

func applyProxy(proxyUrl, rawUrl string) string {
	if proxyUrl == "" {
		return rawUrl
	}
	return strings.TrimSuffix(proxyUrl, "/") + "/" + rawUrl
}

func getTreeData(forceRefresh bool, proxyUrl, githubToken, repo string) (*GitHubTreeResponse, error) {
	repo = normalizeStoreRepo(repo)

	treeCacheMut.Lock()
	defer treeCacheMut.Unlock()

	if !forceRefresh && time.Since(treeCacheTime[repo]) < 10*time.Minute && treeCache[repo] != nil {
		return treeCache[repo], nil
	}

	client := &http.Client{Timeout: 10 * time.Second}

	rawUrl := fmt.Sprintf("https://api.github.com/repos/%s/git/trees/master?recursive=1", repo)
	fetchUrl := applyProxy(proxyUrl, rawUrl)

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

func (t *storePluginDownloadTask) snapshot() StorePluginDownloadProgress {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return StorePluginDownloadProgress{
		Name:       t.name,
		Repo:       t.repo,
		Status:     t.status,
		Downloaded: t.downloaded,
		Total:      t.total,
		Message:    t.message,
	}
}

func (t *storePluginDownloadTask) isActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.status == StorePluginDownloadStatusPending || t.status == StorePluginDownloadStatusDownloading
}

func (t *storePluginDownloadTask) setStatus(status StorePluginDownloadStatus, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.status = status
	t.message = message
}

func (t *storePluginDownloadTask) incrementDownloaded() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.downloaded++
}

func (t *storePluginDownloadTask) requestCancel() {
	t.cancel()

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StorePluginDownloadStatusPending || t.status == StorePluginDownloadStatusDownloading {
		t.status = StorePluginDownloadStatusCancelled
		t.message = "下载已取消"
	}
}

func findStorePluginFiles(tree *GitHubTreeResponse, pluginName string) ([]string, string) {
	var filesToDownload []string
	prefix := "plugins/" + pluginName + "/"
	for _, item := range tree.Tree {
		if item.Type == "blob" && strings.HasPrefix(item.Path, prefix) {
			filesToDownload = append(filesToDownload, item.Path)
		}
	}
	return filesToDownload, prefix
}

// CleanDownloadTemp 启动时整体清空 .download_temp/，删除上次运行残留。
// 调用点位于 main.go 的 router.Run 之前，此时 HTTP 服务尚未对外提供，
// 不可能存在正在进行的下载，整体 RemoveAll 安全。
func CleanDownloadTemp() {
	os.RemoveAll(filepath.Join(getStorePath(), DownloadTempDir))
}

func StartStorePluginDownload(pluginName, proxyUrl, githubToken, repo string) (StorePluginDownloadProgress, error) {
	repo = normalizeStoreRepo(repo)

	tree, err := getTreeData(false, proxyUrl, githubToken, repo)
	if err != nil {
		return StorePluginDownloadProgress{}, err
	}

	filesToDownload, prefix := findStorePluginFiles(tree, pluginName)
	if len(filesToDownload) == 0 {
		return StorePluginDownloadProgress{}, fmt.Errorf("未找到插件或插件为空")
	}

	storePath := getStorePath()
	finalDir := filepath.Join(storePath, pluginName)
	if _, err := os.Stat(finalDir); !os.IsNotExist(err) {
		return StorePluginDownloadProgress{}, fmt.Errorf("插件 %s 已存在，请先删除", pluginName)
	}

	tempDir := getDownloadTempPath(uuid.New().String())
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return StorePluginDownloadProgress{}, fmt.Errorf("创建临时目录失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &storePluginDownloadTask{
		name:        pluginName,
		repo:        repo,
		proxyUrl:    strings.TrimSuffix(proxyUrl, "/"),
		githubToken: githubToken,
		prefix:      prefix,
		files:       filesToDownload,
		tempDir:     tempDir,
		finalDir:    finalDir,
		ctx:         ctx,
		cancel:      cancel,
		status:      StorePluginDownloadStatusPending,
		total:       len(filesToDownload),
		message:     "等待下载",
	}

	key := getStoreDownloadTaskKey(repo, pluginName)
	storeDownloadTaskMut.Lock()
	if existing, ok := storeDownloadTasks[key]; ok {
		if existing.isActive() {
			progress := existing.snapshot()
			storeDownloadTaskMut.Unlock()
			cancel()
			os.RemoveAll(tempDir)
			return progress, nil
		}
		delete(storeDownloadTasks, key)
	}
	for _, existing := range storeDownloadTasks {
		if existing.name == pluginName && existing.isActive() {
			storeDownloadTaskMut.Unlock()
			cancel()
			os.RemoveAll(tempDir)
			return StorePluginDownloadProgress{}, fmt.Errorf("插件 %s 正在下载", pluginName)
		}
	}
	storeDownloadTasks[key] = task
	storeDownloadTaskMut.Unlock()

	go task.run()

	return task.snapshot(), nil
}

func ListStorePluginDownloadProgress(repo string) []StorePluginDownloadProgress {
	repo = normalizeStoreRepo(repo)

	storeDownloadTaskMut.Lock()
	defer storeDownloadTaskMut.Unlock()

	progress := make([]StorePluginDownloadProgress, 0, len(storeDownloadTasks))
	for _, task := range storeDownloadTasks {
		if task.repo == repo {
			progress = append(progress, task.snapshot())
		}
	}
	return progress
}

func CancelStorePluginDownload(pluginName, repo string) (StorePluginDownloadProgress, error) {
	repo = normalizeStoreRepo(repo)
	key := getStoreDownloadTaskKey(repo, pluginName)

	storeDownloadTaskMut.Lock()
	task, ok := storeDownloadTasks[key]
	storeDownloadTaskMut.Unlock()
	if !ok {
		return StorePluginDownloadProgress{}, fmt.Errorf("下载任务不存在")
	}

	task.requestCancel()
	return task.snapshot(), nil
}

func (t *storePluginDownloadTask) run() {
	defer t.cancel()

	if t.ctx.Err() != nil {
		os.RemoveAll(t.tempDir)
		t.setStatus(StorePluginDownloadStatusCancelled, "下载已取消")
		return
	}

	t.setStatus(StorePluginDownloadStatusDownloading, "下载中")
	if _, err := os.Stat(t.finalDir); !os.IsNotExist(err) {
		os.RemoveAll(t.tempDir)
		t.setStatus(StorePluginDownloadStatusFailed, fmt.Sprintf("插件 %s 已存在，请先删除", t.name))
		return
	}

	if err := t.downloadFiles(); err != nil {
		os.RemoveAll(t.tempDir)
		if t.ctx.Err() != nil {
			t.setStatus(StorePluginDownloadStatusCancelled, "下载已取消")
			return
		}
		t.setStatus(StorePluginDownloadStatusFailed, err.Error())
		return
	}

	if t.ctx.Err() != nil {
		os.RemoveAll(t.tempDir)
		t.setStatus(StorePluginDownloadStatusCancelled, "下载已取消")
		return
	}

	if _, err := os.Stat(t.finalDir); !os.IsNotExist(err) {
		os.RemoveAll(t.tempDir)
		t.setStatus(StorePluginDownloadStatusFailed, fmt.Sprintf("插件 %s 已存在，请先删除", t.name))
		return
	}
	if err := os.Rename(t.tempDir, t.finalDir); err != nil {
		os.RemoveAll(t.tempDir)
		t.setStatus(StorePluginDownloadStatusFailed, fmt.Sprintf("提交插件目录失败: %v", err))
		return
	}

	writePluginSource(t.name, "store")
	t.setStatus(StorePluginDownloadStatusCompleted, "插件下载成功")
}

func (t *storePluginDownloadTask) downloadFiles() error {
	workerCount := len(t.files)
	if workerCount > StorePluginDownloadConcurrency {
		workerCount = StorePluginDownloadConcurrency
	}

	jobs := make(chan string)
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				if t.ctx.Err() != nil {
					return
				}
				if err := t.downloadOneFile(path); err != nil {
					select {
					case errChan <- err:
					default:
					}
					t.cancel()
					return
				}
				t.incrementDownloaded()
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, file := range t.files {
			select {
			case <-t.ctx.Done():
				return
			case jobs <- file:
			}
		}
	}()

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
	}
	return t.ctx.Err()
}

func (t *storePluginDownloadTask) downloadOneFile(path string) error {
	select {
	case storeDownloadSemaphore <- struct{}{}:
		defer func() { <-storeDownloadSemaphore }()
	case <-t.ctx.Done():
		return t.ctx.Err()
	}

	relPath := strings.TrimPrefix(path, t.prefix)
	localPath := filepath.Join(t.tempDir, relPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	encodedPath := strings.Join(parts, "/")

	rawUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/master/%s", t.repo, encodedPath)
	downloadUrl := applyProxy(t.proxyUrl, rawUrl)

	if err := downloadFileWithRetry(t.ctx, downloadUrl, localPath, 3, t.githubToken); err != nil {
		return fmt.Errorf("下载文件 %s 失败: %v", relPath, err)
	}
	if err := t.downloadGitLFSObjectIfNeeded(localPath); err != nil {
		return fmt.Errorf("下载 LFS 文件 %s 失败: %v", relPath, err)
	}
	return nil
}

func downloadFileWithRetry(ctx context.Context, url, filepath string, retries int, githubToken string) error {
	headers := make(map[string]string)
	if githubToken != "" {
		headers["Authorization"] = "Bearer " + githubToken
	}
	return downloadFileWithHeadersRetry(ctx, url, filepath, retries, headers)
}

func downloadFileWithHeadersRetry(ctx context.Context, url, filepath string, retries int, headers map[string]string) error {
	var err error
	for i := 0; i < retries; i++ {
		err = downloadFile(ctx, url, filepath, headers)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if i < retries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
			}
		}
	}
	return err
}

func downloadFile(ctx context.Context, urlStr, filepath string, headers map[string]string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	for k, v := range headers {
		req.Header.Set(k, v)
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

func parseGitLFSPointer(path string) (*gitLFSPointer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return nil, err
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "version https://git-lfs.github.com/spec/v1" {
		return nil, nil
	}

	pointer := &gitLFSPointer{}
	sizeSet := false
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "oid sha256:") {
			pointer.OID = strings.TrimPrefix(line, "oid sha256:")
		} else if strings.HasPrefix(line, "size ") {
			size, err := strconv.ParseInt(strings.TrimPrefix(line, "size "), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("LFS pointer size 无效: %v", err)
			}
			pointer.Size = size
			sizeSet = true
		}
	}

	if pointer.OID == "" || !sizeSet {
		return nil, fmt.Errorf("LFS pointer 缺少 oid 或 size")
	}

	return pointer, nil
}

func (t *storePluginDownloadTask) downloadGitLFSObjectIfNeeded(localPath string) error {
	pointer, err := parseGitLFSPointer(localPath)
	if err != nil || pointer == nil {
		return err
	}

	action, err := t.getGitLFSDownloadAction(pointer)
	if err != nil {
		return err
	}

	downloadUrl := applyProxy(t.proxyUrl, action.Href)
	if err := downloadFileWithHeadersRetry(t.ctx, downloadUrl, localPath, 3, action.Header); err != nil {
		return err
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}
	if info.Size() != pointer.Size {
		return fmt.Errorf("LFS 文件大小不匹配，期望 %d，实际 %d", pointer.Size, info.Size())
	}

	return nil
}

func (t *storePluginDownloadTask) getGitLFSDownloadAction(pointer *gitLFSPointer) (*gitLFSAction, error) {
	payload := gitLFSBatchRequest{
		Operation: "download",
		Transfers: []string{"basic"},
		Ref:       gitLFSRef{Name: "refs/heads/master"},
		Objects: []gitLFSObjectRequest{
			{
				OID:  pointer.OID,
				Size: pointer.Size,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	rawUrl := fmt.Sprintf("https://github.com/%s.git/info/lfs/objects/batch", t.repo)
	req, err := http.NewRequestWithContext(t.ctx, "POST", applyProxy(t.proxyUrl, rawUrl), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	if t.githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+t.githubToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("LFS Batch API 返回 %s: %s", resp.Status, msg)
	}

	var batchResp gitLFSBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("解析 LFS Batch 响应失败: %v", err)
	}
	if len(batchResp.Objects) == 0 {
		return nil, fmt.Errorf("LFS Batch 响应缺少对象")
	}

	obj := batchResp.Objects[0]
	if obj.Error != nil {
		return nil, fmt.Errorf("LFS 对象错误 %d: %s", obj.Error.Code, obj.Error.Message)
	}
	if obj.Actions.Download == nil || obj.Actions.Download.Href == "" {
		return nil, fmt.Errorf("LFS Batch 响应缺少下载地址")
	}

	return obj.Actions.Download, nil
}

func FetchStorePluginReadme(name, proxyUrl, githubToken, repo string) (content string, fileName string, err error) {
	repo = normalizeStoreRepo(repo)

	// Reuse the cached tree data (same cache as store listing)
	tree, err := getTreeData(false, proxyUrl, githubToken, repo)
	if err != nil {
		return "", "", fmt.Errorf("获取仓库信息失败: %v", err)
	}

	// Find .md files in plugins/{name}/
	prefix := "plugins/" + name + "/"
	var mdFiles []string
	for _, item := range tree.Tree {
		if item.Type == "blob" && strings.HasPrefix(item.Path, prefix) {
			base := strings.TrimPrefix(item.Path, prefix)
			if strings.HasSuffix(strings.ToLower(base), ".md") {
				mdFiles = append(mdFiles, base)
			}
		}
	}

	if len(mdFiles) == 0 {
		return "", "", fmt.Errorf("商店插件 %s 没有说明文档", name)
	}

	// Prefer README.md
	selectedFile := mdFiles[0]
	for _, f := range mdFiles {
		if strings.EqualFold(f, "README.md") {
			selectedFile = f
			break
		}
	}

	// Download the selected md file content only
	parts := strings.Split(prefix+selectedFile, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	encodedPath := strings.Join(parts, "/")

	rawContentUrl := fmt.Sprintf("https://raw.githubusercontent.com/%s/master/%s", repo, encodedPath)
	downloadUrl := rawContentUrl
	if proxyUrl != "" {
		proxyUrl = strings.TrimSuffix(proxyUrl, "/")
		downloadUrl = proxyUrl + "/" + rawContentUrl
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	if githubToken != "" {
		req.Header.Set("Authorization", "Bearer "+githubToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("下载说明文档失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("下载说明文档 HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取说明文档失败: %v", err)
	}

	return string(data), selectedFile, nil
}
