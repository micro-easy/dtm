package logic

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrArg           = RpcErrorWithCode(int(codes.InvalidArgument), "参数有误")
	ErrInternal      = RpcErrorWithCode(int(codes.Internal), "内部错误")
	ErrAbort         = RpcError("transaction abort")
	ErrNoRowAffected = RpcError("no rows affected")
)

func RpcError(a ...interface{}) error {
	return status.Error(codes.Code(10000), fmt.Sprint(a...))
}

func RpcErrorf(format string, a ...interface{}) error {
	return status.Error(codes.Code(10000), fmt.Sprintf(format, a...))
}

func RpcErrorWithCode(c int, a ...interface{}) error {
	return status.Error(codes.Code(c), fmt.Sprint(a...))
}

func RpcErrorfWithCode(c int, format string, a ...interface{}) error {
	return status.Error(codes.Code(c), fmt.Sprintf(format, a...))
}
