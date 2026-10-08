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
	"fmt"
	"sync/atomic"

	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix/id"

	"css.gomuks.app/database/upgrades"
)

type Database struct {
	*dbutil.Database
	Theme        *ThemeQuery
	Commit       *CommitQuery
	PreviewImage *PreviewImageQuery
	Comment      *CommentQuery

	contentUserCache atomic.Pointer[[]id.UserID]
}

func New(uri string, log zerolog.Logger) (*Database, error) {
	db, err := dbutil.NewWithDialect(uri, "postgres")
	if err != nil {
		return nil, err
	}
	db.Owner = "css.gomuks.app"
	db.Log = dbutil.ZeroLogger(log)
	db.UpgradeTable = upgrades.Table
	return &Database{
		Database:     db,
		Theme:        &ThemeQuery{dbutil.MakeQueryHelper(db, newTheme)},
		Commit:       &CommitQuery{dbutil.MakeQueryHelper(db, newCommit)},
		PreviewImage: &PreviewImageQuery{dbutil.MakeQueryHelper(db, newPreviewImage)},
		Comment:      &CommentQuery{dbutil.MakeQueryHelper(db, newComment)},
	}, nil
}

func newTheme(_ *dbutil.QueryHelper[*Theme]) *Theme                      { return &Theme{} }
func newCommit(_ *dbutil.QueryHelper[*Commit]) *Commit                   { return &Commit{} }
func newPreviewImage(_ *dbutil.QueryHelper[*PreviewImage]) *PreviewImage { return &PreviewImage{} }
func newComment(_ *dbutil.QueryHelper[*Comment]) *Comment                { return &Comment{} }

func (db *Database) ClearContentUserCache() {
	db.contentUserCache.Store(nil)
}

func (db *Database) GetContentUsers(ctx context.Context) ([]id.UserID, error) {
	if cached := db.contentUserCache.Load(); cached != nil {
		return *cached, nil
	}
	rows, err := db.Query(ctx, `SELECT user_id FROM admin UNION SELECT user_id FROM comment WHERE text IS NOT NULL`)
	res, err := dbutil.NewRowIterWithError(rows, func(row dbutil.Scannable) (id.UserID, error) {
		var userID id.UserID
		err := row.Scan(&userID)
		return userID, err
	}, err).AsList()
	if err != nil {
		return nil, err
	}
	db.contentUserCache.Store(&res)
	return res, nil
}

func (db *Database) RemoveUserContent(ctx context.Context, userID id.UserID) error {
	return db.DoTxn(ctx, nil, func(ctx context.Context) error {
		for _, query := range []string{
			`UPDATE theme SET last_commit = NULL WHERE id IN (SELECT theme_id FROM admin WHERE user_id = $1)`,
			`DELETE FROM theme WHERE id IN (SELECT theme_id FROM admin WHERE user_id = $1)`,
			`UPDATE comment SET text = NULL, edited_at = CURRENT_TIMESTAMP WHERE user_id = $1 AND text IS NOT NULL`,
		} {
			if _, err := db.Exec(ctx, query, userID); err != nil {
				return fmt.Errorf("failed to remove content of %s: %w", userID, err)
			}
		}
		return nil
	})
}
