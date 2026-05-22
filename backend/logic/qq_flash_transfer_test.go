package logic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestExtractQQFlashTransferNuxtSharePageInfo(t *testing.T) {
	htmlText := `<html><head><title>ignored</title></head><body>` +
		`<script type="application/json" data-nuxt-data="nuxt-app" id="__NUXT_DATA__">` +
		`[{"name":1,"total_file_count":2},"资料集合",2,` +
		`false,{"download_limit_status":3,"id":5,"size":13},"EhDownloadID1",` +
		`{"name":7,"fileset_id":8,"physical":4,"is_dir":3},"资料1.rar","set-id",` +
		`{"download_limit_status":3,"id":10,"file_size":14},"EhDownloadID2",` +
		`{"name":12,"fileset_id":8,"physical":9,"is_dir":3},"资料2.txt",12345,"67890"]` +
		`</script></body></html>`

	info, err := extractQQFlashTransferNuxtSharePageInfo(htmlText)
	if err != nil {
		t.Fatal(err)
	}

	if info.FileName != "资料集合" {
		t.Fatalf("FileName = %q, want %q", info.FileName, "资料集合")
	}
	if info.DownloadID != "EhDownloadID1" {
		t.Fatalf("DownloadID = %q, want %q", info.DownloadID, "EhDownloadID1")
	}
	if len(info.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(info.Files))
	}
	if info.Files[0].FileName != "资料1.rar" || info.Files[0].DownloadID != "EhDownloadID1" {
		t.Fatalf("Files[0] = %+v", info.Files[0])
	}
	if info.Files[0].FileSize != "12345" {
		t.Fatalf("Files[0].FileSize = %q, want 12345", info.Files[0].FileSize)
	}
	if info.Files[1].FileName != "资料2.txt" || info.Files[1].DownloadID != "EhDownloadID2" {
		t.Fatalf("Files[1] = %+v", info.Files[1])
	}
	if info.Files[1].FileSize != "67890" {
		t.Fatalf("Files[1].FileSize = %q, want 67890", info.Files[1].FileSize)
	}
}

func TestExtractQQFlashTransferLegacyDownloadID(t *testing.T) {
	htmlText := `{"download_limit_status":0},"LegacyDownloadID":{}`

	downloadID, err := extractQQFlashTransferLegacyDownloadID(htmlText)
	if err != nil {
		t.Fatal(err)
	}
	if downloadID != "LegacyDownloadID" {
		t.Fatalf("downloadID = %q, want %q", downloadID, "LegacyDownloadID")
	}
}

func TestDownloadItemSupportState(t *testing.T) {
	for _, filename := range []string{"a.vpk", "b.zip", "c.rar", "d.7z", "地图.VPK"} {
		supported, reason := downloadItemSupportState(filename)
		if !supported || reason != "" {
			t.Fatalf("%s supported=%v reason=%q, want supported", filename, supported, reason)
		}
	}

	supported, reason := downloadItemSupportState("readme.txt")
	if supported {
		t.Fatal("readme.txt should be unsupported")
	}
	if reason == "" {
		t.Fatal("unsupported file should include disabled reason")
	}
}

func TestGetQQFlashTransferDirectURLBatchMapsByBatchID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Referer") != "https://qfile.qq.com/q/share-id" {
			t.Fatalf("Referer = %q", r.Header.Get("Referer"))
		}

		var payload struct {
			DownloadInfo []struct {
				BatchID string `json:"batch_id"`
			} `json:"download_info"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.DownloadInfo) != 2 {
			t.Fatalf("len(download_info) = %d, want 2", len(payload.DownloadInfo))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"retcode": 0,
			"data": {
				"download_rsp": [
					{"url": "https://download.example/two", "batch_id": "id2"},
					{"url": "https://download.example/one", "batch_id": "id1"}
				]
			}
		}`))
	}))
	defer server.Close()

	oldURL := qqFlashTransferBatchDownloadURL
	qqFlashTransferBatchDownloadURL = server.URL
	defer func() {
		qqFlashTransferBatchDownloadURL = oldURL
	}()

	tasks, err := getQQFlashTransferDirectURLBatch(server.Client(), "https://qfile.qq.com/q/share-id", []qqFlashTransferShareFile{
		{FileName: "one.vpk", DownloadID: "id1", FileSize: "111"},
		{FileName: "two.zip", DownloadID: "id2", FileSize: "222"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}

	assertTask := func(index int, wantName string, wantBase string, wantSize string) {
		t.Helper()
		if tasks[index].FileName != wantName {
			t.Fatalf("tasks[%d].FileName = %q, want %q", index, tasks[index].FileName, wantName)
		}
		if tasks[index].FileSize != wantSize {
			t.Fatalf("tasks[%d].FileSize = %q, want %q", index, tasks[index].FileSize, wantSize)
		}
		parsedURL, err := url.Parse(tasks[index].DirectURL)
		if err != nil {
			t.Fatal(err)
		}
		if parsedURL.Scheme+"://"+parsedURL.Host+parsedURL.Path != wantBase {
			t.Fatalf("tasks[%d].DirectURL base = %q, want %q", index, parsedURL.String(), wantBase)
		}
		if parsedURL.Query().Get("filename") != wantName {
			t.Fatalf("filename query = %q, want %q", parsedURL.Query().Get("filename"), wantName)
		}
	}

	assertTask(0, "one.vpk", "https://download.example/one", "111")
	assertTask(1, "two.zip", "https://download.example/two", "222")
}

func TestProbeQQFlashTransferDirectFileSizeUsesContentRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://qfile.qq.com/q/share-id" {
			t.Fatalf("Referer = %q", r.Header.Get("Referer"))
		}

		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.Header.Get("Range") != "bytes=0-0" {
			t.Fatalf("Range = %q", r.Header.Get("Range"))
		}

		w.Header().Set("Content-Range", "bytes 0-0/418943098")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte{0})
	}))
	defer server.Close()

	size := probeQQFlashTransferDirectFileSize(server.Client(), server.URL+"/download", "https://qfile.qq.com/q/share-id")
	if size != "418943098" {
		t.Fatalf("size = %q, want 418943098", size)
	}
}
