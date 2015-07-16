package rest

import (
	"errors"
	"github.com/caikaijie/igo/httpcontext"
	"golang.org/x/net/context"
	"net/http"
)

var _ httpcontext.ContextHandler = New(struct{}{})
var ErrMethodNotAllowed = errors.New("method not found")

type Resource struct {
	geth    *rpcType
	posth   *rpcType
	puth    *rpcType
	deleteh *rpcType
}

func mustMakeRpc(i interface{}, method string) *rpcType {
	rpc, err := makeRpc(i, method)
	if err != nil && err != ErrMethodNotFound {
		panic("rest: makeRpc error")
	}
	return rpc
}

// go method - http method
// Get - GET
// Put - PUT
// Post - POST
// Delete - DELETE
func New(i interface{}) *Resource {
	r := &Resource{}
	r.geth = mustMakeRpc(i, "Get")
	r.posth = mustMakeRpc(i, "Post")
	r.puth = mustMakeRpc(i, "Put")
	r.deleteh = mustMakeRpc(i, "Delete")

	// println("[debug]", r.geth, r.posth, r.puth, r.deleteh)
	return r
}

func (resource *Resource) ServeHTTPWithContext(parent context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	var h *rpcType
	switch r.Method {
	case "GET":
		h = resource.geth
	case "POST":
		h = resource.posth
	case "PUT":
		h = resource.puth
	case "DELETE":
		h = resource.deleteh
	default:
	}

	// println("[debug]", r.Method, h)
	if h != nil {
		return h.ServeHTTPWithContext(parent, w, r)
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	// TODO: set error or cancel context?
	return newContext(parent, ErrMethodNotAllowed)
}
