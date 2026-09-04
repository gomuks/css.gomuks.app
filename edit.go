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

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/hlog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"

	"css.gomuks.app/database"
)

var themeIDRegex = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

const nameMaxLength = 64
const descriptionMaxLength = 8 * 1024
const contentMaxLength = 128 * 1024
const maxPreviewSize = 512 * 1024
const maxPreviewCount = 8

var (
	ErrBadFormData = mautrix.RespError{StatusCode: http.StatusBadRequest, ErrCode: "APP.GOMUKS.CSS.BAD_FORM_DATA", Err: "Invalid multipart form data"}
	ErrConflict    = mautrix.RespError{StatusCode: http.StatusConflict, ErrCode: "APP.GOMUKS.CSS.CONFLICT", Err: "Commit version conflict, did someone else update the theme?"}
)

func postThemeEditPage(w http.ResponseWriter, r *http.Request) {
	log := hlog.FromRequest(r)
	userID := verifyCookie(w, r)
	if userID == "" {
		return
	}
	err := r.ParseMultipartForm(5 * 1024 * 1024)
	if err != nil {
		log.Err(err).Msg("Failed to parse form")
		sendErrorResponse(w, r, ErrBadFormData)
		return
	}
	themeID := database.ThemeID(r.Form.Get("theme_id"))
	if themeID == "new" || themeID == "commit" || !themeIDRegex.MatchString(string(themeID)) {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Invalid theme ID %q", themeID))
		return
	}
	commitVersion, err := strconv.Atoi(r.Form.Get("commit_id"))
	if err != nil {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Invalid commit ID"))
		return
	}
	themeName := r.Form.Get("name")
	if len(themeName) > nameMaxLength {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Too long theme name (max %d)", nameMaxLength))
		return
	}
	if themeName == "" {
		themeName = string(themeID)
	}
	themeDescription := r.Form.Get("description")
	if len(themeDescription) > descriptionMaxLength {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Too long description (max %d)", descriptionMaxLength))
		return
	}
	commitContent := r.Form.Get("content")
	if len(commitContent) > contentMaxLength {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Too long content (max %d)", contentMaxLength))
		return
	}
	commitMessage := r.Form.Get("message")
	if len(commitMessage) > descriptionMaxLength {
		sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Too long commit message (max %d)", descriptionMaxLength))
		return
	}
	var newPreviews []*database.PreviewImage
	var removedPreviews []uuid.UUID
	for _, deletedPreview := range r.Form["delete_preview"] {
		previewID, err := uuid.Parse(deletedPreview)
		if err == nil {
			removedPreviews = append(removedPreviews, previewID)
		}
	}
	for _, preview := range r.MultipartForm.File["preview"] {
		if preview.Size > maxPreviewSize {
			sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Too large preview image (max %d KiB)", maxPreviewSize/1024))
			return
		}
		file, err := preview.Open()
		if err != nil {
			log.Err(err).Msg("Failed to open file")
			sendErrorResponse(w, r, ErrBadFormData.WithMessage("Failed to open preview image"))
			return
		}
		data, err := io.ReadAll(file)
		if err != nil {
			log.Err(err).Msg("Failed to read file")
			sendErrorResponse(w, r, ErrBadFormData.WithMessage("Failed to open preview image"))
			return
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			log.Err(err).Msg("Failed to decode image config")
			sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Invalid preview image file"))
			return
		} else if format != "png" && format != "jpeg" && format != "webp" {
			log.Err(err).Msg("Invalid image format")
			sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Unsupported preview image format %q", format))
			return
		}
		newPreviews = append(newPreviews, &database.PreviewImage{
			ID:        uuid.New(),
			ThemeID:   themeID,
			CreatedAt: time.Now(),
			CreatedBy: userID,
			Width:     cfg.Width,
			Height:    cfg.Height,
			MimeType:  "image/" + format,
			Content:   data,
		})
	}
	var theme *database.Theme
	err = db.DoTxn(r.Context(), nil, func(ctx context.Context) error {
		var err error
		theme, err = db.Theme.Get(r.Context(), themeID)
		if err != nil {
			log.Err(err).Msg("Failed to get theme")
			return mautrix.MUnknown.WithMessage("Failed to get theme %q", themeID)
		} else if theme == nil && commitVersion != 1 {
			return mautrix.MNotFound.WithMessage("Theme %q not found", themeID)
		} else if theme != nil {
			if commitVersion != theme.LatestCommit.Version+1 {
				return ErrConflict
			} else if !slices.Contains(theme.Admins, userID) {
				return mautrix.MForbidden.WithMessage("You're not an admin of %q", themeID)
			}
		}
		if theme == nil || theme.Name != themeName {
			if theme == nil {
				theme = &database.Theme{
					ID:           themeID,
					Name:         themeName,
					Admins:       []id.UserID{userID},
					LatestCommit: &database.Commit{},
				}
				err = db.Theme.Create(ctx, theme)
				if err != nil {
					return fmt.Errorf("failed to create theme: %w", err)
				}
				err = db.Theme.AddAdmin(ctx, theme.ID, userID)
				if err != nil {
					return fmt.Errorf("failed to add theme admin: %w", err)
				}
			} else {
				theme.Name = themeName
				err = db.Theme.Update(ctx, theme)
				if err != nil {
					return fmt.Errorf("failed to update theme: %w", err)
				}
			}
		}
		commit := &database.Commit{
			ThemeID:     theme.ID,
			Version:     commitVersion,
			Message:     commitMessage,
			Description: themeDescription,
			CreatedAt:   time.Now(),
			CreatedBy:   userID,
			Content:     commitContent,
		}
		commit.Previews = slices.Clone(theme.LatestCommit.Previews)
		if commit.Previews == nil {
			commit.Previews = []uuid.UUID{}
		}
		commit.Previews = slices.DeleteFunc(commit.Previews, func(p uuid.UUID) bool {
			return slices.Contains(removedPreviews, p)
		})
		if len(commit.Previews)+len(newPreviews) > maxPreviewCount {
			return fmt.Errorf("too many previews")
		}
		for _, preview := range newPreviews {
			err = db.PreviewImage.Add(ctx, preview)
			if err != nil {
				return fmt.Errorf("failed to add preview image: %w", err)
			}
			commit.Previews = append(commit.Previews, preview.ID)
		}
		firstPreviewID, _ := uuid.Parse(r.Form.Get("first_preview"))
		if firstPreviewID != uuid.Nil {
			wantedFirstPreviewIndex := slices.IndexFunc(commit.Previews, func(p uuid.UUID) bool {
				return p == firstPreviewID
			})
			if wantedFirstPreviewIndex > 0 {
				copy(commit.Previews[1:wantedFirstPreviewIndex+1], commit.Previews[0:wantedFirstPreviewIndex])
				commit.Previews[0] = firstPreviewID
			}
		}
		err = db.Commit.Add(ctx, commit)
		if err != nil {
			return fmt.Errorf("failed to add commit: %w", err)
		}
		theme.LatestCommit = commit
		err = db.Theme.SetLatestCommit(ctx, theme.ID, commit.Version)
		if err != nil {
			return fmt.Errorf("failed to update latest theme commit: %w", err)
		}
		return nil
	})
	if err != nil {
		var respErr mautrix.RespError
		if errors.As(err, &respErr) {
			sendErrorResponse(w, r, respErr)
		} else {
			log.Err(err).Msg("Failed to save theme")
			sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to save theme %q", themeID))
		}
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/theme/%s", themeID))
	w.WriteHeader(http.StatusSeeOther)
}
