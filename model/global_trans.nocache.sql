CREATE TABLE if not EXISTS trans_global (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `gid` varchar(128) NOT NULL COMMENT '事务全局id',
  `status` varchar(20) NOT NULL default '' COMMENT '全局事务的状态  prepared | submitted | exec | rollback | success | abort | rollbacked',
  `check_prepared` varchar(256) COMMENT '检查prepared事务的回调', 
  `check_tried_num` int(11) NOT NULL default 0 COMMENT '全局事务的状态  prepared | submitted | exec | rollback | success | abort | rollbacked',
  `source` TEXT NOT NULL COMMENT '全局事务原始参数',
  `expire_duration` int(11) not null comment '超时时间间隔',
  `version` int(11) comment '版本更新编号',
  `create_time` datetime DEFAULT CURRENT_TIMESTAMP ,
  `update_time` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `gid` (`gid`),
  KEY `status_updatetime_idx` (`status`,`update_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;