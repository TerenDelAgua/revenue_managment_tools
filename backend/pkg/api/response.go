package api

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

type Error struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

func SendError(w http.ResponseWriter, status int, message string) {
	JSON(w, status, Error{Message: message})
}

func BadRequest(w http.ResponseWriter, message string) {
	SendError(w, http.StatusBadRequest, message)
}

func NotFound(w http.ResponseWriter, message string) {
	SendError(w, http.StatusNotFound, message)
}

func InternalServerError(w http.ResponseWriter, message string) {
	SendError(w, http.StatusInternalServerError, message)
}
