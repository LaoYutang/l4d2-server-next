package logic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var qqFlashTransferBatchDownloadURL = "https://qfile.qq.com/http2rpc/gotrpc/noauth/trpc.qqntv2.richmedia.InnerProxy/BatchDownload"

type qqFlashTransferBatchDownloadResp struct {
	RetCode int `json:"retcode"`
	Data    struct {
		DownloadRsp []struct {
			URL     string `json:"url"`
			RetCode string `json:"ret_code"`
			RetMsg  string `json:"ret_msg"`
			BatchID string `json:"batch_id"`
		} `json:"download_rsp"`
	} `json:"data"`
	Msg string `json:"msg"`
}

type qqFlashTransferSharePageInfo struct {
	FileName   string
	DownloadID string
	FileSize   string
	Files      []qqFlashTransferShareFile
}

type qqFlashTransferShareFile struct {
	FileName   string
	DownloadID string
	FileSize   string
}

type qqFlashTransferDownloadFile struct {
	ID        string
	FileName  string
	FileSize  string
	DirectURL string
}

func ParseQQFlashTransferLink(shareURL string) (LinkParseResult, error) {
	files, err := parseQQFlashTransferFiles(shareURL)
	if err != nil {
		return LinkParseResult{}, err
	}
	if len(files) == 0 {
		return LinkParseResult{}, errors.New("没有可下载的文件")
	}

	items := make([]LinkParseItem, 0, len(files))
	for _, file := range files {
		filename := cleanLinkParseFilename(file.FileName)
		if filename == "" {
			filename = "qq_flash_transfer"
		}
		supported, disabledReason := downloadItemSupportState(filename)
		items = append(items, LinkParseItem{
			ID:             file.ID,
			Title:          filename,
			Filename:       filename,
			FileSize:       file.FileSize,
			FileURL:        file.DirectURL,
			Referer:        shareURL,
			Supported:      supported,
			DisabledReason: disabledReason,
		})
	}

	return LinkParseResult{
		SourceType: LinkSourceQQFlashTransfer,
		SourceID:   qqFlashTransferSourceID(shareURL),
		Items:      items,
	}, nil
}

func parseQQFlashTransferFiles(shareURL string) ([]qqFlashTransferDownloadFile, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodGet, shareURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultLinkParseUA())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("打开 QQ 闪传分享页失败，HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	pageInfo, err := extractQQFlashTransferSharePageInfo(string(bodyBytes))
	if err != nil {
		return nil, err
	}

	files := pageInfo.Files
	if len(files) == 0 && pageInfo.DownloadID != "" {
		fileName := pageInfo.FileName
		if fileName == "" {
			fileName = pageInfo.DownloadID
		}
		files = []qqFlashTransferShareFile{{
			FileName:   fileName,
			DownloadID: pageInfo.DownloadID,
			FileSize:   pageInfo.FileSize,
		}}
	}
	if len(files) == 0 {
		return nil, errors.New("没有可下载的文件")
	}

	return getQQFlashTransferDirectURLs(client, shareURL, files)
}

func extractQQFlashTransferSharePageInfo(htmlText string) (qqFlashTransferSharePageInfo, error) {
	info, nuxtErr := extractQQFlashTransferNuxtSharePageInfo(htmlText)
	if nuxtErr == nil {
		if info.FileName == "" {
			info.FileName = extractQQFlashTransferFileNameFromTitle(htmlText)
		}
		return info, nil
	}

	downloadID, legacyErr := extractQQFlashTransferLegacyDownloadID(htmlText)
	if legacyErr != nil {
		return qqFlashTransferSharePageInfo{}, fmt.Errorf("没有从页面中提取到下载 ID，Nuxt 解析失败: %v；旧结构解析失败: %v", nuxtErr, legacyErr)
	}

	return qqFlashTransferSharePageInfo{
		FileName:   extractQQFlashTransferFileNameFromTitle(htmlText),
		DownloadID: downloadID,
	}, nil
}

func extractQQFlashTransferNuxtSharePageInfo(htmlText string) (qqFlashTransferSharePageInfo, error) {
	payload, err := extractQQFlashTransferNuxtDataPayload(htmlText)
	if err != nil {
		return qqFlashTransferSharePageInfo{}, err
	}

	values, err := decodeQQFlashTransferNuxtValues(payload)
	if err != nil {
		return qqFlashTransferSharePageInfo{}, err
	}

	files := findQQFlashTransferNuxtFiles(values)
	downloadID := ""
	if len(files) > 0 {
		downloadID = files[0].DownloadID
	} else {
		downloadID = findQQFlashTransferNuxtPhysicalID(values)
	}
	if downloadID == "" {
		return qqFlashTransferSharePageInfo{}, errors.New("没有从 __NUXT_DATA__ 中找到 physical.id")
	}

	return qqFlashTransferSharePageInfo{
		FileName:   findQQFlashTransferNuxtShareName(values),
		DownloadID: downloadID,
		FileSize:   findQQFlashTransferNuxtPhysicalFileSize(values),
		Files:      files,
	}, nil
}

func decodeQQFlashTransferNuxtValues(payload string) ([]json.RawMessage, error) {
	var values []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &values); err != nil {
		return nil, fmt.Errorf("解析 __NUXT_DATA__ 失败: %w", err)
	}
	return values, nil
}

func extractQQFlashTransferNuxtDataPayload(htmlText string) (string, error) {
	re := regexp.MustCompile(`(?is)<script[^>]*\bid=["']__NUXT_DATA__["'][^>]*>(.*?)</script>`)
	matches := re.FindStringSubmatch(htmlText)
	if len(matches) < 2 {
		return "", errors.New("没有找到 __NUXT_DATA__")
	}

	payload := strings.TrimSpace(html.UnescapeString(matches[1]))
	if payload == "" {
		return "", errors.New("__NUXT_DATA__ 为空")
	}

	return payload, nil
}

func findQQFlashTransferNuxtPhysicalID(values []json.RawMessage) string {
	for _, raw := range values {
		obj, ok := qqFlashTransferNuxtObject(raw)
		if !ok {
			continue
		}

		if _, ok := obj["download_limit_status"]; !ok {
			if _, ok := obj["downloadLimitStatus"]; !ok {
				continue
			}
		}

		idRaw, ok := obj["id"]
		if !ok {
			continue
		}

		if id, ok := qqFlashTransferNuxtStringValue(values, idRaw); ok && id != "" {
			return id
		}
	}

	return ""
}

func findQQFlashTransferNuxtPhysicalFileSize(values []json.RawMessage) string {
	for _, raw := range values {
		obj, ok := qqFlashTransferNuxtObject(raw)
		if !ok {
			continue
		}

		if _, ok := obj["download_limit_status"]; !ok {
			if _, ok := obj["downloadLimitStatus"]; !ok {
				continue
			}
		}

		if size := findQQFlashTransferFileSize(values, obj); size != "" {
			return size
		}
	}

	return ""
}

func findQQFlashTransferNuxtFiles(values []json.RawMessage) []qqFlashTransferShareFile {
	files := make([]qqFlashTransferShareFile, 0)
	seen := make(map[string]bool)

	for _, raw := range values {
		obj, ok := qqFlashTransferNuxtObject(raw)
		if !ok {
			continue
		}

		physicalRaw, ok := obj["physical"]
		if !ok {
			continue
		}

		if isDir, ok := qqFlashTransferNuxtBoolValue(values, obj["is_dir"]); ok && isDir {
			continue
		}

		physicalObj, ok := qqFlashTransferNuxtObjectValue(values, physicalRaw)
		if !ok {
			continue
		}

		idRaw, ok := physicalObj["id"]
		if !ok {
			continue
		}

		downloadID, ok := qqFlashTransferNuxtStringValue(values, idRaw)
		if !ok || downloadID == "" || seen[downloadID] {
			continue
		}

		fileName := downloadID
		if nameRaw, ok := obj["name"]; ok {
			if name, ok := qqFlashTransferNuxtStringValue(values, nameRaw); ok && name != "" {
				fileName = name
			}
		}

		files = append(files, qqFlashTransferShareFile{
			FileName:   cleanLinkParseFilename(fileName),
			DownloadID: downloadID,
			FileSize:   findQQFlashTransferFileSize(values, obj, physicalObj),
		})
		seen[downloadID] = true
	}

	return files
}

func findQQFlashTransferNuxtShareName(values []json.RawMessage) string {
	for _, raw := range values {
		obj, ok := qqFlashTransferNuxtObject(raw)
		if !ok {
			continue
		}

		if _, ok := obj["physical"]; ok {
			continue
		}
		if !looksLikeQQFlashTransferShareObject(obj) {
			continue
		}

		nameRaw, ok := obj["name"]
		if !ok {
			continue
		}
		if name, ok := qqFlashTransferNuxtStringValue(values, nameRaw); ok && name != "" {
			return cleanLinkParseFilename(name)
		}
	}

	return ""
}

func looksLikeQQFlashTransferShareObject(obj map[string]json.RawMessage) bool {
	for _, key := range []string{"total_file_count", "totalFileCount", "share_info", "shareInfo"} {
		if _, ok := obj[key]; ok {
			return true
		}
	}
	return false
}

func findQQFlashTransferFileSize(values []json.RawMessage, objects ...map[string]json.RawMessage) string {
	sizeKeys := []string{
		"file_size",
		"fileSize",
		"filesize",
		"size",
		"bytes",
		"file_byte_size",
		"fileByteSize",
	}

	for _, obj := range objects {
		for _, key := range sizeKeys {
			raw, ok := obj[key]
			if !ok {
				continue
			}
			if size, ok := qqFlashTransferNuxtNumberStringValue(values, raw); ok && size != "" && size != "0" {
				return size
			}
		}
	}

	return ""
}

func qqFlashTransferNuxtObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	return obj, true
}

func qqFlashTransferNuxtObjectValue(values []json.RawMessage, raw json.RawMessage) (map[string]json.RawMessage, bool) {
	resolvedRaw, ok := qqFlashTransferNuxtRawValue(values, raw)
	if !ok {
		return nil, false
	}
	return qqFlashTransferNuxtObject(resolvedRaw)
}

func qqFlashTransferNuxtRawValue(values []json.RawMessage, raw json.RawMessage) (json.RawMessage, bool) {
	var ref int
	if err := json.Unmarshal(raw, &ref); err == nil && ref >= 0 && ref < len(values) {
		return values[ref], true
	}
	return raw, true
}

func qqFlashTransferNuxtStringValue(values []json.RawMessage, raw json.RawMessage) (string, bool) {
	var direct string
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, true
	}

	var ref int
	if err := json.Unmarshal(raw, &ref); err != nil {
		return "", false
	}
	if ref < 0 || ref >= len(values) {
		return "", false
	}

	var resolved string
	if err := json.Unmarshal(values[ref], &resolved); err != nil {
		return "", false
	}
	return resolved, true
}

func qqFlashTransferNuxtNumberStringValue(values []json.RawMessage, raw json.RawMessage) (string, bool) {
	resolvedRaw, ok := qqFlashTransferNuxtRawValue(values, raw)
	if !ok {
		return "", false
	}

	var intValue int64
	if err := json.Unmarshal(resolvedRaw, &intValue); err == nil {
		return strconv.FormatInt(intValue, 10), true
	}

	var floatValue float64
	if err := json.Unmarshal(resolvedRaw, &floatValue); err == nil {
		if floatValue <= 0 {
			return "", true
		}
		return strconv.FormatInt(int64(floatValue), 10), true
	}

	var stringValue string
	if err := json.Unmarshal(resolvedRaw, &stringValue); err == nil {
		stringValue = strings.TrimSpace(stringValue)
		if stringValue == "" {
			return "", true
		}
		if intValue, err := strconv.ParseInt(stringValue, 10, 64); err == nil {
			return strconv.FormatInt(intValue, 10), true
		}
		if floatValue, err := strconv.ParseFloat(stringValue, 64); err == nil {
			return strconv.FormatInt(int64(floatValue), 10), true
		}
		return stringValue, true
	}

	return "", false
}

func qqFlashTransferNuxtBoolValue(values []json.RawMessage, raw json.RawMessage) (bool, bool) {
	if len(raw) == 0 {
		return false, false
	}

	resolvedRaw, ok := qqFlashTransferNuxtRawValue(values, raw)
	if !ok {
		return false, false
	}

	var value bool
	if err := json.Unmarshal(resolvedRaw, &value); err != nil {
		return false, false
	}
	return value, true
}

func extractQQFlashTransferLegacyDownloadID(htmlText string) (string, error) {
	keyword := `"download_limit_status"`
	startIndex := strings.Index(htmlText, keyword)
	if startIndex < 0 {
		return "", errors.New("没有找到 download_limit_status，页面结构可能变了")
	}

	re := regexp.MustCompile(`\},"([^"]+)"\s*:`)
	matches := re.FindStringSubmatch(htmlText[startIndex:])
	if len(matches) < 2 {
		return "", errors.New("没有从页面中提取到下载 ID")
	}

	downloadID := strings.TrimSpace(matches[1])
	if downloadID == "" {
		return "", errors.New("下载 ID 为空")
	}

	return downloadID, nil
}

func extractQQFlashTransferFileNameFromTitle(htmlText string) string {
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	matches := re.FindStringSubmatch(htmlText)
	if len(matches) < 2 {
		return ""
	}

	title := html.UnescapeString(strings.TrimSpace(matches[1]))
	replacer := strings.NewReplacer(
		"\n", "",
		"\r", "",
		"\t", "",
		"｜QQ闪传", "",
		"|QQ闪传", "",
		" - QQ闪传", "",
		"_QQ闪传", "",
		"QQ闪传", "",
	)

	return cleanLinkParseFilename(replacer.Replace(title))
}

func getQQFlashTransferDirectURLs(client *http.Client, shareURL string, files []qqFlashTransferShareFile) ([]qqFlashTransferDownloadFile, error) {
	tasks := make([]qqFlashTransferDownloadFile, 0, len(files))
	for start := 0; start < len(files); start += 30 {
		end := start + 30
		if end > len(files) {
			end = len(files)
		}

		chunkTasks, err := getQQFlashTransferDirectURLBatch(client, shareURL, files[start:end])
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, chunkTasks...)
	}

	return tasks, nil
}

func getQQFlashTransferDirectURLBatch(client *http.Client, shareURL string, files []qqFlashTransferShareFile) ([]qqFlashTransferDownloadFile, error) {
	downloadInfo := make([]map[string]any, 0, len(files))
	for _, file := range files {
		downloadInfo = append(downloadInfo, map[string]any{
			"batch_id": file.DownloadID,
			"scene": map[string]any{
				"business_type": 4,
				"app_type":      22,
				"scene_type":    5,
			},
			"index_node": map[string]any{
				"file_uuid": file.DownloadID,
			},
			"url_type":       2,
			"download_scene": 0,
		})
	}

	payload := map[string]any{
		"req_head": map[string]any{
			"agent": 8,
		},
		"download_info": downloadInfo,
		"scene_type":    103,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, qqFlashTransferBatchDownloadURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultLinkParseUA())
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://qfile.qq.com")
	req.Header.Set("Referer", shareURL)
	req.Header.Set("x-oidb", `{"uint32_command":"0x9248","uint32_service_type":"4"}`)
	req.Header.Set("Cookie", "uin=9000002; p_uin=9000002")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BatchDownload 接口失败，HTTP %d，返回: %s", resp.StatusCode, string(bodyBytes))
	}

	var result qqFlashTransferBatchDownloadResp
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析 BatchDownload 返回失败: %w，原始返回: %s", err, string(bodyBytes))
	}

	if result.RetCode != 0 {
		return nil, fmt.Errorf("BatchDownload retcode=%d msg=%s", result.RetCode, result.Msg)
	}
	if len(result.Data.DownloadRsp) == 0 {
		return nil, fmt.Errorf("BatchDownload 没有返回下载结果，原始返回: %s", string(bodyBytes))
	}

	rspByID := make(map[string]struct {
		URL     string
		RetCode string
		RetMsg  string
		BatchID string
	})
	for _, downloadRsp := range result.Data.DownloadRsp {
		rspByID[downloadRsp.BatchID] = struct {
			URL     string
			RetCode string
			RetMsg  string
			BatchID string
		}{
			URL:     downloadRsp.URL,
			RetCode: downloadRsp.RetCode,
			RetMsg:  downloadRsp.RetMsg,
			BatchID: downloadRsp.BatchID,
		}
	}

	tasks := make([]qqFlashTransferDownloadFile, 0, len(files))
	for index, file := range files {
		downloadRsp, ok := rspByID[file.DownloadID]
		if !ok && len(files) == 1 && len(result.Data.DownloadRsp) == 1 {
			onlyRsp := result.Data.DownloadRsp[0]
			downloadRsp = struct {
				URL     string
				RetCode string
				RetMsg  string
				BatchID string
			}{
				URL:     onlyRsp.URL,
				RetCode: onlyRsp.RetCode,
				RetMsg:  onlyRsp.RetMsg,
				BatchID: onlyRsp.BatchID,
			}
			ok = true
		}
		if !ok {
			return nil, fmt.Errorf("BatchDownload 没有返回第 %d 个文件的下载结果，文件名=%s batch_id=%s，原始返回: %s",
				index+1,
				file.FileName,
				file.DownloadID,
				string(bodyBytes),
			)
		}

		if downloadRsp.URL == "" {
			return nil, fmt.Errorf("BatchDownload 没有返回下载 URL，文件名=%s batch_id=%s ret_code=%s ret_msg=%s，原始返回: %s",
				file.FileName,
				downloadRsp.BatchID,
				downloadRsp.RetCode,
				downloadRsp.RetMsg,
				string(bodyBytes),
			)
		}

		fileName := cleanLinkParseFilename(file.FileName)
		if fileName == "" {
			fileName = file.DownloadID
		}
		fileSize := file.FileSize

		directURL := appendQQFlashTransferFilename(downloadRsp.URL, fileName)
		if fileSize == "" {
			fileSize = probeQQFlashTransferDirectFileSize(client, directURL, shareURL)
		}

		tasks = append(tasks, qqFlashTransferDownloadFile{
			ID:        file.DownloadID,
			FileName:  fileName,
			FileSize:  fileSize,
			DirectURL: directURL,
		})
	}

	return tasks, nil
}

func probeQQFlashTransferDirectFileSize(client *http.Client, directURL string, referer string) string {
	if size := requestQQFlashTransferDirectFileSize(client, http.MethodHead, directURL, referer); size != "" {
		return size
	}
	return requestQQFlashTransferDirectFileSize(client, http.MethodGet, directURL, referer)
}

func requestQQFlashTransferDirectFileSize(client *http.Client, method string, directURL string, referer string) string {
	req, err := http.NewRequest(method, directURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", defaultLinkParseUA())
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if total := parseQQFlashTransferContentRangeTotal(resp.Header.Get("Content-Range")); total > 0 {
		return strconv.FormatInt(total, 10)
	}
	if resp.ContentLength > 0 {
		return strconv.FormatInt(resp.ContentLength, 10)
	}

	return ""
}

func parseQQFlashTransferContentRangeTotal(contentRange string) int64 {
	slashIndex := strings.LastIndex(contentRange, "/")
	if slashIndex < 0 {
		return -1
	}

	totalText := strings.TrimSpace(contentRange[slashIndex+1:])
	if totalText == "" || totalText == "*" {
		return -1
	}

	total, err := strconv.ParseInt(totalText, 10, 64)
	if err != nil {
		return -1
	}

	return total
}

func appendQQFlashTransferFilename(rawURL string, fileName string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	q := u.Query()
	q.Set("filename", fileName)
	u.RawQuery = q.Encode()

	return u.String()
}

func qqFlashTransferSourceID(shareURL string) string {
	u, err := url.Parse(shareURL)
	if err != nil {
		return shareURL
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "q" && parts[1] != "" {
		return parts[1]
	}

	return shareURL
}

func defaultLinkParseUA() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
}
