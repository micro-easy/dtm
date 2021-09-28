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
	transBranchFieldNames          = builderx.FieldNames(&TransBranch{})
	transBranchRows                = strings.Join(transBranchFieldNames, ",")
	transBranchRowsExpectAutoSet   = strings.Join(stringx.Remove(transBranchFieldNames, "id", "create_time", "update_time", "version"), ",")
	transBranchRowsWithPlaceHolder = strings.Join(stringx.Remove(transBranchFieldNames, "id", "create_time", "update_time", "version"), "=?,") + "=?"
)

type (
	TransBranchModel struct {
		conn  sqlx.SqlConn
		table string
	}

	TransBranch struct {
		Id                 int64     `db:"id"`
		Gid                string    `db:"gid"`        // 事务全局id
		Name               string    `db:"name"`       // 操作名称
		Action             string    `db:"action"`     // 正向操作数据
		Compensate         string    `db:"compensate"` // 补偿操作数据
		LayerNum           int64     `db:"layer_num"`
		ActionTriedNum     int64     `db:"action_tried_num"`     // 重试次数
		CompensateTriedNum int64     `db:"compensate_tried_num"` // 重试次数
		Status             string    `db:"status"`               // 步骤的状态 prepared | submitted | exec | rollback | success | abort | rollbacked
		CreateTime         time.Time `db:"create_time"`
		UpdateTime         time.Time `db:"update_time"`
		Version            int64     `db:"version"`
	}
)

func NewTransBranchModel(conn sqlx.SqlConn) *TransBranchModel {
	return &TransBranchModel{
		conn:  conn,
		table: "`trans_branch`",
	}
}

func (m *TransBranchModel) Transact(fn func(session sqlx.Session) error) error {
	return m.conn.Transact(fn)
}

func (m *TransBranchModel) Insert(data TransBranch) (sql.Result, error) {
	query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?, ?)", m.table, transBranchRowsExpectAutoSet)
	ret, err := m.conn.Exec(query, data.Gid, data.Name, data.Action, data.Compensate, data.LayerNum, data.ActionTriedNum, data.CompensateTriedNum, data.Status)
	return ret, err
}

func (m *TransBranchModel) InsertWithSession(s sqlx.Session, data *TransBranch) (sql.Result, error) {
	query := fmt.Sprintf("insert into %s (%s) values (?, ?, ?, ?, ?, ?, ?, ?)", m.table, transBranchRowsExpectAutoSet)
	ret, err := s.Exec(query, data.Gid, data.Name, data.Action, data.Compensate, data.LayerNum, data.ActionTriedNum, data.CompensateTriedNum, data.Status)
	return ret, err
}

func (m *TransBranchModel) FindOne(id int64) (*TransBranch, error) {
	query := fmt.Sprintf("select %s from %s where id = ? limit 1", transBranchRows, m.table)
	var resp TransBranch
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

func (m *TransBranchModel) Update(data TransBranch) error {
	query := fmt.Sprintf("update %s set %s where id = ?", m.table, transBranchRowsWithPlaceHolder)
	_, err := m.conn.Exec(query, data.Gid, data.Name, data.Action, data.Compensate, data.LayerNum, data.ActionTriedNum, data.CompensateTriedNum, data.Status, data.Id)
	return err
}

func (m *TransBranchModel) Delete(id int64) error {
	query := fmt.Sprintf("delete from %s where id = ?", m.table)
	_, err := m.conn.Exec(query, id)
	return err
}

func (m *TransBranchModel) FindAllByGid(gid string) ([]*TransBranch, error) {
	query := fmt.Sprintf("select %s from %s where gid = ? ", transBranchRows, m.table)
	var resp []*TransBranch
	err := m.conn.QueryRows(&resp, query, gid)
	switch err {
	case nil:
		return resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *TransBranchModel) UpdateStatusWithSession(s sqlx.Session, data *TransBranch, ver int64) (sql.Result, error) {
	query := fmt.Sprintf("update %s set status = ? ,action_tried_num = ?,compensate_tried_num = ?, version = ? where id = ? and version = ? ", m.table)
	return s.Exec(query, data.Status, data.ActionTriedNum, data.CompensateTriedNum, ver, data.Id, data.Version)
}
