package utils

import (
	"encoding/json"
	"net/http"
)

type ApiResponse struct {
	Status  string `json:"status"` // success | fail
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func DecodeRequestBody(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
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
