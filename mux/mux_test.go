package mux_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/caikaijie/igo-middleware/mux"
	"github.com/caikaijie/igo/httpcontext"
	"golang.org/x/net/context"
)

/// TestCapture
///////////////
func sameMap(l, r map[string]string) bool {
	if len(l) != len(r) {
		return false
	}

	for k, v := range l {
		if r[k] != v {
			return false
		}
	}

	return true
}

var captureMap = map[[2]string]map[string]string{
	[2]string{"/", "/api/"}: map[string]string{},

	[2]string{"users/", "/api/users/"}: map[string]string{},

	[2]string{"users/:user-id", "/api/users/user123/"}: map[string]string{
		"user-id": "user123",
	},

	[2]string{"users/:user-id/feeds/", "/api/users/user123/feeds/"}: map[string]string{
		"user-id": "user123",
	},

	[2]string{"users/:user-id/feeds/:feed-id", "/api/users/user123/feeds/feed123/"}: map[string]string{
		"user-id": "user123",
		"feed-id": "feed123",
	},

	[2]string{"feeds/", "/api/feeds/"}: map[string]string{},

	[2]string{"timelines/", "/api/timelines/"}: map[string]string{},

	[2]string{"users/:user-id/profile/", "/api/users/user123/profile/"}: map[string]string{
		"user-id": "user123",
	},
}

type captureHanlder struct {
	captures map[string]string
	t        *testing.T
}

func (h *captureHanlder) ServeHTTPWithContext(c context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	captures, ok := mux.FromContext(c)
	fmt.Println("ServeHTTPWithContext: ", captures, h.captures, ok)
	if !ok {
		h.t.Error("mux.FromContext(c), url: " + r.URL.Path)
		return c
	}
	if !sameMap(captures, h.captures) {
		h.t.Error("captures not match, url: " + r.URL.Path)
		return c
	} else {
		fmt.Println("matched. url: " + r.URL.Path)
	}

	return c
}

func mustCaptureHanlder(t *testing.T, m *mux.Mux, captures map[string]string) http.Handler {
	var h httpcontext.ContextHandler = &captureHanlder{
		t:        t,
		captures: captures,
	}
	return httpcontext.MakeHandler(context.Background(), m, h)
}

func TestCapture(t *testing.T) {
	m := mux.New("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("not found, url: " + r.URL.Path)
	}))

	for k, v := range captureMap {
		m.Handle(k[0], mustCaptureHanlder(t, m, v))
	}

	m.Init()
	m.Print()

	for k, _ := range captureMap {
		fakeReq, _ := http.NewRequest("whatever", k[1], nil)
		m.ServeHTTP(nil, fakeReq)
		fmt.Println("===")
	}
}

func BenchmarkCapture(b *testing.B) {
	m := mux.New("/api/", nil)

	for k, _ := range captureMap {
		m.Handle(k[0], httpcontext.MakeHandler(context.Background(), m))
	}

	m.Init()
	b.ResetTimer()

	var reqs []*http.Request
	for k, _ := range captureMap {
		fakeReq, _ := http.NewRequest("whatever", k[1], nil)
		reqs = append(reqs, fakeReq)
	}

	i := 0
	for {
		for _, req := range reqs {
			m.ServeHTTP(nil, req)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

func BenchmarkStd(b *testing.B) {
	m := http.NewServeMux()

	for k, _ := range captureMap {
		m.Handle(k[0], http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	}

	b.ResetTimer()

	var reqs []*http.Request
	for k, _ := range captureMap {
		fakeReq, _ := http.NewRequest("whatever", k[1], nil)
		reqs = append(reqs, fakeReq)
	}

	i := 0
	for {
		for _, req := range reqs {
			m.ServeHTTP(nil, req)
			i++
			if i >= b.N {
				return
			}
		}
	}
}

/// TestAmbiguous
/////////////////
var ambiguousMap = map[string]struct{}{
	"users/:user-id/":  struct{}{},
	"users/:user-id2/": struct{}{},
}

func TestAmbiguous(t *testing.T) {
	defer func() {
		r := recover()
		if strings.Index(r.(string), "mux: pattern ambiguous") == 0 {
		} else {
			t.Error("")
		}
	}()
	m := mux.New("/whatever/", nil)
	for k, _ := range ambiguousMap {
		m.Handle(k, nil)
	}
	m.Init()
}

/// TestNotFound
// var notFoundMap = map[string]string {
// 	"/api/users/": "/badpath/",
// }
