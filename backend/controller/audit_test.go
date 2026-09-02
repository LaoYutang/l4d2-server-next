package controller

import (
	"bytes"
	"encoding/json"
	"l4d2-manager-next/db"
	"l4d2-manager-next/model"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLogOpRecordsFinalRequestResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	records := make([]model.AuditLog, 0, 4)
	oldEnqueueAuditLog := enqueueAuditLog
	enqueueAuditLog = func(entry model.AuditLog) {
		records = append(records, entry)
	}
	t.Cleanup(func() { enqueueAuditLog = oldEnqueueAuditLog })

	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/success", func(c *gin.Context) {
		c.Set("role", "admin")
		defer LogOp(c, "success detail")()
		c.Status(http.StatusNoContent)
	})
	router.GET("/failure", func(c *gin.Context) {
		c.Set("role", "guest")
		defer LogOp(c, "failure detail")()
		c.String(http.StatusBadRequest, "bad request")
	})
	router.GET("/map-upload", func(c *gin.Context) {
		c.Set("role", "map_uploader")
		defer LogOp(c, "upload detail")()
		c.Status(http.StatusNoContent)
	})
	router.GET("/panic", func(c *gin.Context) {
		c.Set("role", "admin")
		defer LogOp(c, "panic detail")()
		panic("test panic")
	})

	for _, path := range []string{"/success", "/failure", "/map-upload", "/panic"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
	}

	if len(records) != 4 {
		t.Fatalf("expected 4 audit records, got %d", len(records))
	}
	if !records[0].Success || records[0].Role != "admin" || records[0].Detail != "success detail" {
		t.Fatalf("unexpected success record: %+v", records[0])
	}
	if records[1].Success || records[1].Role != "guest" || records[1].Detail != "failure detail" {
		t.Fatalf("unexpected failure record: %+v", records[1])
	}
	if !records[2].Success || records[2].Role != "map_uploader" || records[2].Detail != "upload detail" {
		t.Fatalf("unexpected map uploader record: %+v", records[2])
	}
	if records[3].Success || records[3].Detail != "panic detail" {
		t.Fatalf("unexpected panic record: %+v", records[3])
	}
}

func TestListAuditLogsRejectsGuest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(AuditListRequest{Page: 1, PageSize: 20, Role: "map_uploader"})
	request := httptest.NewRequest(http.MethodPost, "/audit/list", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request
	context.Set("role", "guest")

	ListAuditLogs(context)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", response.Code)
	}
}

func TestListAuditLogsReturnsServiceUnavailableWithoutDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldAuditDB := db.AuditDB
	db.AuditDB = nil
	t.Cleanup(func() { db.AuditDB = oldAuditDB })

	body, _ := json.Marshal(AuditListRequest{Page: 1, PageSize: 20, Role: "map_uploader"})
	request := httptest.NewRequest(http.MethodPost, "/audit/list", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = request
	context.Set("role", "admin")

	ListAuditLogs(context)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", response.Code)
	}
}
