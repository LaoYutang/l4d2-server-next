package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"l4d2-manager-next/logic"
	"l4d2-manager-next/middlewares"

	"github.com/gin-gonic/gin"
)

func TestAuthCodeLogStreamClosesWhenAuthorizationInvalidated(t *testing.T) {
	for _, operation := range []string{"revoke", "delete", "expire"} {
		t.Run(operation, func(t *testing.T) {
			store := setupAuthControllerStore(t)
			logsDir := setupSourceModLogControllerTestDir(t)
			if err := os.WriteFile(filepath.Join(logsDir, "stream.log"), []byte("initial history\n"), 0644); err != nil {
				t.Fatal(err)
			}
			created, err := store.Create(logic.CreateAuthCodeRequest{Code: "streamAuth"})
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.GET("/logs/stream", middlewares.Auth(), StreamSourceModLog)
			server := httptest.NewServer(router)
			defer server.Close()
			client := server.Client()
			client.Timeout = 4 * time.Second
			request, _ := http.NewRequest("GET", server.URL+"/logs/stream?file=stream.log", nil)
			request.Header.Set("Authorization", "Bearer "+created.Code)
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatalf("stream: %d", response.StatusCode)
			}
			switch operation {
			case "revoke":
				_, err = store.Revoke(created.ID)
			case "delete":
				_, err = store.Delete(created.ID)
			case "expire":
				past := time.Now().Add(-time.Minute)
				_, err = store.Update(logic.UpdateAuthCodeRequest{ID: created.ID, ExpiresAt: &past})
			}
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil || !strings.Contains(string(body), "event:auth-expired") || !strings.Contains(string(body), "initial history") {
				t.Fatalf("stream did not close with auth event: %q %v", body, err)
			}
		})
	}
}
