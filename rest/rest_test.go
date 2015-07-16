package rest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/caikaijie/igo-middleware/rest"
	"golang.org/x/net/context"
)

func mustReq(method, urlStr string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		panic("")
	}
	return req
}

var (
	getReq           *http.Request
	getQReq          *http.Request
	postFormReq      *http.Request
	putJSONReq       *http.Request
	putJSONReq2      *http.Request
	postMultiPartReq *http.Request
)

type ReqType struct {
	F1 string `json:"f1" schema:"f1"`
	F2 int    `json:"f2" schema:"f2"`
}

func init() {
	urlStr := "x.com/p/"

	req := &ReqType{
		F1: "F1",
		F2: 2,
	}
	form := url.Values{}
	form.Set("f1", "F1")
	form.Set("f2", "2")

	// reqs:
	getReq = mustReq("GET", urlStr, nil)

	getQReq = mustReq("GET", urlStr+"?"+form.Encode(), nil)

	postFormReq = mustReq("POST", urlStr, strings.NewReader(form.Encode()))
	postFormReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	js, _ := json.Marshal(req)
	putJSONReq = mustReq("PUT", urlStr, bytes.NewReader(js))
	putJSONReq.Header.Set("Content-Type", "application/json")

	putJSONReq2 = mustReq("PUT", urlStr, bytes.NewReader(js))
	putJSONReq2.Header.Set("Content-Type", "application/json")

	filename := "rest_test.go"
	mfBodyBuf := &bytes.Buffer{}
	mfBodyW := multipart.NewWriter(mfBodyBuf)
	mfFileW, _ := mfBodyW.CreateFormFile("uploadfile", filename)
	f, _ := os.Open(filename)
	_, _ = io.Copy(mfFileW, f)
	mfFieldW, _ := mfBodyW.CreateFormField("fieldname")
	mfFieldW.Write([]byte("fieldvalue"))
	mfCt := mfBodyW.FormDataContentType()
	mfBodyW.Close()
	postMultiPartReq = mustReq("POST", urlStr, mfBodyBuf)
	postMultiPartReq.Header.Set("Content-Type", mfCt)

}

func testRPC(desc string, resource *rest.Resource, r *http.Request) {
	w := httptest.NewRecorder()
	c := resource.ServeHTTPWithContext(context.Background(), w, r)
	fmt.Printf("===RPC Record Begin===\nDesc:%s\nCode:%d\nContent-Type:%s\nBody:%s\nErr:%v\n===RPC Record End===\n\n", desc, w.Code, w.Header().Get("Content-Type"), w.Body.String(), rest.Err(c))
}

type getOnly struct {
	t *testing.T
}

func (g *getOnly) Get(context.Context) (bool, error) {
	// fmt.Println("Get method successfully called.")
	return true, nil
}

func TestResourceMethod(t *testing.T) {
	r := rest.New(new(getOnly))
	testRPC("Get Only.", r, getReq)
	testRPC("Post Not Support.", r, postFormReq)
}

type t1 struct {
	t *testing.T
}

func (*t1) Get(_ context.Context, req *ReqType) (*ReqType, error) {
	return req, nil
}

func (*t1) Post(_ context.Context, w http.ResponseWriter, r *http.Request, req *ReqType) (*ReqType, error) {
	return req, nil
}

func (*t1) Put(_ context.Context, req *ReqType) (*ReqType, error) {
	return req, nil
}

type t2 struct {
	t *testing.T
}

func (t2_ *t2) Post(_ context.Context, mf *multipart.Form) (bool, error) {
	if mf.Value["fieldname"][0] != "fieldvalue" {
		t2_.t.Fatal("MultipartForm filename not match.")
	}
	filename := "rest_test.go"
	f1, _ := os.Open(filename)
	bs1, _ := ioutil.ReadAll(f1)
	f2, _ := mf.File["uploadfile"][0].Open()
	bs2, _ := ioutil.ReadAll(f2)
	if !bytes.Equal(bs1, bs2) {
		t2_.t.Fatal("MultipartForm uploadfile not match")
	}

	//fmt.Println("[debug]\n", string(bs1), mf.File["uploadfile"][0].Header)

	return true, nil
}

// no pointer
func (*t2) Put(_ context.Context, req ReqType) (ReqType, error) {
	return req, nil
}

func TestRpc(t *testing.T) {
	r := rest.New(new(t1))
	testRPC("Get Query.", r, getQReq)
	testRPC("Post Form.", r, postFormReq)
	testRPC("Put JSON.", r, putJSONReq)

	r = rest.New(new(t2))
	testRPC("Put JSON 2.", r, putJSONReq2)
	testRPC("Post MultipartForm.", r, postMultiPartReq)
}
