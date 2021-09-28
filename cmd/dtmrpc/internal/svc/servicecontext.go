package svc

import (
	"github.com/micro-easy/dtm/cmd/dtmrpc/internal/config"
	"github.com/micro-easy/dtm/model"

	"github.com/micro-easy/go-zero/core/collection"
	"github.com/micro-easy/go-zero/core/logx"
	"github.com/micro-easy/go-zero/core/stores/redis"
	"github.com/micro-easy/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Conf             config.Config
	TransGlobalModel *model.TransGlobalModel
	TransBranchModel *model.TransBranchModel
	TimingWheel      *collection.TimingWheel
	Redis            *redis.Redis
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlConn := sqlx.NewMysql(c.DataSource)
	return &ServiceContext{
		Conf:             c,
		TransGlobalModel: model.NewTransGlobalModel(sqlConn),
		TransBranchModel: model.NewTransBranchModel(sqlConn),
		Redis:            c.Redis.NewRedis(),
	}
}

func Stop(ctx *ServiceContext) {

	logx.Infof("server stop success!")
}
