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

package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/hlog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"

	"css.gomuks.app/database"
)

const commentMaxLength = 8 * 1024

func readThemeAction(w http.ResponseWriter, r *http.Request) (*database.Theme, id.UserID) {
	userID := verifyCookie(w, r)
	if userID == "" {
		return nil, ""
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := r.ParseForm(); err != nil {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Invalid form data"))
		return nil, ""
	}
	themeID := database.ThemeID(r.PathValue("themeID"))
	theme, err := db.Theme.Get(r.Context(), themeID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get theme")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get theme %q", themeID))
		return nil, ""
	} else if theme == nil {
		sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Theme %q not found", themeID))
		return nil, ""
	}
	return theme, userID
}

func postThemeLike(w http.ResponseWriter, r *http.Request) {
	theme, userID := readThemeAction(w, r)
	if theme == nil {
		return
	}
	action := r.PostForm.Get("action")
	if action != "like" && action != "unlike" {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Invalid like action"))
		return
	}
	if err := db.Theme.SetLike(r.Context(), theme.ID, userID, action == "like"); err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to save like")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to save like"))
		return
	}
	http.Redirect(w, r, "/theme/"+string(theme.ID), http.StatusSeeOther)
}

func readCommentText(w http.ResponseWriter, r *http.Request) *string {
	text := strings.TrimSpace(r.PostForm.Get("text"))
	if text == "" || len(text) > commentMaxLength {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Comment must contain between 1 and %d bytes", commentMaxLength))
		return nil
	}
	return &text
}

func postThemeComment(w http.ResponseWriter, r *http.Request) {
	theme, userID := readThemeAction(w, r)
	if theme == nil {
		return
	}
	text := readCommentText(w, r)
	if text == nil {
		return
	}
	comment := &database.Comment{ID: uuid.New(), ThemeID: theme.ID, UserID: userID, Text: text, CreatedAt: time.Now()}
	if replyTo := r.PostForm.Get("reply_to"); replyTo != "" {
		parentID, err := uuid.Parse(replyTo)
		if err != nil {
			sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Invalid reply comment ID"))
			return
		}
		replyTarget, err := db.Comment.Get(r.Context(), theme.ID, parentID)
		if err != nil {
			hlog.FromRequest(r).Err(err).Msg("Failed to get reply comment")
			sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get reply comment"))
			return
		} else if replyTarget == nil {
			sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Reply target comment not found"))
			return
		}
		comment.ReplyTo = &parentID
	}
	err := db.Comment.Add(r.Context(), comment)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to add comment")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to add comment"))
		return
	}
	http.Redirect(w, r, "/theme/"+string(theme.ID)+"#comment-"+comment.ID.String(), http.StatusSeeOther)
}

func postThemeCommentEdit(w http.ResponseWriter, r *http.Request) {
	updateThemeComment(w, r, false)
}

func postThemeCommentDelete(w http.ResponseWriter, r *http.Request) {
	updateThemeComment(w, r, true)
}

func getThemeCommentActionPage(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	var title string
	switch action {
	case "reply":
		title = "Reply to comment"
	case "edit":
		title = "Edit comment"
	case "delete":
		title = "Delete comment"
	default:
		sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Unknown comment action"))
		return
	}
	theme, comment := readCommentAction(w, r, action != "reply")
	if theme == nil {
		return
	}
	sendResponse(w, r, fmt.Sprintf("%s - %s", theme.Name, title), "comment-action", &ThemePageData{
		Theme:         theme,
		Comment:       comment,
		CommentAction: action,
	})
}

func readCommentAction(w http.ResponseWriter, r *http.Request, requireOwner bool) (*database.Theme, *database.Comment) {
	theme, userID := readThemeAction(w, r)
	if theme == nil {
		return nil, nil
	}
	commentID, err := uuid.Parse(r.PathValue("commentID"))
	if err != nil {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Invalid comment ID"))
		return nil, nil
	}
	comment, err := db.Comment.Get(r.Context(), theme.ID, commentID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get comment")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get comment"))
		return nil, nil
	} else if comment == nil || (requireOwner && comment.Text == nil) {
		sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Comment not found"))
		return nil, nil
	} else if requireOwner && comment.UserID != userID {
		sendErrorResponse(w, r, mautrix.MForbidden.WithMessage("You can only change your own comments"))
		return nil, nil
	}
	return theme, comment
}

func updateThemeComment(w http.ResponseWriter, r *http.Request, deleteComment bool) {
	theme, comment := readCommentAction(w, r, true)
	if theme == nil {
		return
	}

	var text *string
	if !deleteComment {
		text = readCommentText(w, r)
		if text == nil {
			return
		}
	}
	err := db.Comment.Update(r.Context(), theme.ID, comment.ID, comment.UserID, text)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to update comment")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to update comment"))
		return
	}
	http.Redirect(w, r, "/theme/"+string(theme.ID)+"#comment-"+comment.ID.String(), http.StatusSeeOther)
}

type CommentThread struct {
	*database.Comment
	User    id.UserID
	Replies []*CommentThread
}

func makeCommentThreads(comments []*database.Comment, userID id.UserID) []*CommentThread {
	byID := make(map[uuid.UUID]*CommentThread, len(comments))
	for _, comment := range comments {
		byID[comment.ID] = &CommentThread{Comment: comment, User: userID}
	}
	var roots []*CommentThread
	for _, comment := range comments {
		thread := byID[comment.ID]
		if comment.ReplyTo != nil && byID[*comment.ReplyTo] != nil {
			parent := byID[*comment.ReplyTo]
			parent.Replies = append(parent.Replies, thread)
		} else {
			roots = append(roots, thread)
		}
	}
	return roots
}
