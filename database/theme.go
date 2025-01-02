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
	"slices"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.mau.fi/util/dbutil"
	"go.mau.fi/util/exslices"
	"maunium.net/go/mautrix/id"
)

const (
	getAllThemesQuery = `
		SELECT
			id, name, description,
			COALESCE(commit.version, 0), commit.created_at, COALESCE(commit.created_by, ''), COALESCE(commit.content, ''),
			ARRAY(SELECT user_id FROM admin WHERE theme_id = theme.id),
			ARRAY(SELECT image_id FROM preview_image WHERE theme_id = theme.id)
		FROM theme
		LEFT JOIN commit ON theme.id = commit.theme_id AND theme.last_commit = commit.version
	`
	getThemeByIDQuery     = getAllThemesQuery + `WHERE id = $1`
	getThemesByAdminQuery = getAllThemesQuery + `INNER JOIN admin ON theme.id = admin.theme_id AND admin.user_id = $1`
	createThemeQuery      = `
		INSERT INTO theme (id, name, description, last_commit)
		VALUES ($1, $2, $3, $4)
	`
	updateThemeQuery = `
		UPDATE theme SET name = $2, description = $3 WHERE id = $1
	`
	setLatestThemeCommitQuery = `UPDATE theme SET last_commit = $2 WHERE id = $1`
	deleteThemeQuery          = `
		DELETE FROM theme WHERE id = $1
	`
	addThemeAdminQuery = `
		INSERT INTO admin (theme_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	removeThemeAdminQuery = `
		DELETE FROM admin WHERE theme_id = $1 AND user_id = $2
	`
)

type ThemeQuery struct {
	*dbutil.QueryHelper[*Theme]
}

func (tq *ThemeQuery) Get(ctx context.Context, id ThemeID) (*Theme, error) {
	return tq.QueryOne(ctx, getThemeByIDQuery, id)
}

func (tq *ThemeQuery) GetAll(ctx context.Context) ([]*Theme, error) {
	return tq.QueryMany(ctx, getAllThemesQuery)
}

func (tq *ThemeQuery) GetByAdmin(ctx context.Context, admin id.UserID) ([]*Theme, error) {
	return tq.QueryMany(ctx, getThemesByAdminQuery, admin)
}

func (tq *ThemeQuery) Create(ctx context.Context, theme *Theme) error {
	return tq.Exec(ctx, createThemeQuery, theme.sqlVariables()...)
}

func (tq *ThemeQuery) Update(ctx context.Context, theme *Theme) error {
	return tq.Exec(ctx, updateThemeQuery, theme.sqlVariables()...)
}

func (tq *ThemeQuery) SetLatestCommit(ctx context.Context, themeID ThemeID, latestCommit int) error {
	return tq.Exec(ctx, setLatestThemeCommitQuery, themeID, latestCommit)
}

func (tq *ThemeQuery) Delete(ctx context.Context, id ThemeID) error {
	return tq.Exec(ctx, deleteThemeQuery, id)
}

func (tq *ThemeQuery) AddAdmin(ctx context.Context, themeID ThemeID, adminID id.UserID) error {
	return tq.Exec(ctx, addThemeAdminQuery, themeID, adminID)
}

func (tq *ThemeQuery) RemoveAdmin(ctx context.Context, themeID ThemeID, adminID id.UserID) error {
	return tq.Exec(ctx, removeThemeAdminQuery, themeID, adminID)
}

type ThemeID string

type Theme struct {
	ID          ThemeID `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`

	LatestCommit Commit      `json:"latest_commit"`
	Admins       []id.UserID `json:"admins,omitempty"`
	Previews     []uuid.UUID `json:"previews,omitempty"`
}

func (t *Theme) Scan(row dbutil.Scannable) (*Theme, error) {
	var admins []string
	err := row.Scan(
		&t.ID, &t.Name, &t.Description,
		&t.LatestCommit.Version, &t.LatestCommit.CreatedAt, &t.LatestCommit.CreatedBy, &t.LatestCommit.Content,
		pq.Array(&admins), pq.Array(&t.Previews),
	)
	if err != nil {
		return nil, err
	}
	t.Admins = exslices.CastToString[id.UserID](admins)
	return t, nil
}

func (t *Theme) sqlVariables() []any {
	var lastCommitID *int
	if t.LatestCommit.Version > 0 {
		lastCommitID = &t.LatestCommit.Version
	}
	return []any{t.ID, t.Name, t.Description, lastCommitID}
}

func (t *Theme) IsAdmin(userID id.UserID) bool {
	return slices.Contains(t.Admins, userID)
}
