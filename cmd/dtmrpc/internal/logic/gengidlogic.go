package logic

import (
	"context"

	"github.com/micro-easy/dtm/cmd/dtmrpc/dtm"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/svc"

	"github.com/micro-easy/go-zero/core/logx"
)

type GenGidLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenGidLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenGidLogic {
	return &GenGidLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GenGidLogic) GenGid(in *dtm.GenGidReq) (*dtm.GenGidResp, error) {
	// the id generated by snowflake may not be unique, so we add ip to make
	// it unique
	return &dtm.GenGidResp{
		Gid: genGid(),
	}, nil
}
