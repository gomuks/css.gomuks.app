// css.gomuks.app - A user CSS repository for gomuks web.
// Copyright (C) 2024 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package database

import (
	"context"
	"time"

	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix/id"
)

const (
	getAllCommitsQuery = `
		SELECT theme_id, version, message, created_at, created_by, content
		FROM commit
		WHERE theme_id = $1
	`
	getCommitQuery = getAllCommitsQuery + `AND version = $2`
	addCommitQuery = `
		INSERT INTO commit (theme_id, version, message, created_at, created_by, content)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
)

type CommitQuery struct {
	*dbutil.QueryHelper[*Commit]
}

func (cq *CommitQuery) GetAll(ctx context.Context, themeID ThemeID) ([]*Commit, error) {
	return cq.QueryMany(ctx, getAllCommitsQuery, themeID)
}

func (cq *CommitQuery) Get(ctx context.Context, themeID ThemeID, version int) (*Commit, error) {
	return cq.QueryOne(ctx, getCommitQuery, themeID, version)
}

func (cq *CommitQuery) Add(ctx context.Context, commit *Commit) error {
	return cq.Exec(ctx, addCommitQuery, commit.sqlVariables()...)
}

type Commit struct {
	ThemeID   ThemeID   `json:"theme_id"`
	Version   int       `json:"version"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy id.UserID `json:"created_by"`
	Content   string    `json:"content"`
}

func (c *Commit) Scan(row dbutil.Scannable) (*Commit, error) {
	return dbutil.ValueOrErr(c, row.Scan(&c.ThemeID, &c.Version, &c.Message, &c.CreatedAt, &c.CreatedBy, &c.Content))
}

func (c *Commit) sqlVariables() []any {
	return []any{c.ThemeID, c.Version, c.Message, c.CreatedAt, c.CreatedBy, c.Content}
}
