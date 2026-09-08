package controller

import (
	"encoding/json"
	"errors"
	"l4d2-manager-next/logic"
	"l4d2-manager-next/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupMapQueueControllerTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	originalSnapshot := getMapQueueSnapshot
	originalAction := executeMapQueueAction
	originalAudit := enqueueAuditLog
	enqueueAuditLog = func(model.AuditLog) {}
	t.Cleanup(func() {
		getMapQueueSnapshot = originalSnapshot
		executeMapQueueAction = originalAction
		enqueueAuditLog = originalAudit
	})
}

func newMapQueueTestContext(path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("role", "guest")
	return context, response
}

func TestMapQueueSnapshotController(t *testing.T) {
	setupMapQueueControllerTest(t)
	state := logic.MapQueueStateStopped
	getMapQueueSnapshot = func() (logic.MapQueueSnapshot, error) {
		return logic.MapQueueSnapshot{
			Installed: true,
			Enabled:   true,
			Supported: true,
			State:     &state,
			Pending:   []logic.MapQueueItem{},
			Schema:    1,
		}, nil
	}

	context, response := newMapQueueTestContext("/maps/queue/snapshot", "")
	GetMapQueueSnapshot(context)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	var snapshot logic.MapQueueSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if !snapshot.Installed || snapshot.Schema != 1 || snapshot.Pending == nil {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestMapQueueControllerActionMappings(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		handler  gin.HandlerFunc
		wantKind logic.MapQueueActionKind
		wantMap  string
	}{
		{name: "add front", path: "/maps/queue/add", body: `{"map":"c1m1_hotel","position":"front"}`, handler: AddMapQueueItem, wantKind: logic.MapQueueActionAddFront, wantMap: "c1m1_hotel"},
		{name: "add back", path: "/maps/queue/add", body: `{"map":"c1m1_hotel","position":"back"}`, handler: AddMapQueueItem, wantKind: logic.MapQueueActionAddBack, wantMap: "c1m1_hotel"},
		{name: "remove", path: "/maps/queue/remove", body: `{"map":"c1m1_hotel"}`, handler: RemoveMapQueueItems, wantKind: logic.MapQueueActionRemove, wantMap: "c1m1_hotel"},
		{name: "start now", path: "/maps/queue/start", body: `{"mode":"now"}`, handler: StartMapQueue, wantKind: logic.MapQueueActionRun},
		{name: "start after campaign", path: "/maps/queue/start", body: `{"mode":"after_campaign"}`, handler: StartMapQueue, wantKind: logic.MapQueueActionRunAfter},
		{name: "pause", path: "/maps/queue/pause", handler: PauseMapQueue, wantKind: logic.MapQueueActionPause},
		{name: "skip", path: "/maps/queue/skip", handler: SkipMapQueueItem, wantKind: logic.MapQueueActionSkip},
		{name: "clear", path: "/maps/queue/clear", handler: ClearMapQueue, wantKind: logic.MapQueueActionClear},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupMapQueueControllerTest(t)
			var gotKind logic.MapQueueActionKind
			var gotMap string
			executeMapQueueAction = func(kind logic.MapQueueActionKind, mapName string) (logic.MapQueueActionResponse, error) {
				gotKind = kind
				gotMap = mapName
				return logic.MapQueueActionResponse{OK: true, Message: "ok"}, nil
			}

			context, response := newMapQueueTestContext(test.path, test.body)
			test.handler(context)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
			}
			if gotKind != test.wantKind || gotMap != test.wantMap {
				t.Fatalf("action = (%q, %q), want (%q, %q)", gotKind, gotMap, test.wantKind, test.wantMap)
			}
		})
	}
}

func TestMapQueueControllerRejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		handler gin.HandlerFunc
	}{
		{name: "position", body: `{"map":"c1m1_hotel","position":"middle"}`, handler: AddMapQueueItem},
		{name: "mode", body: `{"mode":"later"}`, handler: StartMapQueue},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupMapQueueControllerTest(t)
			called := false
			executeMapQueueAction = func(logic.MapQueueActionKind, string) (logic.MapQueueActionResponse, error) {
				called = true
				return logic.MapQueueActionResponse{}, nil
			}
			context, response := newMapQueueTestContext("/maps/queue/action", test.body)
			test.handler(context)
			if response.Code != http.StatusBadRequest || called {
				t.Fatalf("status = %d, called = %v", response.Code, called)
			}
		})
	}
}

func TestMapQueueControllerErrorStatusMapping(t *testing.T) {
	tests := []struct {
		kind logic.MapQueueErrorKind
		want int
	}{
		{kind: logic.MapQueueErrorInvalid, want: http.StatusBadRequest},
		{kind: logic.MapQueueErrorRCON, want: http.StatusServiceUnavailable},
		{kind: logic.MapQueueErrorProtocol, want: http.StatusBadGateway},
		{kind: logic.MapQueueErrorRejected, want: http.StatusConflict},
	}

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			setupMapQueueControllerTest(t)
			getMapQueueSnapshot = func() (logic.MapQueueSnapshot, error) {
				return logic.MapQueueSnapshot{}, &logic.MapQueueError{Kind: test.kind, Message: "test error"}
			}
			context, response := newMapQueueTestContext("/maps/queue/snapshot", "")
			GetMapQueueSnapshot(context)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}

	t.Run("unknown", func(t *testing.T) {
		setupMapQueueControllerTest(t)
		getMapQueueSnapshot = func() (logic.MapQueueSnapshot, error) {
			return logic.MapQueueSnapshot{}, errors.New("unknown")
		}
		context, response := newMapQueueTestContext("/maps/queue/snapshot", "")
		GetMapQueueSnapshot(context)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", response.Code)
		}
	})
}
