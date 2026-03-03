package interceptors

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type ctxKey int

const traceIDKey ctxKey = iota

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

func TraceIDInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		var traceID string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get("x-trace-id"); len(vals) > 0 {
				traceID = vals[0]
			}
		}

		if traceID == "" {
			traceID = uuid.NewString()
		}

		ctx = WithTraceID(ctx, traceID)
		ctx = metadata.AppendToOutgoingContext(ctx, "x-trace-id", traceID)

		return handler(ctx, req)
	}

}

func TraceIDClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		traceID := GetTraceID(ctx)
		if traceID == "" {
			traceID = "unknown"
		}

		md := metadata.Pairs("x-trace-id", traceID)
		ctx = metadata.NewOutgoingContext(ctx, md)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
