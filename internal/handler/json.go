package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"billing-payment-api/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const maxRequestBytes = 1 << 20 // 1 MiB

// bindJSON accepts exactly one JSON object and validates its declared fields.
func bindJSON(c *gin.Context, target any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		response.Error(c, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json", nil)
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(target)
	if err == nil {
		var extra any
		err = decoder.Decode(&extra)
		if errors.Is(err, io.EOF) {
			err = binding.Validator.ValidateStruct(target)
		} else if err == nil {
			err = errors.New("body must contain exactly one JSON object")
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Error(c, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "body must not exceed 1 MiB", nil)
		} else {
			response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), nil)
		}
		return false
	}
	return true
}
