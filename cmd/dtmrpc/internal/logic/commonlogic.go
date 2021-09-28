package logic

import (
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/micro-easy/dtm/cmd/dtmrpc/dtm"
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/svc"
	"github.com/micro-easy/dtm/model"

	"github.com/micro-easy/go-zero/core/logx"
	"github.com/micro-easy/go-zero/core/mr"
	"github.com/micro-easy/go-zero/core/netx"
	"github.com/micro-easy/go-zero/core/stores/redis"
	"github.com/micro-easy/go-zero/core/stores/sqlx"
)

type StepInfo struct {
	bTrans     *model.TransBranch
	Action     *dtm.Action
	Compensate *dtm.Action
}
type GlobalTransInfo struct {
	StepsInfo   []*StepInfo
	CheckAction *dtm.Action
	gTrans      *model.TransGlobal
	svc         *svc.ServiceContext
}

func NewGlobalTransInfo(svc *svc.ServiceContext, gt *model.TransGlobal) (*GlobalTransInfo, error) {
	var err error
	obj := GlobalTransInfo{}
	obj.svc = svc
	obj.gTrans = gt
	obj.CheckAction, err = decodeAction(gt.CheckPrepared)
	if err != nil {
		logx.Errorf("decodeAction CheckPrepared err %v", err)
		return nil, err
	}
	obj.StepsInfo, err = getStepsInfo(svc, gt.Gid)
	if err != nil {
		logx.Errorf("getStepsInfo  err %v", err)
		return nil, err
	}
	return &obj, nil
}

func (g *GlobalTransInfo) findSteps(asc bool, statuses ...string) []*StepInfo {
	if asc {
		sort.SliceStable(g.StepsInfo, func(i, j int) bool {
			return g.StepsInfo[i].bTrans.LayerNum < g.StepsInfo[j].bTrans.LayerNum
		})
	} else {
		sort.SliceStable(g.StepsInfo, func(i, j int) bool {
			return g.StepsInfo[i].bTrans.LayerNum > g.StepsInfo[j].bTrans.LayerNum
		})
	}

	var res []*StepInfo
	layer := int64(0)
	got := false
	for _, step := range g.StepsInfo {
		if got && step.bTrans.LayerNum != layer {
			continue
		}
		for _, status := range statuses {
			if status == step.bTrans.Status {
				res = append(res, step)
				got = true
				layer = step.bTrans.LayerNum
			}
		}
	}
	return res
}

// save global and branch transactions
func (g *GlobalTransInfo) save() error {
	ver := g.gTrans.Version + 1
	return g.svc.TransGlobalModel.Transact(func(s sqlx.Session) error {
		if err := checkSqlRet(g.svc.TransGlobalModel.UpdateStatusWithSession(s, g.gTrans, ver)); err != nil {
			return err
		}

		for _, stepInfo := range g.StepsInfo {
			if err := checkSqlRet(g.svc.TransBranchModel.UpdateStatusWithSession(s, stepInfo.bTrans, ver)); err != nil {
				return err
			}
			stepInfo.bTrans.Version = ver
		}
		g.gTrans.Version = ver
		return nil
	})
}

func (g *GlobalTransInfo) setFullStatus(status string) {
	g.gTrans.Status = status
	for _, stepInfo := range g.StepsInfo {
		stepInfo.bTrans.Status = status
	}
}

func (g *GlobalTransInfo) invokeCheckAction() error {
	if g.gTrans.CheckTriedNum >= g.CheckAction.RetryNum {
		g.setFullStatus(SubmitAbort)
		return g.save()
	}

	submitted, err := InvokeAction(g.CheckAction)
	if err != nil {
		g.gTrans.CheckTriedNum += 1
		if err := g.save(); err != nil {
			return err
		}
		return err
	}

	if !submitted {
		g.setFullStatus(SubmitAbort)
		return g.save()
	}
	// here change g status to submitted
	g.setFullStatus(Submitted)
	return g.save()
}

type StepResult struct {
	Step *StepInfo
	Err  error
}

func (g *GlobalTransInfo) invokeSteps(currentSteps []*StepInfo, isCompensate bool) error {
	var retErr error
	if !isCompensate {
		mr.MapReduce(func(source chan<- interface{}) {
			for _, curStep := range currentSteps {
				source <- curStep
			}
		}, func(item interface{}, writer mr.Writer, cancel func(error)) {
			curStep := item.(*StepInfo)
			stepResult := &StepResult{
				Step: curStep,
				Err:  nil,
			}
			if curStep.bTrans.ActionTriedNum >= curStep.Action.RetryNum {
				curStep.bTrans.Status = Abort
				writer.Write(stepResult)
				return
			}
			// handle exec step
			if curStep.Action != nil {
				_, err := InvokeAction(curStep.Action)
				if err != nil {
					curStep.bTrans.ActionTriedNum += 1
					stepResult.Err = err
					writer.Write(stepResult)
					return
				}
				curStep.bTrans.Status = Success
			} else {
				curStep.bTrans.Status = UnExpectedError
			}

			writer.Write(stepResult)
		}, func(pipe <-chan interface{}, writer mr.Writer, cancel func(error)) {
			for item := range pipe {
				stepResult := item.(*StepResult)
				if stepResult.Step.bTrans.Status == Abort {
					g.gTrans.Status = Rollback
				}
			}
			retErr = g.save()
		})
		return retErr
	}

	mr.MapReduce(func(source chan<- interface{}) {
		for _, curStep := range currentSteps {
			source <- curStep
		}
	}, func(item interface{}, writer mr.Writer, cancel func(error)) {
		curStep := item.(*StepInfo)
		stepResult := &StepResult{
			Step: curStep,
			Err:  nil,
		}
		if curStep.bTrans.CompensateTriedNum >= curStep.Compensate.RetryNum {
			curStep.bTrans.Status = RollbackAbort
			writer.Write(stepResult)
			return
		}
		// handle exec step
		if curStep.Compensate != nil {
			_, err := InvokeAction(curStep.Compensate)
			if err != nil {
				curStep.bTrans.CompensateTriedNum += 1
				stepResult.Err = err
				writer.Write(stepResult)
				return
			}
		}

		curStep.bTrans.Status = Rollbacked
		writer.Write(stepResult)

	}, func(pipe <-chan interface{}, writer mr.Writer, cancel func(error)) {
		for item := range pipe {
			stepResult := item.(*StepResult)
			if stepResult.Step.bTrans.Status == RollbackAbort {
				g.gTrans.Status = RollbackAbort
				retErr = ErrAbort
			}
		}
		if err := g.save(); err != nil {
			retErr = err
		}
	})
	return retErr
}

func genGid() string {
	return netx.InternalIp() + gNode.Generate().Base64()
}

func decodeAction(actionStr string) (*dtm.Action, error) {
	action := &dtm.Action{}
	if err := json.Unmarshal([]byte(actionStr), action); err != nil {
		logx.Errorf("decodeAction err %v for action %s", err, actionStr)
		return nil, err
	}
	return action, nil
}

type ResponseData struct {
	Errcode   int64  `json:"errcode"`
	Errmsg    string `json:"errmsg"`
	CanSubmit bool   `json:"can_submit"`
}

func InvokeAction(action *dtm.Action) (bool, error) {
	// for test purpose
	if action.Url == "test" {
		time.Sleep(time.Second)
		if action.Data == "success" {
			return false, nil
		} else if action.Data == "fail" {
			return false, RpcError("invoke error")
		} else if action.Data == "cansub" {
			return true, nil
		} else if action.Data == "cannotsub" {
			return false, nil
		} else {
			return false, RpcError("unknown error")
		}
	}
	// only for http protocol
	resp, err := RestyClient.R().SetBody(string(action.Data)).
		SetHeader("Content-type", "application/json").
		Execute("POST", action.Url)
	if err != nil {
		return false, err
	}
	var respData ResponseData
	err = json.Unmarshal([]byte(resp.String()), &respData)
	if err != nil {
		return false, err
	}
	if respData.Errcode != 0 {
		return false, RpcError("errcode not zero")
	}
	return respData.CanSubmit, nil
}

// handle all steps
func handleSteps(svc *svc.ServiceContext, gid string, locker *redis.RedisLock) error {
	locked, err := locker.Acquire()
	if err != nil {
		return err
	}
	if !locked {
		return RpcError("get lock failed")
	}
	defer locker.Release()
	gTrans, err := svc.TransGlobalModel.FindOneByGid(gid)
	if err != nil {
		logx.Errorf("FindOneByGid err %v", err)
		return err
	}
	g, err := NewGlobalTransInfo(svc, gTrans)
	if err != nil {
		logx.Errorf("NewGlobalTransInfo err %v", err)
		return err
	}
	// check the transaction submitted
	err = handleStep(g)
	if err != nil && err != ErrInternal && err != ErrAbort && err != ErrNoRowAffected {
		// set timer
		g.svc.TimingWheel.SetTimer(gTrans.Gid, struct{}{}, time.Duration(gTrans.ExpireDuration)*time.Second)
	}
	return nil

}

// 出错情况下，跳出循环，等待延时处理，内部错误ErrInternal/ErrAbort不再处理;
// 处理成功情况下，如何过渡到下一个step；这其中包括正向的和反向的处理
func handleStep(g *GlobalTransInfo) error {
	for {
		// check global status
		switch g.gTrans.Status {
		case Prepared:
			if g.CheckAction == nil {
				// serve error
				g.gTrans.Status = UnExpectedError
				if err := g.save(); err != nil {
					return err
				}
				return ErrInternal
			}

			// invoke check action, when error occurs, try later
			// if submit success, then handle submit status
			if err := g.invokeCheckAction(); err != nil {
				return err
			}

		case Submitted:
			// check branch status and set
			currentSteps := g.findSteps(true, Submitted)
			logx.Infof("Submitted currentSteps %v", currentSteps)
			if len(currentSteps) == 0 {
				// serve error
				g.gTrans.Status = UnExpectedError
				if err := g.save(); err != nil {
					return err
				}
				return ErrInternal
			}
			// set status and flush
			g.gTrans.Status = Exec
			for _, curStep := range currentSteps {
				curStep.bTrans.Status = Exec
			}
			if err := g.save(); err != nil {
				logx.Errorf("save err %v", err)
				return err
			}
			// invoke step
			if err := g.invokeSteps(currentSteps, false); err != nil {
				return err
			}
		case Exec:
			// check branch status and set
			currentSteps := g.findSteps(true, Exec, Submitted)
			if len(currentSteps) == 0 {
				// all branch finished
				g.gTrans.Status = Success
				return g.save()
			}
			// set status and flush
			statusChange := false
			for _, curStep := range currentSteps {
				if curStep.bTrans.Status == Submitted {
					curStep.bTrans.Status = Exec
					statusChange = true
				}
			}
			if statusChange {
				if err := g.save(); err != nil {
					return err
				}
			}

			// invoke step
			if err := g.invokeSteps(currentSteps, false); err != nil {
				return err
			}
		case Rollback:
			// find all brother branches and father branches
			currentSteps := g.findSteps(false, Exec, Success, Rollback)
			if len(currentSteps) == 0 {
				// all branch rollbacked
				g.gTrans.Status = Rollbacked
				return g.save()
			}
			// set status and flush
			statusChange := false
			for _, curStep := range currentSteps {
				if curStep.bTrans.Status != Rollback {
					curStep.bTrans.Status = Rollback
					statusChange = true
				}
			}
			if statusChange {
				if err := g.save(); err != nil {
					return err
				}
			}

			// invoke step
			if err := g.invokeSteps(currentSteps, true); err != nil {
				return err
			}
		}
	}

}

func getStepsInfo(svc *svc.ServiceContext, gid string) ([]*StepInfo, error) {
	bTranses, err := svc.TransBranchModel.FindAllByGid(gid)
	if err != nil {
		logx.Errorf("FindAllByGid err %v", err)
		return nil, err
	}
	var stepsInfo []*StepInfo
	for _, bTrans := range bTranses {
		action, err := decodeAction(bTrans.Action)
		if err != nil {
			return nil, err
		}
		compensate, err := decodeAction(bTrans.Compensate)
		if err != nil {
			return nil, err
		}
		stepsInfo = append(stepsInfo, &StepInfo{
			bTrans:     bTrans,
			Action:     action,
			Compensate: compensate,
		})
	}
	return stepsInfo, nil
}

func checkSqlRet(ret sql.Result, err error) error {
	if err != nil {
		return err
	}
	rowNum, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if rowNum == 0 {
		return ErrNoRowAffected
	}
	return nil
}
