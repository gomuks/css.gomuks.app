// css.gomuks.app - A user CSS repository for gomuks web.
// Copyright (C) 2026 Tulir Asokan
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

const commentColumns = `comment_id, theme_id, CASE WHEN text IS NULL THEN '' ELSE user_id END, created_at, edited_at, text, reply_to`

const (
	getCommentsBaseQuery = `SELECT ` + commentColumns + ` FROM comment WHERE theme_id = $1`
	getCommentsQuery     = getCommentsBaseQuery + ` ORDER BY created_at ASC`
	getCommentQuery      = getCommentsBaseQuery + ` AND comment_id = $2`

	insertCommentQuery = `
		INSERT INTO comment (comment_id, theme_id, user_id, created_at, text, reply_to)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	editCommentQuery = `
		UPDATE comment SET text = $4, edited_at = CURRENT_TIMESTAMP
		WHERE theme_id = $1 AND comment_id = $2 AND user_id = $3 AND text IS NOT NULL
	`
)

type CommentQuery struct {
	*dbutil.QueryHelper[*Comment]
}

func (cq *CommentQuery) GetAll(ctx context.Context, themeID ThemeID) ([]*Comment, error) {
	return cq.QueryMany(ctx, getCommentsQuery, themeID)
}

func (cq *CommentQuery) Get(ctx context.Context, themeID ThemeID, commentID uuid.UUID) (*Comment, error) {
	return cq.QueryOne(ctx, getCommentQuery, themeID, commentID)
}

func (cq *CommentQuery) Add(ctx context.Context, comment *Comment) error {
	return cq.Exec(ctx, insertCommentQuery, comment.ID, comment.ThemeID, comment.UserID, comment.CreatedAt, comment.Text, comment.ReplyTo)
}

func (cq *CommentQuery) Update(ctx context.Context, themeID ThemeID, commentID uuid.UUID, userID id.UserID, text *string) error {
	return cq.Exec(ctx, editCommentQuery, themeID, commentID, userID, text)
}

type Comment struct {
	ID        uuid.UUID  `json:"id"`
	ThemeID   ThemeID    `json:"theme_id"`
	UserID    id.UserID  `json:"user_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	EditedAt  *time.Time `json:"edited_at,omitempty"`
	Text      *string    `json:"text"`
	ReplyTo   *uuid.UUID `json:"reply_to,omitempty"`
}

func (c *Comment) Scan(row dbutil.Scannable) (*Comment, error) {
	return dbutil.ValueOrErr(c, row.Scan(&c.ID, &c.ThemeID, &c.UserID, &c.CreatedAt, &c.EditedAt, &c.Text, &c.ReplyTo))
}
