package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"l4d2-manager-next/logic"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupGameBanControllerStubs(t *testing.T) {
	t.Helper()
	oldList := listGameBans
	oldAdd := addGameBan
	oldRemove := removeGameBan
	t.Cleanup(func() {
		listGameBans = oldList
		addGameBan = oldAdd
		removeGameBan = oldRemove
	})
}

func TestGameBanEndpointsRequireAdministrator(t *testing.T) {
	setupAccessControlControllerTest(t)
	setupGameBanControllerStubs(t)
	called := false
	listGameBans = func() (logic.GameBanList, error) {
		called = true
		return logic.GameBanList{}, nil
	}

	router := accessControlControllerRouter("guest")
	request := httptest.NewRequest(http.MethodPost, "/access-control/game-bans/list", nil)
	request.RemoteAddr = "198.51.100.20:45000"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	if called {
		t.Fatal("game ban logic was called for guest")
	}
}

func TestAddGameBanNormalizesSteamIDAndReturnsList(t *testing.T) {
	setupAccessControlControllerTest(t)
	setupGameBanControllerStubs(t)
	var captured logic.GameBanChange
	addGameBan = func(change logic.GameBanChange) (logic.GameBanList, error) {
		captured = change
		return logic.GameBanList{
			SteamBans: []logic.GameBanEntry{{Kind: logic.GameBanKindSteamID, Value: change.Value, Permanent: true}},
			IPBans:    []logic.GameBanEntry{},
			Warnings:  []string{},
		}, nil
	}

	body := []byte(`{"kind":"steam_id","value":"[U:1:247]","duration_minutes":0}`)
	request := httptest.NewRequest(http.MethodPost, "/access-control/game-bans/add", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "198.51.100.20:45000"
	recorder := httptest.NewRecorder()
	accessControlControllerRouter("admin").ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if captured.Value != "STEAM_1:1:123" || captured.Kind != logic.GameBanKindSteamID || captured.DurationMinutes != 0 {
		t.Fatalf("captured change = %#v", captured)
	}
}

func TestGameBanControllerMapsTypedErrors(t *testing.T) {
	setupAccessControlControllerTest(t)
	setupGameBanControllerStubs(t)
	listGameBans = func() (logic.GameBanList, error) {
		return logic.GameBanList{}, errors.Join(logic.ErrGameBanRCONUnavailable, errors.New("dial failed"))
	}

	request := httptest.NewRequest(http.MethodPost, "/access-control/game-bans/list", nil)
	request.RemoteAddr = "198.51.100.20:45000"
	recorder := httptest.NewRecorder()
	accessControlControllerRouter("admin").ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["error"] != "rcon_unavailable" {
		t.Fatalf("error code = %#v", response["error"])
	}
}
