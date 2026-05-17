package respond

import (
	"encoding/json"
	"net/http"

	"contrato/internal/storage"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func OK(w http.ResponseWriter, v any) { JSON(w, http.StatusOK, v) }

func Created(w http.ResponseWriter, v any) { JSON(w, http.StatusCreated, v) }

func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

func BadRequest(w http.ResponseWriter, msg string) { Error(w, http.StatusBadRequest, msg) }

func NotFound(w http.ResponseWriter) { Error(w, http.StatusNotFound, "not found") }

func InternalError(w http.ResponseWriter, err error) {
	Error(w, http.StatusInternalServerError, err.Error())
}

func StorageError(w http.ResponseWriter, err error) {
	switch err {
	case storage.ErrNotFound:
		NotFound(w)
	case storage.ErrConflict:
		Error(w, http.StatusConflict, "version conflict")
	default:
		InternalError(w, err)
	}
}
