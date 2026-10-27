package control

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recorder struct{ calls int }

func (r *recorder) Execute(context.Context, Command) (Result, error) {
	r.calls++
	return Result{Status: "accepted"}, nil
}
func TestHTTPRejectsUnauthorizedAndInvalidBeforeSerial(t *testing.T) {
	d := &recorder{}
	h := NewHandler("secret", d)
	cases := []struct {
		body, token string
		want        int
	}{
		{`{"action":"enable"}`, "", 401},
		{`{"action":"poses","ids":[1],"positions":[65535]}`, "secret", 400},
		{`{"action":"enable","typo":1}`, "secret", 400},
		{`{"action":"enable"} {"action":"calibrate"}`, "secret", 400},
		{`null`, "secret", 400},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(c.body))
		req.Header.Set("Authorization", "Bearer "+c.token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != c.want {
			t.Fatalf("%s: %d %s", c.body, w.Code, w.Body.String())
		}
	}
	if d.calls != 0 {
		t.Fatal("invalid input reached hardware")
	}
	req := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(`{"action":"enable"}`))
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 || d.calls != 1 {
		t.Fatalf("valid command: %d %s", w.Code, w.Body.String())
	}
}
