package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// movieSetsResponse is a plain movie set, as movieQuery returns it.
const movieSetsResponse = `{"data":{"movies_by_id":{"id":"429617","movie_sets":[{"id":"23725","popularity":980,"user_created":{"username":"CmdrRiker"},"movie_poster":[{"id":"riker-poster","modified_on":"2024-01-02T00:00:00.000Z","language":{"display_name":"English"}}],"movie_backdrop":[],"files":[]}]}}}`

// collectionSetsResponse is the same movie reached through its TMDB
// collection, where the poster hangs off the collection's member movie.
const collectionSetsResponse = `{"data":{"movies_by_id":{"id":"429617","collection_id":{"movies":[
 {"id":"429617","posters":[{"id":"willtong-poster","modified_on":"2024-02-03T00:00:00.000Z","language":{"display_name":"English"},"collection_set":{"id":"24304","popularity":12,"user_created":{"username":"willtong93"}}}],"backdrops":[{"id":"willtong-backdrop","modified_on":"2024-02-03T00:00:00.000Z","language":{"display_name":"English"},"collection_set":{"id":"24304","popularity":12,"user_created":{"username":"willtong93"}}}]},
 {"id":"315635","posters":[{"id":"other-movie-poster","modified_on":"2024-02-03T00:00:00.000Z","language":{"display_name":"English"},"collection_set":{"id":"24304","popularity":12,"user_created":{"username":"willtong93"}}}],"backdrops":[]}
]}}}}`

// mediuxStub answers the movie-set and collection-set queries separately so a
// test can assert which branch the provider asked for.
func mediuxStub(t *testing.T, movieSets, collectionSets string, collectionCalls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(q.Query, "collection_id") {
			if collectionCalls != nil {
				collectionCalls.Add(1)
			}
			_, _ = w.Write([]byte(collectionSets))
			return
		}
		_, _ = w.Write([]byte(movieSets))
	}))
}

func testProvider(t *testing.T, endpoint string, cfg map[string]any) *provider {
	t.Helper()
	p := newProvider()
	p.endpoint = endpoint
	p.client = &http.Client{Timeout: time.Second}
	p.configure(cfg)
	return p
}

// A movie whose only prioritized artwork lives in a collection set must win
// over a far more popular movie set by an unlisted creator. This is the
// reported Spider-Man: Far From Home case.
func TestCollectionSetBeatsPopularMovieSetOnCreatorPriority(t *testing.T) {
	srv := mediuxStub(t, movieSetsResponse, collectionSetsResponse, nil)
	defer srv.Close()
	p := testProvider(t, srv.URL, map[string]any{"api_token": "safe", "priority_creators": "willtong93"})

	b, err := p.get(context.Background(), "movie", "429617")
	if err != nil {
		t.Fatal(err)
	}
	got := p.automaticArt(b, p.snapshot(), "poster", 0, 0, "en")
	if len(got) != 1 {
		t.Fatalf("posters=%d", len(got))
	}
	if got[0].ID != "willtong-poster" {
		t.Fatalf("poster=%s, want the prioritized collection set", got[0].ID)
	}
	if got[0].Creator != "willtong93" {
		t.Fatalf("creator=%s", got[0].Creator)
	}
}

// Without a priority list the popular movie set still wins, so adding the
// collection branch does not quietly re-rank existing installs.
func TestCollectionSetDoesNotOutrankPopularityByDefault(t *testing.T) {
	srv := mediuxStub(t, movieSetsResponse, collectionSetsResponse, nil)
	defer srv.Close()
	p := testProvider(t, srv.URL, map[string]any{"api_token": "safe"})

	b, err := p.get(context.Background(), "movie", "429617")
	if err != nil {
		t.Fatal(err)
	}
	got := p.automaticArt(b, p.snapshot(), "poster", 0, 0, "en")
	if len(got) == 0 || got[0].ID != "riker-poster" {
		t.Fatalf("poster=%v, want the more popular movie set", got)
	}
}

// Only the requested movie's artwork may enter the bundle; the other members
// of the collection resolve through their own lookups.
func TestCollectionSetsIgnoreOtherCollectionMembers(t *testing.T) {
	b, err := parseCollectionBundle(json.RawMessage(mustData(t, collectionSetsResponse)), "429617")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Sets) != 1 {
		t.Fatalf("sets=%d, want the two images grouped into one set", len(b.Sets))
	}
	if b.Sets[0].ID != "24304" || b.Sets[0].Creator != "willtong93" || b.Sets[0].Popularity != 12 {
		t.Fatalf("set=%+v", b.Sets[0])
	}
	if len(b.Sets[0].Art) != 2 {
		t.Fatalf("art=%d", len(b.Sets[0].Art))
	}
	for _, a := range b.Sets[0].Art {
		if a.ID == "other-movie-poster" {
			t.Fatal("artwork from another collection member leaked in")
		}
		if a.Language != "en" {
			t.Fatalf("language=%s", a.Language)
		}
	}
}

// A movie outside any collection yields no extra sets and no error.
func TestMovieWithoutCollectionIsNotAnError(t *testing.T) {
	const noCollection = `{"data":{"movies_by_id":{"id":"429617","collection_id":null}}}`
	srv := mediuxStub(t, movieSetsResponse, noCollection, nil)
	defer srv.Close()
	p := testProvider(t, srv.URL, map[string]any{"api_token": "safe"})

	b, err := p.get(context.Background(), "movie", "429617")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Sets) != 1 || b.Sets[0].ID != "23725" {
		t.Fatalf("sets=%+v", b.Sets)
	}
}

// Collection sets are additive, so losing them must not cost the caller the
// movie sets that already resolved.
func TestCollectionFailureKeepsMovieSets(t *testing.T) {
	srv := mediuxStub(t, movieSetsResponse, `{"errors":[{"message":"boom"}]}`, nil)
	defer srv.Close()
	p := testProvider(t, srv.URL, map[string]any{"api_token": "safe"})

	b, err := p.get(context.Background(), "movie", "429617")
	if err != nil {
		t.Fatalf("collection failure broke the lookup: %v", err)
	}
	if len(b.Sets) != 1 || b.Sets[0].ID != "23725" {
		t.Fatalf("sets=%+v", b.Sets)
	}
}

// A mismatched identity is rejected rather than attributed to the wrong movie.
func TestCollectionIdentityMismatchRejected(t *testing.T) {
	if _, err := parseCollectionBundle(json.RawMessage(mustData(t, collectionSetsResponse)), "999"); err == nil {
		t.Fatal("identity mismatch accepted")
	}
}

// Series have no collections, so the series path must not spend a request on
// the collection branch.
func TestSeriesLookupSkipsCollectionQuery(t *testing.T) {
	const shows = `{"data":{"shows_by_id":{"id":"12","show_sets":[]}}}`
	var collectionCalls atomic.Int32
	srv := mediuxStub(t, shows, collectionSetsResponse, &collectionCalls)
	defer srv.Close()
	p := testProvider(t, srv.URL, map[string]any{"api_token": "safe"})

	if _, err := p.get(context.Background(), "series", "12"); err != nil {
		t.Fatal(err)
	}
	if collectionCalls.Load() != 0 {
		t.Fatalf("collection queries=%d", collectionCalls.Load())
	}
}

// mustData unwraps the GraphQL envelope the way fetch does before parsing.
func mustData(t *testing.T, body string) []byte {
	t.Helper()
	var g struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &g); err != nil {
		t.Fatal(err)
	}
	return g.Data
}
