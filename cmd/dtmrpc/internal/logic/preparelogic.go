package logic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/micro-easy/dtm/cmd/dtmrpc/dtm"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/svc"
	"github.com/micro-easy/dtm/model"

	"github.com/micro-easy/go-zero/core/logx"
	"github.com/micro-easy/go-zero/core/stores/sqlx"
)

type PrepareLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPrepareLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrepareLogic {
	return &PrepareLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PrepareLogic) Prepare(in *dtm.PrepareReq) (*dtm.PrepareResp, error) {
	if len(in.Steps) == 0 || in.FirstExec <= 0 {
		return nil, ErrArg
	}
	// 生成唯一事务id
	gid := genGid()

	if in.Check == nil {
		return nil, ErrArg
	}
	checkInfo, err := json.Marshal(in.Check)
	if err != nil {
		return nil, RpcError(err)
	}

	stepsSource, err := json.Marshal(in.Steps)
	if err != nil {
		return nil, RpcError(err)
	}

	if err := l.svcCtx.TransGlobalModel.Transact(func(s sqlx.Session) error {
		if err := checkSqlRet(l.svcCtx.TransGlobalModel.InsertWithSession(s, &model.TransGlobal{
			Gid:            gid,
			Status:         Prepared,
			ExpireDuration: in.ExpireDuration,
			CheckTriedNum:  0,
			CheckPrepared:  string(checkInfo),
			Source:         string(stepsSource),
		})); err != nil {
			logx.Infof("insert global err %v", err)
			return err
		}

		for _, step := range in.Steps {
			if step.Action == nil {
				return RpcError("nil action")
			}

			actionStr, err := json.Marshal(step.Action)
			if err != nil {
				return err
			}
			compensateStr, err := json.Marshal(step.Compensate)
			if err != nil {
				return err
			}
			if err := checkSqlRet(l.svcCtx.TransBranchModel.InsertWithSession(s, &model.TransBranch{
				Gid:                gid,
				Name:               step.Name,
				Action:             string(actionStr),
				Compensate:         string(compensateStr),
				ActionTriedNum:     0,
				CompensateTriedNum: 0,
				LayerNum:           step.LayerNum,
				Status:             Prepared,
			})); err != nil {
				logx.Infof("insert branch err %v", err)
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// 超时用时间轮算法扫描
	l.svcCtx.TimingWheel.SetTimer(gid, time.Now().Unix(), time.Duration(in.FirstExec)*time.Second)
	// 还得有个保底的扫描算法
	return &dtm.PrepareResp{
		Gid: gid,
	}, nil
}
