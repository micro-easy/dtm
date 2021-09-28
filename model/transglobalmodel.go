package model

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/micro-easy/go-zero/core/stores/sqlc"
	"github.com/micro-easy/go-zero/core/stores/sqlx"
	"github.com/micro-easy/go-zero/core/stringx"
	"github.com/micro-easy/go-zero/tools/goctl/model/sql/builderx"
)

var (
	transGlobalFieldNames        = builderx.FieldNames(&TransGlobal{})
	transGlobalRows              = strings.Join(transGlobalFieldNames, ",")
	transGlobalRowsExpectAutoSet = strings.Join(stringx.Remove(transGlobalFieldNames, "id", "create_time", "update_time", "version"), ",")
	transUnfinishedStatus        = strings.Join([]string{"\"prepared\"", "\"submitted\"", "\"exec\"", "\"rollback\""}, ",")
)

type (
	TransGlobalModel struct {
		conn  sqlx.SqlConn
		table string
	}

	TransGlobal struct {
		Id             int64     `db:"id"`
		Gid            string    `db:"gid"`             // 事务全局id
		Status         string    `db:"status"`          // 全局事务的状态  prepared | submitted | exec | rollback | success | abort | rollbacked
		CheckPrepared  string    `db:"check_prepared"`  // 检查prepared事务的回调
		CheckTriedNum  int64     `db:"check_tried_num"` // 全局事务的状态  prepared | submitted | exec | rollback | success | abort | rollbacked
		Source         string    `db:"source"`          // 全局事务原始参数
		ExpireDuration int64     `db:"expire_duration"` // 超时时间间隔
		CreateTime     time.Time `db:"create_time"`
		UpdateTime     time.Time `db:"update_time"`
		Version        int64     `db:"version"`
	}
)

func NewTransGlobalModel(conn sqlx.SqlConn) *TransGlobalModel {
	return &TransGlobalModel{
		conn:  conn,
		table: "`trans_global`",
	}
}

func (m *TransGlobalModel) Transact(fn func(session sqlx.Session) error) error {
	return m.conn.Transact(fn)
}

func (m *TransGlobalModel) Insert(data TransGlobal) (sql.Result, error) {
	query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?)", m.table, transGlobalRowsExpectAutoSet)
	ret, err := m.conn.Exec(query, data.Gid, data.Status, data.CheckPrepared, data.CheckTriedNum, data.Source, data.ExpireDuration)
	return ret, err
}

func (m *TransGlobalModel) FindOne(id int64) (*TransGlobal, error) {
	query := fmt.Sprintf("select %s from %s where id = ? limit 1", transGlobalRows, m.table)
	var resp TransGlobal
	err := m.conn.QueryRow(&resp, query, id)
	switch err {
	case nil:
		return &resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *TransGlobalModel) FindOneByGid(gid string) (*TransGlobal, error) {
	var resp TransGlobal
	query := fmt.Sprintf("select %s from %s where gid = ? limit 1", transGlobalRows, m.table)
	err := m.conn.QueryRow(&resp, query, gid)
	switch err {
	case nil:
		return &resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *TransGlobalModel) FindExpiredTrans(expireTime, limit int64) ([]*TransGlobal, error) {
	var resp []*TransGlobal
	query := fmt.Sprintf("select %s from %s where status in (%s) and update_time < ? limit ? ", transGlobalRows, m.table, transUnfinishedStatus)
	err := m.conn.QueryRows(&resp, query, time.Now().Add(-time.Duration(expireTime)*time.Second), limit)
	switch err {
	case nil:
		return resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *TransGlobalModel) InsertWithSession(s sqlx.Session, data *TransGlobal) (sql.Result, error) {
	query := fmt.Sprintf("insert into %s (%s) values ( ?, ?, ?, ?, ?, ?)", m.table, transGlobalRowsExpectAutoSet)
	ret, err := s.Exec(query, data.Gid, data.Status, data.CheckPrepared, data.CheckTriedNum, data.Source, data.ExpireDuration)
	return ret, err
}

func (m *TransGlobalModel) UpdateStatusWithSession(s sqlx.Session, data *TransGlobal, ver int64) (sql.Result, error) {
	query := fmt.Sprintf("update %s set status = ? ,check_tried_num = ?, version = ? where id = ? and version = ? ", m.table)
	return s.Exec(query, data.Status, data.CheckTriedNum, ver, data.Id, data.Version)
}

func (m *TransGlobalModel) Touch(data *TransGlobal) (sql.Result, error) {
	query := fmt.Sprintf("update %s set update_time = ? where id = ? ", m.table)
	return m.conn.Exec(query, time.Now(), data.Id)
}
