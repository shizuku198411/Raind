package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const MaxJSONBodyBytes int64 = 1 << 20

var ErrRequestBodyTooLarge = errors.New("request body too large")

type ApiResponse struct {
	Status  string `json:"status"` // success | fail
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

type StreamEvent struct {
	Status  string `json:"status"`
	ID      string `json:"id,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Current int64  `json:"current,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Error   string `json:"error,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func DecodeRequestBody(r *http.Request, v any) error {
	body, err := readLimitedBody(r.Body, MaxJSONBodyBytes)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("invalid json: multiple JSON values")
	}
	return nil
}

func DecodeErrorStatus(err error) int {
	if errors.Is(err, ErrRequestBodyTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func readLimitedBody(r io.Reader, limit int64) ([]byte, error) {
	lr := &io.LimitedReader{R: r, N: limit + 1}
	body, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: max=%d bytes", ErrRequestBodyTooLarge, limit)
	}
	return body, nil
}

func WriteJson(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

func RespondSuccess(w http.ResponseWriter, statusCode int, message string, data any) {
	WriteJson(w, statusCode, ApiResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

func RespondFail(w http.ResponseWriter, statusCode int, message string, data any) {
	WriteJson(w, statusCode, ApiResponse{
		Status:  "fail",
		Message: message,
		Data:    data,
	})
}

func StreamJson(w http.ResponseWriter, event StreamEvent) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	_ = json.NewEncoder(w).Encode(event)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
