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

	"github.com/rs/zerolog/hlog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"

	"css.gomuks.app/database"
)

const reportMaxLength = 1024

func readReportTarget(w http.ResponseWriter, r *http.Request) (*database.Theme, *database.Comment, id.UserID) {
	if r.PathValue("commentID") != "" {
		theme, comment := readCommentAction(w, r, false)
		if theme == nil {
			return nil, nil, ""
		} else if comment.Text == nil {
			sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Comment not found"))
			return nil, nil, ""
		}
		return theme, comment, comment.UserID
	}
	theme, _ := readThemeAction(w, r)
	if theme == nil {
		return nil, nil, ""
	} else if len(theme.Admins) == 0 {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Theme has no admins to report"))
		return nil, nil, ""
	}
	return theme, nil, theme.Admins[0]
}

func getReportPage(w http.ResponseWriter, r *http.Request) {
	theme, comment, _ := readReportTarget(w, r)
	if theme == nil {
		return
	}
	title := "Report theme"
	if comment != nil {
		title = "Report comment"
	}
	sendResponse(w, r, fmt.Sprintf("%s - %s", theme.Name, title), "report", &ThemePageData{Theme: theme, Comment: comment})
}

func postReport(w http.ResponseWriter, r *http.Request) {
	theme, comment, target := readReportTarget(w, r)
	if theme == nil {
		return
	}
	reason := r.PostForm.Get("reason")
	if strings.TrimSpace(reason) == "" || len(reason) > reportMaxLength {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Report reason must contain between 1 and %d bytes", reportMaxLength))
		return
	}
	reason = strings.ReplaceAll(strings.ReplaceAll(reason, "\r\n", "\n"), "\r", "\n")
	path := "/theme/" + string(theme.ID)
	kind := "theme"
	if comment != nil {
		path += "#comment-" + comment.ID.String()
		kind = "comment"
	}
	reason = fmt.Sprintf("%s reported a %s: https://css.gomuks.app%s\n\n> %s", readCookie(r), kind, path, strings.ReplaceAll(reason, "\n", "\n> "))
	if err := matrixClient.ReportUser(r.Context(), target, reason); err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to send report")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to send report"))
		return
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}
