package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/smpp-server/smpp-server/internal/shared"
)

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
