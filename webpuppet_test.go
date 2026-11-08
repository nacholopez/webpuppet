package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func stubSleep(t *testing.T) *[]time.Duration {
	t.Helper()
	var calls []time.Duration
	orig := sleep
	sleep = func(d time.Duration) { calls = append(calls, d) }
	t.Cleanup(func() { sleep = orig })
	return &calls
}

func serve(method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, req)
	return rec
}

func TestRandInteger(t *testing.T) {
	valid := []struct{ lo, hi int }{{0, 0}, {5, 5}, {0, 1}, {1, 10}, {0, 1000}}
	for _, tc := range valid {
		for i := 0; i < 200; i++ {
			got, err := RandInteger(tc.lo, tc.hi)
			if err != nil {
				t.Fatalf("RandInteger(%d, %d) unexpected error: %v", tc.lo, tc.hi, err)
			}
			if got < tc.lo || got > tc.hi {
				t.Fatalf("RandInteger(%d, %d) = %d, out of range", tc.lo, tc.hi, got)
			}
		}
	}

	invalid := []struct{ lo, hi int }{{-1, 5}, {-5, -1}, {10, 1}}
	for _, tc := range invalid {
		if _, err := RandInteger(tc.lo, tc.hi); err == nil {
			t.Errorf("RandInteger(%d, %d) expected error", tc.lo, tc.hi)
		}
	}
}

func TestRandIntegerReachesUpperBound(t *testing.T) {
	for i := 0; i < 1000; i++ {
		if got, _ := RandInteger(0, 1); got == 1 {
			return
		}
	}
	t.Fatal("RandInteger(0, 1) never returned the upper bound")
}

func TestRandStringRunes(t *testing.T) {
	for _, n := range []int{0, 1, 64} {
		s := RandStringRunes(n)
		if len(s) != n {
			t.Errorf("RandStringRunes(%d) length = %d", n, len(s))
		}
		for _, r := range s {
			if !strings.ContainsRune(string(letterRunes), r) {
				t.Errorf("RandStringRunes(%d) produced unexpected rune %q", n, r)
			}
		}
	}
}

func TestHealth(t *testing.T) {
	rec := serve(http.MethodGet, "/health", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "OK" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodPost, "/health", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /health got %d", rec.Code)
	}
}

func TestSleep(t *testing.T) {
	tests := []struct {
		path      string
		wantCode  int
		wantSleep time.Duration
	}{
		{"/sleep/0", http.StatusOK, 0},
		{"/sleep/3", http.StatusOK, 3 * time.Second},
		{"/sleep/3600", http.StatusOK, time.Hour},
		{"/sleep/3601", http.StatusBadRequest, -1},
		{"/sleep/-1", http.StatusBadRequest, -1},
		{"/sleep/abc", http.StatusBadRequest, -1},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			calls := stubSleep(t)
			rec := serve(http.MethodGet, tc.path, "")
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tc.wantCode)
			}
			if tc.wantSleep < 0 {
				if len(*calls) != 0 {
					t.Fatalf("slept on invalid input: %v", *calls)
				}
				return
			}
			if len(*calls) != 1 || (*calls)[0] != tc.wantSleep {
				t.Fatalf("sleep calls = %v, want [%v]", *calls, tc.wantSleep)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
		})
	}
}

func TestSleepMsMessageLength(t *testing.T) {
	path := func(maxMs, minMs, maxML, minML string) string {
		return "/sleepms/max/" + maxMs + "/min/" + minMs + "/messagelength/max/" + maxML + "/min/" + minML
	}

	t.Run("fixed values", func(t *testing.T) {
		calls := stubSleep(t)
		rec := serve(http.MethodGet, path("20", "20", "8", "8"), "")
		if rec.Code != http.StatusOK {
			t.Fatalf("code = %d", rec.Code)
		}
		if len(*calls) != 1 || (*calls)[0] != 20*time.Millisecond {
			t.Fatalf("sleep calls = %v", *calls)
		}
		body := rec.Body.String()
		if !strings.HasPrefix(body, `{ "msg": "`) || len(body) != len(`{ "msg": "" }`+"\n")+8 {
			t.Fatalf("unexpected body %q", body)
		}
	})

	t.Run("zero length message", func(t *testing.T) {
		stubSleep(t)
		rec := serve(http.MethodGet, path("0", "0", "0", "0"), "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Slept for 0 milliseconds") {
			t.Fatalf("got %d %q", rec.Code, rec.Body.String())
		}
	})

	invalid := map[string]string{
		"min greater than max": path("10", "20", "1", "1"),
		"negative sleep":       path("10", "-1", "1", "1"),
		"non numeric sleep":    path("x", "1", "1", "1"),
		"sleep over limit":     path("3600001", "0", "1", "1"),
		"length min over max":  path("1", "1", "1", "2"),
		"non numeric length":   path("1", "1", "1", "y"),
		"length over limit":    path("1", "1", "1048577", "0"),
	}
	for name, p := range invalid {
		t.Run(name, func(t *testing.T) {
			calls := stubSleep(t)
			rec := serve(http.MethodGet, p, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400", rec.Code)
			}
			if len(*calls) != 0 {
				t.Fatalf("slept on invalid input: %v", *calls)
			}
		})
	}
}

func TestHTTPResponseCode(t *testing.T) {
	tests := []struct {
		code     string
		wantCode int
	}{
		{"100", 100},
		{"204", 204},
		{"418", 418},
		{"599", 599},
		{"99", http.StatusBadRequest},
		{"600", http.StatusBadRequest},
		{"abc", http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			rec := serve(http.MethodPost, "/httpResponseCode/"+tc.code, "")
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tc.wantCode)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if tc.wantCode == http.StatusBadRequest && rec.Body.String() != "{ \"error\": \"invalid code\" }\n" {
				t.Errorf("body = %q", rec.Body.String())
			}
		})
	}
}

func TestMirror(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mirror", strings.NewReader("<script>alert(1)</script> 100%d"))
	req.Header.Set("X-Test", "value")
	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if v := rec.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", v)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "X-Test: value") {
		t.Errorf("headers not mirrored: %q", body)
	}
	if !strings.Contains(body, "BODY\n====\n<script>alert(1)</script> 100%d\n") {
		t.Errorf("body not mirrored verbatim: %q", body)
	}
}

func TestMirrorWithoutBody(t *testing.T) {
	rec := serve(http.MethodGet, "/mirror", "")
	if strings.Contains(rec.Body.String(), "BODY") {
		t.Errorf("unexpected BODY section: %q", rec.Body.String())
	}
}

func TestPrintRejectsOversizedBody(t *testing.T) {
	for _, p := range []string{"/print/stdout", "/print/stderr", "/mirror"} {
		t.Run(p, func(t *testing.T) {
			rec := serve(http.MethodPost, p, strings.Repeat("a", maxBodyBytes+1))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400", rec.Code)
			}
		})
	}
}

func TestPrintAcceptsBody(t *testing.T) {
	for _, p := range []string{"/print/stdout", "/print/stderr"} {
		t.Run(p, func(t *testing.T) {
			if rec := serve(http.MethodPost, p, "hello"); rec.Code != http.StatusOK {
				t.Fatalf("code = %d", rec.Code)
			}
			if rec := serve(http.MethodGet, p, ""); rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("GET code = %d", rec.Code)
			}
		})
	}
}
