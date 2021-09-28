package config

import (
	"github.com/micro-easy/go-zero/core/stores/redis"
	"github.com/micro-easy/go-zero/zrpc"
)

type Config struct {
	Env string `json:",default=dev,options=dev|qa|stag|pre|pro"` // 运行环境
	zrpc.RpcServerConf
	DataSource string   // 数据库地址
	Tables     struct { // 表名集合
		TransGlobal string
		TransBranch string
	}
	TransCronInterval int64 `json:",default=10"`
	ExpireTime        int64 `json:",default=60"`
	ExpireLimit       int64 `json:",default=50"`
	Redis             redis.RedisConf
}
