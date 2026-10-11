package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestCreatorRoute(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("Saved token missing")
		}
		var request struct {
			Query     string            `json:"query"`
			Variables map[string]string `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Query != creatorQuery || request.Variables["query"] != "will" {
			t.Errorf("Unexpected bounded search: %+v", request)
		}
		fmt.Fprint(w, `{"data":{"contributors":[{"username":"willtong93"},{"username":"WILLTONG93"},{"username":""},{"username":"bad,name"},{"username":"Willow"}]}}`)
	}))
	defer upstream.Close()
	p := newProvider()
	p.endpoint = upstream.URL
	p.configure(map[string]any{"api_token": "test-token"})
	s := &server{p: p}
	query, _ := structpb.NewStruct(map[string]any{"q": " will "})
	req := &pluginv1.HandleHTTPRequest{Method: "GET", Path: "/creators", Query: query, Headers: map[string]string{"X-Silo-User-Role": "admin"}}
	response, err := s.Handle(context.Background(), req)
	if err != nil || response.StatusCode != 200 {
		t.Fatalf("response=%v err=%v", response, err)
	}
	var results creatorResults
	if err := json.Unmarshal(response.Body, &results); err != nil {
		t.Fatal(err)
	}
	if len(results.Items) != 2 || results.Items[0].Value != "willtong93" || results.HasMore {
		t.Fatalf("Unexpected results: %+v", results)
	}
	for _, tc := range []struct {
		name, role, path, q, method string
		status                      int
	}{
		{"non-admin", "user", "/creators", "will", "GET", 403},
		{"unknown-route", "admin", "/other", "will", "GET", 404},
		{"short-search", "admin", "/creators", "w", "GET", 400},
		{"long-search", "admin", "/creators", strings.Repeat("a", 81), "GET", 400},
		{"write-denied", "admin", "/creators", "will", "POST", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, _ := structpb.NewStruct(map[string]any{"q": tc.q})
			r, e := s.Handle(context.Background(), &pluginv1.HandleHTTPRequest{Method: tc.method, Path: tc.path, Query: q, Headers: map[string]string{"X-Silo-User-Role": tc.role}})
			if e != nil || int(r.StatusCode) != tc.status {
				t.Fatalf("status=%v error=%v", r, e)
			}
		})
	}
	if calls != 1 {
		t.Fatalf("Invalid requests reached upstream: %d", calls)
	}
}

func TestCreatorLookupBoundAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantError bool
	}{
		{"bounded", 200, `{"data":{"contributors":[` + strings.TrimSuffix(strings.Repeat(`{"username":"user"},`, 51), ",") + `]}}`, false},
		{"graphql-error", 200, `{"errors":[{"message":"sensitive upstream error"}]}`, true},
		{"unauthorized", 401, "token rejected", true},
		{"oversized", 200, strings.Repeat("x", (64<<10)+1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			if tc.name == "bounded" {
				var b strings.Builder
				b.WriteString(`{"data":{"contributors":[`)
				for i := 0; i < 51; i++ {
					if i > 0 {
						b.WriteByte(',')
					}
					fmt.Fprintf(&b, `{"username":"user%d"}`, i)
				}
				b.WriteString(`]}}`)
				body = b.String()
			}
			u := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, body) }))
			defer u.Close()
			p := newProvider()
			p.endpoint = u.URL
			p.configure(map[string]any{"api_token": "test-token"})
			r, e := p.creators(context.Background(), "user")
			if (e != nil) != tc.wantError {
				t.Fatalf("error=%v", e)
			}
			if tc.name == "bounded" && (len(r.Items) != 50 || !r.HasMore) {
				t.Fatalf("Unbounded results: %+v", r)
			}
			if e != nil && strings.Contains(e.Error(), "sensitive") {
				t.Fatal("Upstream error leaked")
			}
		})
	}
}
