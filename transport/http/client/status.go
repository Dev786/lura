// SPDX-License-Identifier: Apache-2.0
package client

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"encoding/json"
	"net/http"
	"github.com/devopsfaith/krakend/config"
)

// Namespace to be used in extra config
const Namespace = "github.com/devopsfaith/krakend/http"

// ErrInvalidStatusCode is the error returned by the http proxy when the received status code
// is not a 200 nor a 201
var ErrInvalidStatusCode = errors.New("Invalid status code")

// HTTPStatusHandler defines how we tread the http response code
type HTTPStatusHandler func(context.Context, *http.Response) (*http.Response, error)

// GetHTTPStatusHandler returns a status handler. If the 'return_error_details' key is defined
// at the extra config, it returns a DetailedHTTPStatusHandler. Otherwise, it returns a
// DefaultHTTPStatusHandler
func GetHTTPStatusHandler(remote *config.Backend) HTTPStatusHandler {
	if e, ok := remote.ExtraConfig[Namespace]; ok {
		if m, ok := e.(map[string]interface{}); ok {
			if v, ok := m["return_error_details"]; ok {
				if b, ok := v.(string); ok && b != "" {
					return DetailedHTTPStatusHandler(DefaultHTTPStatusHandler, b)
				}
			}
		}
	}
	return DefaultHTTPStatusHandler
}

// DefaultHTTPStatusHandler is the default implementation of HTTPStatusHandler
func DefaultHTTPStatusHandler(ctx context.Context, resp *http.Response) (*http.Response, error) {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, ErrInvalidStatusCode
	}

	return resp, nil
}

// NoOpHTTPStatusHandler is a NO-OP implementation of HTTPStatusHandler
func NoOpHTTPStatusHandler(_ context.Context, resp *http.Response) (*http.Response, error) {
	return resp, nil
}

// Custom Struct Added for our use case
type HttpBody struct{
	StatusCode int `json:"statusCode"`
	Message string `json:"message"`
	IEC string `json:"iec"`
}

type HttpErrorResponse struct {
	HttpBody string `json:"http_body"`
	HttpStatusCode int `json:"http_status_code"`
}

type HttpErrorService struct {
	ErrorService HttpErrorResponse `json:"error_service"`
}

type ServiceError struct {
	Body HttpBody `json:"http_body"`
	HttpStatusCode int `json:"http_status_code"`
}

type ErrorResponse struct{
	ErrorService ServiceError `json:"error_service"`
}

// custom function to parse the http_body
func ModifyServiceErrorResponse(resp *http.Response){
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		body = []byte{}
	}
	defer resp.Body.Close()
	var response map[string]interface{}
	err = json.Unmarshal([]byte(body), &response)
	if _, ok := response["error_service"];ok && err == nil{
		var httpErrorResponse HttpErrorService
		err = json.Unmarshal([]byte(body), &httpErrorResponse)
		var httpBody HttpBody
		err = json.Unmarshal([]byte(httpErrorResponse.ErrorService.HttpBody), &httpBody)
		serviceError := ServiceError{
			httpBody,
			httpErrorResponse.ErrorService.HttpStatusCode,
		}

		body,_ = json.Marshal(ErrorResponse{
			serviceError,
		})
		// format the response here
	}
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))
}

// DetailedHTTPStatusHandler is a HTTPStatusHandler implementation
func DetailedHTTPStatusHandler(next HTTPStatusHandler, name string) HTTPStatusHandler {
	return func(ctx context.Context, resp *http.Response) (*http.Response, error) {
		ModifyServiceErrorResponse(resp);
		if r, err := next(ctx, resp); err == nil {
			return r, nil
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			body = []byte{}
		}
		resp.Body.Close()
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		return resp, HTTPResponseError{
			Code: resp.StatusCode,
			Msg:  string(body),
			name: name,
		}
	}
}

// HTTPResponseError is the error to be returned by the DetailedHTTPStatusHandler
type HTTPResponseError struct {
	Code int    `json:"http_status_code"`
	Msg  string `json:"http_body,omitempty"`
	name string
}

// Error returns the error message
func (r HTTPResponseError) Error() string {
	return r.Msg
}

// Name returns the name of the error
func (r HTTPResponseError) Name() string {
	return r.name
}

// StatusCode returns the status code returned by the backend
func (r HTTPResponseError) StatusCode() int {
	return r.Code
}
