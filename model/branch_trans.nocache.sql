CREATE TABLE IF NOT EXISTS trans_branch (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `gid` varchar(128) NOT NULL COMMENT '事务全局id',
  `name` varchar(60) NOT NULL COMMENT '操作名称',
  `action` TEXT NOT NULL COMMENT '正向操作数据',
  `compensate` TEXT COMMENT '补偿操作数据',
  `layer_num` smallint NOT NULL default 0,
  `action_tried_num` int(11) NOT NULL default 0 COMMENT '重试次数',
  `compensate_tried_num` int(11) NOT NULL default 0 COMMENT '重试次数',
  `status` varchar(20) NOT NULL COMMENT '步骤的状态 prepared | submitted | exec | rollback | success | abort | rollbacked',
  `version` int(11) not null default 0 comment '版本更新编号',
  `create_time` datetime DEFAULT CURRENT_TIMESTAMP,
  `update_time` datetime DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `gid` (`gid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;