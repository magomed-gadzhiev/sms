package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/smpp-server/smpp-server/internal/shared"
)

// convertMapToStruct converts a map[string]interface{} to a *structpb.Struct
func convertMapToStruct(m map[string]interface{}) (*structpb.Struct, error) {
	return structpb.NewStruct(m)
}

// respondJSON отправляет JSON ответ
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("ошибка кодирования JSON ответа")
	}
}

// respondError отправляет ошибку в формате JSON
func respondError(w http.ResponseWriter, err *shared.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
	}

	if err.Details != "" {
		response["error"].(map[string]interface{})["details"] = err.Details
	}

	json.NewEncoder(w).Encode(response)
}

// respondGRPCError преобразует gRPC ошибку в HTTP ответ
func respondGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		log.Error().Err(err).Msg("неизвестная ошибка gRPC")
		respondError(w, shared.ErrInternalServer("Внутренняя ошибка сервера"))
		return
	}

	var appErr *shared.AppError
	switch st.Code() {
	case codes.NotFound:
		appErr = shared.ErrNotFound(st.Message())
	case codes.InvalidArgument:
		appErr = shared.ErrInvalidInput(st.Message())
	case codes.Unauthenticated:
		appErr = shared.ErrUnauthorized(st.Message())
	case codes.PermissionDenied:
		appErr = shared.ErrForbidden(st.Message())
	case codes.AlreadyExists:
		appErr = shared.ErrConflict(st.Message())
	case codes.ResourceExhausted:
		appErr = &shared.AppError{Code: "TOO_MANY_REQUESTS", Message: st.Message(), HTTPStatus: http.StatusTooManyRequests}
	case codes.DeadlineExceeded, codes.Unavailable:
		appErr = shared.ErrServiceUnavailable("Сервис временно недоступен")
	default:
		log.Error().Err(err).Str("code", st.Code().String()).Msg("ошибка gRPC")
		appErr = shared.ErrInternalServer(st.Message())
	}

	respondError(w, appErr)
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

// parsePagination извлекает параметры пагинации из запроса
func parsePagination(r *http.Request) (page, perPage int32) {
	page = parseIntParam(r, "page", 1)
	perPage = parseIntParam(r, "per_page", 20)

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	return page, perPage
}
