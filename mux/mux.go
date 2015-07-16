package mux

import (
	"net/http"
	"strings"

	"github.com/caikaijie/igo/httpcontext"
	"golang.org/x/net/context"
)

type key int

const contextKey key = 0

var _ httpcontext.ContextHandler = New("", nil)

type Mux struct {
	prefix string
	m      map[string]http.Handler
	// inited bool
	root     *node
	notFound http.Handler
	// sync.Mutex?
}

func New(prefix string, notFound http.Handler) *Mux {
	if notFound == nil {
		notFound = http.HandlerFunc(http.NotFound)
	}

	if len(prefix) == 0 {
		prefix = "/"
	}
	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	if prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}
	// "/api/", "/"

	return &Mux{
		prefix:   prefix,
		m:        make(map[string]http.Handler),
		notFound: notFound,
	}
}

func (mux *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, _ := mux.match(mux.root, r.URL.Path)
	if h == nil {
		h = mux.notFound
	}

	h.ServeHTTP(w, r)
}

func Err(c context.Context) error {
	// no error
	return nil
}

func FromContext(c context.Context) (map[string]string, bool) {
	captures, ok := c.Value(contextKey).(map[string]string)
	return captures, ok
}

func newContext(parent context.Context, captures map[string]string) context.Context {
	return context.WithValue(parent, contextKey, captures)
}

func (mux *Mux) ServeHTTPWithContext(parent context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	_, captures := mux.match(mux.root, r.URL.Path)
	return newContext(parent, captures)
}

func (mux *Mux) Handle(pattern string, h http.Handler) {
	if mux.root != nil {
		panic("mux: already inited.")
	}

	if pattern[0] == '/' {
		pattern = pattern[1:]
	}
	if pattern != "" && pattern[len(pattern)-1] != '/' {
		pattern = pattern + "/"
	}
	// "users/", "users/10001/", ""(for root)

	if _, ok := mux.m[pattern]; ok {
		panic("mux: pattern existed: " + pattern)
	}

	mux.m[pattern] = h
}

func (mux *Mux) Init() {
	if mux.root != nil {
		return
	}

	m := make(map[string][]string)
	for pattern, _ := range mux.m {
		names := strings.Split(pattern, "/")
		for _, name := range names {
			if name == "" {
				continue
			}
			m[pattern] = append(m[pattern], name+"/")
		}
	}

	root := &node{
		name: mux.prefix,
		h:    mux.m[""],
	}

	makeNode := func(cur *node, pattern, name string) *node {
		for _, child := range cur.children {
			if child.name == name {
				return child
			} else if child.name[0] == ':' && name[0] == ':' {
				panic("mux: pattern ambiguous: " + pattern + ", " + child.name)
			}
		}

		// println("new node: " + name)
		child := &node{name: name}
		cur.children = append(cur.children, child)

		return child
	}

	for pattern, names := range m {
		cur := root
		for _, name := range names {
			child := makeNode(cur, pattern, name)
			cur = child
		}
		cur.h = mux.m[pattern]
	}

	// root Handler?
	// for k, v := range m {
	// 	if k == "" {
	// 		root.h = v
	// 		delete(m, k)
	// 	}
	// }

	mux.root = root
}

type node struct {
	name     string // must end with '/'
	h        http.Handler
	children []*node
}

// for debug
func (mux *Mux) Print() {
	printNode(mux.root, "")
}

func printNode(n *node, indent string) {
	name := n.name
	if n.h != nil {
		name += "(h)"
	}
	println(indent + name)
	for _, child := range n.children {
		printNode(child, indent+"\t")
	}
}

func capture(n *node, path string) (h http.Handler, sub, ck, cv string) {
	// println("[debug]capturing or matching: " + n.name + " -> " + path)
	if n.name[0] == ':' {
		ck = n.name[1 : len(n.name)-1]
		slashIdx := strings.Index(path, "/")
		if slashIdx == -1 {
			return
		}
		cv = path[:slashIdx]
		sub = path[slashIdx+1:]

		if sub == "" {
			h = n.h
			// println("[debug]capture one name[" + ck + ", " + cv + "] successfully, done")
			return
		}
		// println("[debug]capture one name[" + ck + ", " + cv + "] successfully, now sub: " + sub)
		return
	} else if l := len(n.name); l <= len(path) && n.name == path[:l] {
		if l == len(path) {
			h = n.h
			// println("[debug]match one name[" + n.name + "] successfully, done." )
			return
		}
		sub = path[l:]
		if sub == "" {
			h = n.h
			// println("[debug]match one name[" + n.name + "] successfully, done." )
			return
		}
		// println("[debug]match one name[" + n.name + "] successfully, now sub: " + sub )
	}

	// println("[debug]not match one name[" + n.name + "], now will try next.")

	return
}

func (mux *Mux) match(root *node, path string) (h http.Handler, captures map[string]string) {
	if len(path) == 0 || len(path) < len(mux.prefix) {
		return
	}

	lp := len(mux.prefix)
	if mux.prefix == path[:lp] {
		if len(path) == lp {
			h = root.h
			return
		} else {
			path = path[lp:]
		}
	} else {
		return
	}

	// println("[debug]start match pattern: " + path)

	ns := root.children
	p := path
	captures_ := make(map[string]string)

	for {
		if len(ns) == 0 {
			return
		}

		for _, cur := range ns {
			h_, sub, ck, cv := capture(cur, p)

			if ck != "" {
				captures_[ck] = cv
			}

			if h_ != nil {
				h = h_
				captures = captures_
				return
			}

			if sub == "" {
				continue
			}
			p = sub

			ns = cur.children
			break
		}

		if len(p) == 0 {
			return
		}
	}

	// TODO: more info?
	panic("mux: never reached.")
}
