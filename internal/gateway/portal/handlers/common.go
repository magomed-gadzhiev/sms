package handlers

import (
	"net/http"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/smpp-server/smpp-server/internal/api/http/response"
)

// convertMapToStruct converts a map[string]interface{} to a *structpb.Struct
func convertMapToStruct(m map[string]interface{}) (*structpb.Struct, error) {
	return structpb.NewStruct(m)
}

// respondJSON delegates to the shared response package
var respondJSON = response.JSON

// respondError delegates to the shared response package
var respondError = response.Error

// respondGRPCError delegates to the shared response package
var respondGRPCError = response.GRPCError

// parseIntParam delegates to the shared response package
func parseIntParam(r *http.Request, name string, defaultVal int32) int32 {
	return response.ParseIntParam(r, name, defaultVal)
}

// parsePagination delegates to the shared response package
func parsePagination(r *http.Request) (page, perPage int32) {
	return response.ParsePagination(r)
}
