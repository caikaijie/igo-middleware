package rest

import (
	"encoding/json"
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"reflect"

	"github.com/gorilla/schema"
)

var (
	ErrContentType       = errors.New("Content-Type not supported")
	ErrMultipartMismatch = errors.New("multipart/form-data rpc mismatch")

	formDecoder = schema.NewDecoder()
)

func populateRequestForm(r *http.Request, req interface{}) error {
	// force to parse form
	r.FormValue("")
	return formDecoder.Decode(req, r.Form)
}

func populateRequestMultiForm(r *http.Request, req interface{}) error {
	// force to parse form
	r.FormValue("")

	// seems duplicated. rpc just uses r.MultipartForm instead?
	mf := req.(*multipart.Form)
	mf.Value = r.MultipartForm.Value
	mf.File = r.MultipartForm.File
	return nil
}

func populateRequestJSON(r *http.Request, req interface{}) error {
	return json.NewDecoder(r.Body).Decode(req)
}

// currently, only support:
// "Form"				(GET/DELETE + url query, POST/PUT + application/x-www-form-urlencoded)
// "MultiForm"	(POST/PUT + multipart/form-data)
// "JSON"				(POST/PUT + application/json)
func populateRequest(r *http.Request, reqT reflect.Type, req interface{}) error {
	if reqT == nil {
		return nil
	}
	method := r.Method

	// DELETE?
	if method == "GET" || method == "DELETE" {
		return populateRequestForm(r, req)
	}

	ct := r.Header.Get("Content-Type")
	if ct == "" {
		// TODO: guess content?
		return ErrContentType
	}
	ct, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return err
	}

	if ct == "application/json" {
		return populateRequestJSON(r, req)
	} else if ct == "multipart/form-data" {
		if reqT != typeOfMultipartForm {
			return ErrMultipartMismatch
		}
		return populateRequestMultiForm(r, req)
	} else {
		return populateRequestForm(r, req)
	}

	// never reached.
	return nil
}

// only json response.
func respond(w http.ResponseWriter, r *http.Request, rsp interface{}) error {
	// TODO: lots of work
	// gzip, pretty, jsonp...
	data, err := json.Marshal(rsp)
	if err != nil {
		return err
	}
	
	w.WriteHeader(200)
	w.Write(data)
	w.Header().Set("Content-Type", "application/json")

	return nil
}
