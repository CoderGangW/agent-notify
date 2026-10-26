package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalOnly(t *testing.T) {
	h := localOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	host := fmt.Sprintf("127.0.0.1:%d", daemonPort)
	cases := []struct {
		name   string
		host   string
		header [2]string
		want   int
	}{
		{"hook client", host, [2]string{}, http.StatusOK},
		{"browser cross-origin POST", host, [2]string{"Origin", "https://evil.example"}, http.StatusForbidden},
		{"browser no-cors GET", host, [2]string{"Sec-Fetch-Site", "cross-site"}, http.StatusForbidden},
		{"DNS rebinding", fmt.Sprintf("evil.example:%d", daemonPort), [2]string{}, http.StatusForbidden},
		{"localhost alias", fmt.Sprintf("localhost:%d", daemonPort), [2]string{}, http.StatusForbidden},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "/event", nil)
		r.Host = c.host
		if c.header[0] != "" {
			r.Header.Set(c.header[0], c.header[1])
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s: got %d, want %d", c.name, w.Code, c.want)
		}
	}
}
