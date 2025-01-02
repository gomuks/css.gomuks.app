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
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"go.mau.fi/util/dbutil"

	"css.gomuks.app/database/upgrades"
)

type Database struct {
	*dbutil.Database
	Theme        *ThemeQuery
	Commit       *CommitQuery
	PreviewImage *PreviewImageQuery
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
	}, nil
}

func newTheme(_ *dbutil.QueryHelper[*Theme]) *Theme                      { return &Theme{} }
func newCommit(_ *dbutil.QueryHelper[*Commit]) *Commit                   { return &Commit{} }
func newPreviewImage(_ *dbutil.QueryHelper[*PreviewImage]) *PreviewImage { return &PreviewImage{} }
