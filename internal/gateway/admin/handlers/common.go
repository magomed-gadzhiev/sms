package handlers

import (
	"net/http"
	"strconv"

	"github.com/smpp-server/smpp-server/internal/api/http/response"
)

// respondJSON delegates to the shared response package
var respondJSON = response.JSON

// respondError delegates to the shared response package
var respondError = response.Error

// respondGRPCError delegates to the shared response package
var respondGRPCError = response.GRPCError

// parseIntParam извлекает целочисленный query-параметр с дефолтным значением
func parseIntParam(r *http.Request, name string, defaultVal int32) int32 {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return int32(n)
}
