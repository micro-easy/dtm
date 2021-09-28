package logic

import (
	"context"
	"encoding/json"

	"github.com/micro-easy/dtm/cmd/dtmrpc/dtm"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/svc"
	"github.com/micro-easy/dtm/model"

	"github.com/micro-easy/go-zero/core/stores/redis"
	"github.com/micro-easy/go-zero/core/stores/sqlx"
	"github.com/micro-easy/go-zero/core/threading"

	"github.com/micro-easy/go-zero/core/logx"
)

type SubmitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSubmitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubmitLogic {
	return &SubmitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SubmitLogic) Submit(in *dtm.SubmitReq) (*dtm.SubmitResp, error) {
	if len(in.Gid) == 0 {
		return nil, ErrArg
	}
	locker := redis.NewRedisLock(l.svcCtx.Redis, in.Gid)
	locked, err := locker.Acquire()
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, RpcError("get lock failed")
	}

	gt, err := l.svcCtx.TransGlobalModel.FindOneByGid(in.Gid)
	if err != nil && err != model.ErrNotFound {
		return nil, err
	}

	submitted := false
	if gt == nil {
		stepsSource, err := json.Marshal(in.Steps)
		if err != nil {
			return nil, err
		}

		if err := l.svcCtx.TransGlobalModel.Transact(func(s sqlx.Session) error {
			if err := checkSqlRet(l.svcCtx.TransGlobalModel.InsertWithSession(s, &model.TransGlobal{
				Gid:            in.Gid,
				Status:         Submitted,
				ExpireDuration: in.ExpireDuration,
				Source:         string(stepsSource),
			})); err != nil {
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
					Gid:                in.Gid,
					Name:               step.Name,
					Action:             string(actionStr),
					Compensate:         string(compensateStr),
					ActionTriedNum:     0,
					CompensateTriedNum: 0,
					LayerNum:           step.LayerNum,
					Status:             Submitted,
				})); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}

		submitted = true
	} else {
		if gt.Status == Prepared {
			g, err := NewGlobalTransInfo(l.svcCtx, gt)
			if err != nil {
				return nil, err
			}
			g.gTrans.Status = Submitted
			for _, step := range g.StepsInfo {
				step.bTrans.Status = Submitted
			}
			if err := g.save(); err != nil {
				return nil, err
			}
			submitted = true
		}
	}

	// 此处利用锁的可重入性，将锁移交给执行函数处理
	if submitted && in.FirstExec == 0 {
		threading.GoSafe(func() {
			if err := handleSteps(l.svcCtx, in.Gid, locker); err != nil {
				logx.Infof("handleSteps err %v for g %s", err, in.Gid)
			}
		})
	} else {
		locker.Release()
	}

	return &dtm.SubmitResp{
		Gid: in.Gid,
	}, nil
}
