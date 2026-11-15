package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/task-management/backend/internal/domain"
)

// Error codes are part of the API contract; clients branch on code, not message.
const (
	CodeValidation       = "VALIDATION_ERROR"
	CodeInvalidJSON      = "INVALID_JSON"
	CodeNotFound         = "NOT_FOUND"
	CodeDuplicateTitle   = "DUPLICATE_TITLE"
	CodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
	CodeInternal         = "INTERNAL_ERROR"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, errorBody{Error: errorDetail{Code: code, Message: msg}})
}

// respondError is the single place that maps domain errors to HTTP.
func respondError(c *gin.Context, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		writeError(c, http.StatusBadRequest, CodeValidation, ve.Message)
	case errors.Is(err, domain.ErrNotFound):
		writeError(c, http.StatusNotFound, CodeNotFound, err.Error())
	case errors.Is(err, domain.ErrDuplicateTitle):
		writeError(c, http.StatusConflict, CodeDuplicateTitle, err.Error())
	default:
		// Never leak internal details (SQL, hostnames) to clients.
		slog.Error("unhandled error", "path", c.FullPath(), "err", err)
		writeError(c, http.StatusInternalServerError, CodeInternal, "internal server error")
	}
}

// recovery turns panics into the standard error body instead of an empty 500.
func recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, rec any) {
		slog.Error("panic", "path", c.Request.URL.Path, "panic", rec)
		writeError(c, http.StatusInternalServerError, CodeInternal, "internal server error")
	})
}
