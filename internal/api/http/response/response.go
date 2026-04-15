package response

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/shared"
)

// JSON отправляет JSON ответ с указанным статусом
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if msg, ok := data.(proto.Message); ok {
		b, err := protojson.MarshalOptions{
			EmitUnpopulated: true,
			UseProtoNames:   true,
		}.Marshal(msg)
		if err != nil {
			log.Error().Err(err).Msg("ошибка кодирования protobuf JSON ответа")
			return
		}
		w.Write(b)
		w.Write([]byte("\n"))
		return
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Error().Err(err).Msg("ошибка кодирования JSON ответа")
	}
}

// Error отправляет ошибку AppError в фо��мате JSON
func Error(w http.ResponseWriter, err *shared.AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus)

	resp := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
	}

	if err.Details != "" {
		resp["error"].(map[string]interface{})["details"] = err.Details
	}

	json.NewEncoder(w).Encode(resp)
}

// GRPCError преобразует gRPC ошибку в HTTP JSON ответ
func GRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		log.Error().Err(err).Msg("неизвестная ошибка gRPC")
		Error(w, shared.ErrInternalServer("Внутренняя ошибка сервера"))
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
	case codes.FailedPrecondition:
		appErr = shared.ErrInvalidInput(st.Message())
	case codes.ResourceExhausted:
		appErr = &shared.AppError{Code: "TOO_MANY_REQUESTS", Message: st.Message(), HTTPStatus: http.StatusTooManyRequests}
	case codes.DeadlineExceeded, codes.Unavailable:
		appErr = shared.ErrServiceUnavailable("Сервис временно недоступен")
	default:
		log.Error().Err(err).Str("code", st.Code().String()).Msg("ошибка gRPC")
		appErr = shared.ErrInternalServer(st.Message())
	}

	Error(w, appErr)
}

// ParseIntParam извлекает целочисленный query-параметр с дефолтным значением
func ParseIntParam(r *http.Request, name string, defaultVal int32) int32 {
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

// ParsePagination извлекает параметры пагинации из запроса
func ParsePagination(r *http.Request) (page, perPage int32) {
	page = ParseIntParam(r, "page", config.DefaultMinPage)
	perPage = ParseIntParam(r, "per_page", config.DefaultPageSize)

	if page < config.DefaultMinPage {
		page = config.DefaultMinPage
	}
	if perPage < 1 {
		perPage = config.DefaultPageSize
	}
	if perPage > config.DefaultMaxPageSize {
		perPage = config.DefaultMaxPageSize
	}

	return page, perPage
}
