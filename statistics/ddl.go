// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package statistics

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/util/sqlexec"
)

type ddlType int

const (
	createTable ddlType = iota
	dropTable
	createColumn
	dropColumn
)

// DDLTask is a task describing how we should update the stats table after a ddl operation has done.
type DDLTask struct {
	tp        ddlType
	tableInfo *model.TableInfo
}

// DDLCh return the ddl task channel.
func (h *Handle) DDLCh() <-chan *DDLTask {
	return h.ddlCh
}

// CreateTable creates a ddl task and pushes to channel. A worker will read the channel and updates the stats table.
func (h *Handle) CreateTable(tableInfo *model.TableInfo) {
	h.ddlCh <- &DDLTask{tp: createTable, tableInfo: tableInfo}
}

// HandleDDLTask begins to process a ddl task.
func (h *Handle) HandleDDLTask(t *DDLTask) error {
	switch t.tp {
	case createTable:
		return h.onCreateTable(t.tableInfo)
	}
	return nil
}

func (h *Handle) onCreateTable(info *model.TableInfo) error {
	_, err := h.ctx.(sqlexec.SQLExecutor).Execute("begin")
	if err != nil {
		return errors.Trace(err)
	}
	_, err = h.ctx.(sqlexec.SQLExecutor).Execute(fmt.Sprintf("insert into mysql.stats_meta (version, table_id) values(%d, %d)", h.ctx.Txn().StartTS(), info.ID))
	if err != nil {
		return errors.Trace(err)
	}
	for _, col := range info.Columns {
		_, err = h.ctx.(sqlexec.SQLExecutor).Execute(fmt.Sprintf("insert into mysql.stats_histograms (table_id, is_index, hist_id, distinct_count, version) values(%d, false, %d, 0, %d)", info.ID, col.ID, h.ctx.Txn().StartTS()))
		if err != nil {
			return errors.Trace(err)
		}
	}
	for _, idx := range info.Indices {
		_, err = h.ctx.(sqlexec.SQLExecutor).Execute(fmt.Sprintf("insert into mysql.stats_histograms (table_id, is_index, hist_id, distinct_count, version) values(%d, true, %d, 0, %d)", info.ID, idx.ID, h.ctx.Txn().StartTS()))
		if err != nil {
			return errors.Trace(err)
		}
	}
	_, err = h.ctx.(sqlexec.SQLExecutor).Execute("commit")
	return errors.Trace(err)
}
