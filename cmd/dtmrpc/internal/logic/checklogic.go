package logic

import (
	"context"

	"github.com/micro-easy/dtm/cmd/dtmrpc/dtm"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/svc"

	"github.com/micro-easy/go-zero/core/logx"
)

type CheckLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckLogic {
	return &CheckLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckLogic) Check(in *dtm.CheckReq) (*dtm.CheckResp, error) {
	// todo: add your logic here and delete this line

	return &dtm.CheckResp{}, nil
}
