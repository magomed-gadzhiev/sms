package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/smpp-server/smpp-server/internal/shared"
)

const traceIDKey = "x-request-id"

// TraceUnaryServerInterceptor извлекает request_id из gRPC metadata
// и добавляет его в контекст. Если request_id отсутствует — генерирует новый.
func TraceUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		requestID := extractRequestID(ctx)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx = shared.WithRequestID(ctx, requestID)
		logger := log.With().Str("request_id", requestID).Logger()
		ctx = logger.WithContext(ctx)

		// Добавляем request_id в ответные headers
		_ = grpc.SetHeader(ctx, metadata.Pairs(traceIDKey, requestID))

		return handler(ctx, req)
	}
}

// TraceUnaryClientInterceptor пропагирует request_id из контекста
// в gRPC metadata исходящего вызова.
func TraceUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		requestID := shared.GetRequestID(ctx)
		if requestID != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, traceIDKey, requestID)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// extractRequestID извлекает request_id из входящей gRPC metadata.
func extractRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(traceIDKey)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}
