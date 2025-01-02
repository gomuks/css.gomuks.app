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

	"github.com/google/uuid"
	"go.mau.fi/util/dbutil"
	"maunium.net/go/mautrix/id"
)

const (
	getPreviewImageQuery = `
		SELECT image_id, theme_id, created_at, created_by, width, height, mime_type, content
		FROM preview_image
		WHERE image_id = $1
	`
	addPreviewImageQuery = `
		INSERT INTO preview_image (image_id, theme_id, created_at, created_by, width, height, mime_type, content)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	deletePreviewImageQuery = `
		DELETE FROM preview_image WHERE image_id = $1
	`
)

type PreviewImageQuery struct {
	*dbutil.QueryHelper[*PreviewImage]
}

func (piq *PreviewImageQuery) Get(ctx context.Context, imageID uuid.UUID) (*PreviewImage, error) {
	return piq.QueryOne(ctx, getPreviewImageQuery, imageID)
}

func (piq *PreviewImageQuery) Add(ctx context.Context, image *PreviewImage) error {
	return piq.Exec(ctx, addPreviewImageQuery, image.sqlVariables()...)
}

func (piq *PreviewImageQuery) Delete(ctx context.Context, imageID uuid.UUID) error {
	return piq.Exec(ctx, deletePreviewImageQuery, imageID)
}

type PreviewImage struct {
	ID        uuid.UUID `json:"id"`
	ThemeID   ThemeID   `json:"theme_id"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy id.UserID `json:"created_by"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	MimeType  string    `json:"mime_type"`
	Content   []byte    `json:"content"`
}

func (pi *PreviewImage) Scan(row dbutil.Scannable) (*PreviewImage, error) {
	return dbutil.ValueOrErr(pi, row.Scan(
		&pi.ID, &pi.ThemeID, &pi.CreatedAt, &pi.CreatedBy, &pi.Width, &pi.Height, &pi.MimeType, &pi.Content,
	))
}

func (pi *PreviewImage) sqlVariables() []any {
	return []any{pi.ID, pi.ThemeID, pi.CreatedAt, pi.CreatedBy, pi.Width, pi.Height, pi.MimeType, pi.Content}
}
