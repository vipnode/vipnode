package jsonrpc2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const httpContentType = "application/json"

var _ http.Handler = &HTTPServer{}

// HTTPServer provides a JSONRPC2 server over HTTP by implementing http.Handler.
type HTTPServer struct {
	Server

	// MaxContentLength is the request size limit (optional)
	MaxContentLength int64
}

func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Convert http.Error(...) output into actual JSONRPC error responses?
	if r.Method == http.MethodGet && r.ContentLength == 0 && r.URL.RawQuery == "" {
		// Ignore empty GET requests
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.MaxContentLength > 0 && r.ContentLength > h.MaxContentLength {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	var body io.Reader = r.Body
	if h.MaxContentLength > 0 {
		body = io.LimitReader(r.Body, h.MaxContentLength)
	}

	codec := &jsonCodec{
		encoder:    json.NewEncoder(w),
		decoder:    json.NewDecoder(body),
		closer:     r.Body,
		remoteAddr: r.RemoteAddr,
	}

	defer codec.Close()
	msg, err := codec.ReadMessage()
	if err != nil {
		err = &ErrResponse{
			Code:    ErrCodeParse,
			Message: fmt.Sprintf("failed to parse request: %s", err),
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", httpContentType)
	resp := h.Server.Handle(r.Context(), msg)
	err = codec.WriteMessage(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var _ Service = &HTTPService{}

type HTTPService struct {
	Client
	HTTPClient http.Client

	// Endpoint is the HTTP URL to dial for RPC calls.
	Endpoint string
	// MaxContentLength is the response size limit (optional)
	MaxContentLength int64
}

func (service *HTTPService) Call(ctx context.Context, result interface{}, method string, params ...interface{}) error {
	msg, err := service.Client.Request(method, params...)
	if err != nil {
		return err
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, service.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", httpContentType)
	req.Header.Set("Accept", httpContentType)
	req = req.WithContext(ctx)

	resp, err := service.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return HTTPRequestError{
			Response: resp,
			Reason:   fmt.Sprintf("bad status code: %d", resp.StatusCode),
		}
	}
	if service.MaxContentLength > 0 && resp.ContentLength > service.MaxContentLength {
		return HTTPRequestError{
			Response: resp,
			Reason:   "response too large",
		}
	}

	var r io.Reader = resp.Body
	if service.MaxContentLength > 0 {
		r = io.LimitReader(resp.Body, service.MaxContentLength)
	}

	var respMsg Message
	if err := json.NewDecoder(r).Decode(&respMsg); err != nil {
		return err
	}
	if respMsg.Response == nil {
		return HTTPRequestError{
			Response: resp,
			Reason:   "missing response in RPC message",
		}
	}
	return respMsg.Response.UnmarshalResult(result)
}

// HTTPRequestError is used when RPC over HTTP encounters an error during transport.
type HTTPRequestError struct {
	Response *http.Response
	Reason   string
}

func (err HTTPRequestError) Error() string {
	return fmt.Sprintf("http rpc request error: %s", err.Reason)
}
