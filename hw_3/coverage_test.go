package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Тестовые сущности
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	t.Cleanup(ts.Close)
	return ts
}

func newTestClient(server *httptest.Server) *SearchClient {
	return &SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}
}

func TestSearchWithoutClient(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet,
		"/?limit=20&offset=0&order_field=Id&order_by=1", nil)
	req.Header.Set("AccessToken", accessToken)

	w := httptest.NewRecorder()
	SearchServer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Got %d, body %s, but wanted 200", w.Code, w.Body.String())
	}

	var users []User
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(users) != 20 {
		t.Fatalf("want 20 users, got %d", len(users))
	}

	if users[0].ID >= users[1].ID {
		t.Fatalf("not sorted asc by Id: %d >= %d", users[0].ID, users[1].ID)
	}
}

func TestFindUsersBasic(t *testing.T) {
	ts := newTestServer(t)
	tc := newTestClient(ts)

	resp, err := tc.FindUsers(SearchRequest{
		Limit:      20,
		Offset:     0,
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Users) != 20 {
		t.Fatalf("want 3 users, got %d", len(resp.Users))
	}

	for i := 1; i < len(resp.Users); i++ {
		if resp.Users[i-1].ID > resp.Users[i].ID {
			t.Fatalf("not sorted asc: %q > %q",
				resp.Users[i-1].Name, resp.Users[i].Name)
		}
	}

	if !resp.NextPage {
		t.Fatalf("expected NextPage=true (dataset has more than 3 rows)")
	}
}

func TestFindUsersWithPagination(t *testing.T) {
	ts := newTestServer(t)
	tc := newTestClient(ts)

	req := SearchRequest{
		Limit:      3,
		Offset:     0,
		OrderField: "Id",
	}

	req.Offset = 0
	page1, err := tc.FindUsers(req)
	if err != nil {
		t.Fatal(err)
	}

	req.Offset = 2
	page2, err := tc.FindUsers(req)
	if err != nil {
		t.Fatal(err)
	}

	if len(page1.Users) != 3 || len(page2.Users) != 3 {
		t.Fatalf("want 3+3 users, got %d+%d", len(page1.Users), len(page2.Users))
	}

	if page2.Users[0].ID != page1.Users[1].ID+1 {
		t.Fatalf("pages overlap: p1 last=%d, p2 first=%d",
			page1.Users[1].ID, page2.Users[0].ID)
	}
}

func TestFindUsersWithWrongOrderField(t *testing.T) {
	ts := newTestServer(t)
	tc := newTestClient(ts)
	req := SearchRequest{
		Limit:      3,
		Offset:     0,
		OrderField: "Opagamnastile",
		OrderBy:    OrderByAsc,
	}

	_, err := tc.FindUsers(req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindUsersWithWrongToken(t *testing.T) {
	ts := newTestServer(t)
	sc := &SearchClient{
		AccessToken: "password",
		URL:         ts.URL,
	}

	_, err := sc.FindUsers(SearchRequest{
		Limit:      20,
		Offset:     0,
		OrderField: "Id",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bad AccessToken") {
		t.Fatalf("unexpected error: %v", err)
	}

	sc = &SearchClient{
		AccessToken: "",
		URL:         ts.URL,
	}

	_, err = sc.FindUsers(SearchRequest{
		Limit:      20,
		Offset:     0,
		OrderField: "Id",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bad AccessToken") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindUsersWithDescOrder(t *testing.T) {
	ts := newTestServer(t)
	tc := newTestClient(ts)

	resp, err := tc.FindUsers(SearchRequest{
		Limit:      20,
		Offset:     0,
		OrderField: "Id",
		OrderBy:    OrderByDesc,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Users) != 20 {
		t.Fatalf("want 3 users, got %d", len(resp.Users))
	}

	for i := 1; i < len(resp.Users); i++ {
		if resp.Users[i-1].ID < resp.Users[i].ID {
			t.Fatalf("not sorted desc: %q < %q",
				resp.Users[i-1].Name, resp.Users[i].Name)
		}
	}

	if !resp.NextPage {
		t.Fatalf("expected NextPage=true (dataset has more than 3 rows)")
	}
}

func TestFindUsersServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal error", http.StatusInternalServerError)
		},
	))
	defer ts.Close()

	client := newTestClient(ts)
	_, err := client.FindUsers(SearchRequest{Limit: 10})

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindUsersInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{"))
		},
	))
	defer ts.Close()

	client := newTestClient(ts)
	_, err := client.FindUsers(SearchRequest{Limit: 10})

	if err == nil {
		t.Fatal("expected JSON error")
	}
}

func TestSearchServerInvalidLimit(t *testing.T) {
	for _, limit := range []string{"abc", "-1"} {
		t.Run(limit, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"/?limit="+limit+"&offset=0&order_field=Id&order_by=1",
				nil,
			)
			req.Header.Set("AccessToken", accessToken)

			w := httptest.NewRecorder()
			SearchServer(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400", w.Code)
			}
		})
	}
}
func TestSearchServerInvalidOffset(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/?limit=10&offset=abc&order_field=Id&order_by=1",
		nil,
	)
	req.Header.Set("AccessToken", accessToken)

	w := httptest.NewRecorder()
	SearchServer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", w.Code)
	}
}

func TestSearchServerMissingToken(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/?limit=10&offset=0&order_field=Id&order_by=1",
		nil,
	)

	w := httptest.NewRecorder()
	SearchServer(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", w.Code)
	}
}

func TestSearchServerInvalidOrderBy(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/?limit=10&offset=0&order_field=Id&order_by=999",
		nil,
	)
	req.Header.Set("AccessToken", accessToken)

	w := httptest.NewRecorder()
	SearchServer(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", w.Code)
	}
}
