package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func testSettingsServer(t *testing.T, save func(context.Context, map[string]any) error) *server {
	t.Helper()
	p := newProvider()
	p.configure(map[string]any{"api_token": "server-only-token", "cache_hours": 24})
	return &server{p: p, savePreferences: save}
}

func settingsRequest(method, path string, headers map[string]string, body string) *pluginv1.HandleHTTPRequest {
	return &pluginv1.HandleHTTPRequest{Method: method, Path: path, Headers: headers, Body: []byte(body)}
}

func adminHeaders() map[string]string {
	return map[string]string{"X-Silo-User-Role": "admin"}
}

func settingsCSRF(t *testing.T, s *server) string {
	t.Helper()
	response, err := s.handleSettings(context.Background(), settingsRequest("GET", "/settings/config", adminHeaders(), ""))
	if err != nil || response.GetStatusCode() != 200 {
		t.Fatalf("config response=%#v err=%v", response, err)
	}
	var out map[string]any
	if err := json.Unmarshal(response.GetBody(), &out); err != nil {
		t.Fatalf("csrf response=%s err=%v", response.GetBody(), err)
	}
	if _, nested := out["config"]; nested {
		t.Fatalf("settings response must be flat: %#v", out)
	}
	if _, leaked := out["api_token"]; leaked || !out["token_configured"].(bool) {
		t.Fatalf("invalid public settings response: %#v", out)
	}
	csrf, _ := out["csrf"].(string)
	if csrf == "" {
		t.Fatalf("missing csrf: %#v", out)
	}
	return csrf
}

func settingsPut(t *testing.T, s *server, body string) (*pluginv1.HandleHTTPResponse, error) {
	t.Helper()
	patch := map[string]any{}
	if err := json.Unmarshal([]byte(body), &patch); err != nil {
		return s.handleSettings(context.Background(), settingsRequest("PUT", "/settings/config", adminHeaders(), body))
	}
	patch["csrf"] = settingsCSRF(t, s)
	raw, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}
	return s.handleSettings(context.Background(), settingsRequest("PUT", "/settings/config", adminHeaders(), string(raw)))
}

func TestSettingsRoutesRequireAdminAndCSRF(t *testing.T) {
	s := testSettingsServer(t, func(context.Context, map[string]any) error { return nil })
	for _, request := range []*pluginv1.HandleHTTPRequest{
		settingsRequest("GET", "/settings", nil, ""),
		settingsRequest("GET", "/settings/config", map[string]string{"X-Silo-User-Role": "user"}, ""),
		settingsRequest("PUT", "/settings/config", map[string]string{"X-Silo-User-Role": "user"}, `{}`),
	} {
		response, err := s.handleSettings(context.Background(), request)
		if err != nil || response.GetStatusCode() != 403 {
			t.Fatalf("unauthorized response=%#v err=%v", response, err)
		}
	}
	response, err := s.handleSettings(context.Background(), settingsRequest("PUT", "/settings/config", adminHeaders(), `{"quality":"original"}`))
	if err != nil || response.GetStatusCode() != 403 {
		t.Fatalf("missing CSRF response=%#v err=%v", response, err)
	}
}

func TestSettingsRejectMalformedAndInvalidPreferences(t *testing.T) {
	s := testSettingsServer(t, func(context.Context, map[string]any) error { return nil })
	for _, body := range []string{
		`{`,
		`{"unknown":true}`,
		`{"api_token":"browser-secret"}`,
		`{"season_consistency":"Anything"}`,
		`{"titlecard_consistency":false}`,
		`{"missing_artwork":"Replace"}`,
		`{"priority_creators":"alice\nbob"}`,
		`{"excluded_creators":12}`,
		`{"cache_hours":1.5}`,
	} {
		response, err := settingsPut(t, s, body)
		if err != nil || response.GetStatusCode() != 400 {
			t.Fatalf("body %s: response=%#v err=%v", body, response, err)
		}
	}
}

func TestSettingsAcceptCreatorPriorityConsistency(t *testing.T) {
	s := testSettingsServer(t, func(context.Context, map[string]any) error { return nil })
	response, err := settingsPut(t, s, `{"season_consistency":"CreatorPriority","titlecard_consistency":"CreatorPriority"}`)
	if err != nil || response.GetStatusCode() != 200 {
		t.Fatalf("CreatorPriority response=%#v err=%v", response, err)
	}
	got := s.p.snapshot()
	if got.SeasonConsistency != "CreatorPriority" || got.TitlecardConsistency != "CreatorPriority" {
		t.Fatalf("settings=%#v", got)
	}
}

func TestSettingsSaveFailureDoesNotPublishChanges(t *testing.T) {
	s := testSettingsServer(t, func(context.Context, map[string]any) error { return errors.New("storage down") })
	before := s.p.snapshot()
	response, err := settingsPut(t, s, `{"priority_creators":"new"}`)
	if err != nil || response.GetStatusCode() != 500 {
		t.Fatalf("save failure response=%#v err=%v", response, err)
	}
	if after := s.p.snapshot(); after != before {
		t.Fatalf("save failure changed live state: before=%#v after=%#v", before, after)
	}
}

func TestSettingsSavePreservesTokenAndUpdatesPublicState(t *testing.T) {
	var persisted map[string]any
	s := testSettingsServer(t, func(_ context.Context, values map[string]any) error {
		persisted = values
		return nil
	})
	response, err := settingsPut(t, s, `{"priority_creators":" Alice,alice,BOB ","cache_hours":12,"quality":"original"}`)
	if err != nil || response.GetStatusCode() != 200 {
		t.Fatalf("save response=%#v err=%v", response, err)
	}
	if got := s.p.snapshot(); got.Token != "server-only-token" || got.Priority != "Alice,BOB" || got.CacheHours != 12 || got.Quality != "original" {
		t.Fatalf("live settings=%#v", got)
	}
	if persisted == nil || persisted["priority_creators"] != "Alice,BOB" || persisted["cache_hours"] != 12 {
		t.Fatalf("persisted=%#v", persisted)
	}
	if _, leaked := persisted["api_token"]; leaked {
		t.Fatalf("persisted token leaked: %#v", persisted)
	}
	if string(response.GetBody()) == "" || strings.Contains(string(response.GetBody()), "server-only-token") {
		t.Fatalf("invalid response body=%q", response.GetBody())
	}
	var out map[string]any
	if err := json.Unmarshal(response.GetBody(), &out); err != nil {
		t.Fatal(err)
	}
	if _, leaked := out["api_token"]; leaked {
		t.Fatalf("response token leaked: %#v", out)
	}
}

func TestConfigureMergesEncryptedPreferencesRegardlessOfEntryOrder(t *testing.T) {
	p := newProvider()
	s := &server{p: p}
	settingsValue, err := structpb.NewStruct(map[string]any{"api_token": "server-only-token", "priority_creators": "old", "season_consistency": "SeriesPoster"})
	if err != nil {
		t.Fatal(err)
	}
	preferencesValue, err := structpb.NewStruct(map[string]any{"priority_creators": "saved", "season_consistency": "MostComplete", "titlecard_consistency": "Independent"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Configure(context.Background(), &pluginv1.ConfigureRequest{Config: []*pluginv1.ConfigEntry{
		{Key: "artwork_preferences", Value: preferencesValue},
		{Key: "settings", Value: settingsValue},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := p.snapshot()
	if got.Token != "server-only-token" || got.Priority != "saved" || got.SeasonConsistency != "MostComplete" || got.TitlecardConsistency != "Independent" {
		t.Fatalf("merged configuration=%#v", got)
	}
}
