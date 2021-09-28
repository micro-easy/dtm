package logic

import (
	"fmt"
	"sync"
	"time"

	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/svc"

	"github.com/bwmarrin/snowflake"
	"github.com/go-resty/resty/v2"
	"github.com/micro-easy/go-zero/core/collection"
	"github.com/micro-easy/go-zero/core/logx"
	"github.com/micro-easy/go-zero/core/stores/redis"
	"github.com/micro-easy/go-zero/core/threading"
)

var (
	gNode       *snowflake.Node
	once        sync.Once
	RestyClient *resty.Client
)

func init() {
	once.Do(
		func() {
			var err error
			// all nodes num assigned by the same number 1, so the id generated
			// by snowflake may not be unique
			gNode, err = snowflake.NewNode(1)
			if err != nil {
				panic(fmt.Sprintf("init snowflake node failed %v", err))
			}
			RestyClient = resty.New()
		})
}

func InitTasks(svc *svc.ServiceContext) {
	var err error
	svc.TimingWheel, err = collection.NewTimingWheel(time.Second, 300, func(k, v interface{}) {
		// TODO handle the expired global transactions
		// k is gid and v is details
		gid, ok := k.(string)
		if !ok {
			return
		}
		locker := redis.NewRedisLock(svc.Redis, gid)
		if err := handleSteps(svc, gid, locker); err != nil {
			logx.Errorf("handleSteps err %v for gid %s", err, gid)
		}

	})
	if err != nil {
		panic(err)
	}
	threading.GoSafe(func() {
		c := time.Tick(time.Duration(svc.Conf.TransCronInterval) * time.Second)
		for {
			<-c
			threading.GoSafe(func() {
				// here get a lock
				scanLocker := redis.NewRedisLock(svc.Redis, ExpireScanLocker)
				locked, _ := scanLocker.Acquire()
				if !locked {
					return
				}
				defer scanLocker.Release()
				exGlobalTranses, err := svc.TransGlobalModel.FindExpiredTrans(svc.Conf.ExpireTime, svc.Conf.ExpireLimit)
				if err != nil {
					logx.Errorf("FindExpiredTrans err %v", err)
					return
				}
				if len(exGlobalTranses) == 0 {
					return
				}
				for _, exGlobalTrans := range exGlobalTranses {
					// touch the global trans
					svc.TransGlobalModel.Touch(exGlobalTrans)
					// go handle the global trans
					threading.GoSafe(func() {
						locker := redis.NewRedisLock(svc.Redis, exGlobalTrans.Gid)
						if err := handleSteps(svc, exGlobalTrans.Gid, locker); err != nil {
							logx.Errorf("handleSteps err %v for gid %s", err, exGlobalTrans.Gid)
						}
					})
				}

			})
		}
	})
}
