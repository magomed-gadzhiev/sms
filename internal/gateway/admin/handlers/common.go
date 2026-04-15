package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/smpp-server/smpp-server/internal/api/http/response"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// respondJSON delegates to the shared response package
var respondJSON = response.JSON

// respondError delegates to the shared response package
var respondError = response.Error

// respondGRPCError delegates to the shared response package
var respondGRPCError = response.GRPCError

// safeTimestamp безопасно преобразует *timestamppb.Timestamp в time.Time.
// Возвращает zero time если ts == nil, предотвращая nil pointer dereference.
func safeTimestamp(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// nullableString converts *string to interface{} for SQL parameters.
// nil pointer or empty string → nil (SQL NULL); otherwise the value.
func nullableString(s *string) interface{} {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

// parsePagination извлекает page и per_page из query-параметров
func parsePagination(r *http.Request) (page, perPage int32) {
	page = parseIntParam(r, "page", 1)
	perPage = parseIntParam(r, "per_page", 50)
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 500 {
		perPage = 50
	}
	return
}

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
