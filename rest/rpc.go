package rest

import (
	"errors"
	"golang.org/x/net/context"
	"mime/multipart"
	"net/http"
	"reflect"
	"unicode"
	"unicode/utf8"
)

type rpcType struct {
	methodV reflect.Value
	numIn   int // TODO
	reqType reflect.Type
}

type key int

const contextKey key = 0

var ErrMethodNotFound = errors.New("method not found")
var ErrRpcErr = errors.New("rest rpc err")

const sigPanicMsg = "rest rpc: method signature not match."

var (
	typeOfError              = reflect.TypeOf((*error)(nil)).Elem()
	typeOfContext            = reflect.TypeOf((*context.Context)(nil)).Elem()
	typeOfHttpResponseWriter = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
	typeOfHttpRequestPtr     = reflect.TypeOf((*http.Request)(nil))
	typeOfMultipartForm      = reflect.TypeOf((*multipart.Form)(nil))
)

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// typ.Method(context, w, r, req) (rsp, err)
// typ.Method(context, w, r) (rsp, err)
// typ.Method(context, req) (rsp, err)
// typ.Method(context) (rsp, err)
// req must be exported.
func makeRpc(i interface{}, method string) (rpc *rpcType, err error) {
	methodV := reflect.ValueOf(i).MethodByName(method)
	if !methodV.IsValid() {
		err = ErrMethodNotFound
		return
	}

	methodT := methodV.Type()
	if methodT.NumOut() != 2 || methodT.Out(1) != typeOfError {
		panic(sigPanicMsg)
	}
	numIn := methodT.NumIn()
	rpc = &rpcType{
		methodV: methodV,
		numIn:   numIn,
	}

	switch numIn {
	case 1:
		if methodT.In(0) != typeOfContext {
			panic(sigPanicMsg)
		}
		return
	case 2:
		if methodT.In(0) != typeOfContext ||
			!isExportedOrBuiltinType(methodT.In(1)) {
			panic(sigPanicMsg)
		}
		rpc.reqType = methodT.In(1)
		return
	case 3:
		if methodT.In(0) != typeOfContext ||
			methodT.In(1) != typeOfHttpResponseWriter ||
			methodT.In(2) != typeOfHttpRequestPtr {
			panic(sigPanicMsg)
		}
		return
	case 4:
		if methodT.In(0) != typeOfContext ||
			methodT.In(1) != typeOfHttpResponseWriter ||
			methodT.In(2) != typeOfHttpRequestPtr ||
			!isExportedOrBuiltinType(methodT.In(3)) {
			panic(sigPanicMsg)
		}
		rpc.reqType = methodT.In(3)
		return
	default:
		panic(sigPanicMsg)
	}
}

// Errors are values!
func Err(c context.Context) error {
	errI := c.Value(contextKey)
	if errI != nil {
		return errI.(error)
	} else {
		return nil
	}
}

func newContext(parent context.Context, err error) context.Context {
	return context.WithValue(parent, contextKey, err)
}

func (rpc *rpcType) ServeHTTPWithContext(c context.Context, w http.ResponseWriter, r *http.Request) context.Context {
	var reqV reflect.Value
	if rpc.reqType != nil && rpc.reqType.Kind() != reflect.Invalid {
		reqIsPtr := false
		if rpc.reqType.Kind() == reflect.Ptr {
			reqV = reflect.New(rpc.reqType.Elem())
			reqIsPtr = true
		} else {
			reqV = reflect.New(rpc.reqType)
		}

		err := populateRequest(r, rpc.reqType, reqV.Interface())
		if err != nil {
			return newContext(c, err)
		}

		if !reqIsPtr {
			reqV = reqV.Elem()
		}
	}

	var args []reflect.Value
	switch rpc.numIn {
	case 1:
		args = append(args, reflect.ValueOf(c))
	case 2:
		args = append(args, reflect.ValueOf(c), reqV)
	case 3:
		args = append(args, reflect.ValueOf(c),
			reflect.ValueOf(w), reflect.ValueOf(r))
	case 4:
		args = append(args, reflect.ValueOf(c),
			reflect.ValueOf(w), reflect.ValueOf(r), reqV)
	default:
		panic("not reached.")
	}

	result := rpc.methodV.Call(args)
	rsp := result[0].Interface()
	errI := result[1].Interface()
	var err error
	if errI != nil {
		err = errI.(error)
	}

	if err != nil {
		if h, ok := err.(http.Handler); ok {
			h.ServeHTTP(w, r)
			return c
		} else {
			http.Error(w, "rest: api error.", http.StatusInternalServerError)
			return newContext(c, ErrRpcErr)
		}
	}

	err = respond(w, r, rsp)
	if err != nil {
		http.Error(w, "rest: api error.", http.StatusInternalServerError)
		return newContext(c, ErrRpcErr)
	}

	//
	return newContext(c, err)
}
