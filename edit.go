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
	"maunium.net/go/mautrix/id"

	"css.gomuks.app/database"
)

var themeIDRegex = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)

const nameMaxLength = 64
const descriptionMaxLength = 8 * 1024
const contentMaxLength = 128 * 1024
const maxPreviewSize = 512 * 1024
const maxPreviewCount = 8

func postThemeEditPage(w http.ResponseWriter, r *http.Request) {
	log := hlog.FromRequest(r)
	userID := verifyCookie(r)
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		// TODO write body
		return
	}
	err := r.ParseMultipartForm(5 * 1024 * 1024)
	if err != nil {
		log.Err(err).Msg("Failed to parse form")
		// TODO write body
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	themeID := database.ThemeID(r.Form.Get("theme_id"))
	if themeID == "new" || themeID == "commit" || !themeIDRegex.MatchString(string(themeID)) {
		w.WriteHeader(http.StatusBadRequest)
		// TODO write body
		return
	}
	commitVersion, err := strconv.Atoi(r.Form.Get("commit_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		// TODO write body
		return
	}
	themeName := r.Form.Get("name")
	if len(themeName) > nameMaxLength {
		w.WriteHeader(http.StatusBadRequest)
		// TODO write body
		return
	}
	themeDescription := r.Form.Get("description")
	if len(themeDescription) > descriptionMaxLength {
		w.WriteHeader(http.StatusBadRequest)
		// TODO write body
		return
	}
	commitContent := r.Form.Get("content")
	if len(commitContent) > contentMaxLength {
		w.WriteHeader(http.StatusBadRequest)
		// TODO write body
		return
	}
	commitMessage := r.Form.Get("message")
	if len(commitMessage) > descriptionMaxLength {
		w.WriteHeader(http.StatusBadRequest)
		// TODO write body
		return
	}
	var newPreviews []*database.PreviewImage
	var removedPreviews []uuid.UUID
	for _, preview := range r.MultipartForm.File["preview"] {
		if preview.Size > maxPreviewSize {
			w.WriteHeader(http.StatusBadRequest)
			// TODO write body
			return
		}
		file, err := preview.Open()
		if err != nil {
			log.Err(err).Msg("Failed to open preview file")
			// TODO write body
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(file)
		if err != nil {
			log.Err(err).Msg("Failed to read file")
			// TODO write body
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			log.Err(err).Msg("Failed to decode image config")
			// TODO write body
			w.WriteHeader(http.StatusBadRequest)
			return
		} else if format != "png" && format != "jpeg" && format != "webp" {
			log.Err(err).Msg("Invalid image format")
			// TODO write body
			w.WriteHeader(http.StatusBadRequest)
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
			return err
		} else if theme == nil && commitVersion != 1 {
			return fmt.Errorf("theme not found")
		} else if theme != nil && commitVersion != theme.LatestCommit.Version+1 {
			return fmt.Errorf("invalid commit version")
		}
		if theme == nil || theme.Name != themeName || theme.Description != themeDescription {
			if theme == nil {
				theme = &database.Theme{
					ID:          themeID,
					Name:        themeName,
					Description: themeDescription,
					Admins:      []id.UserID{userID},
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
				theme.Description = themeDescription
				theme.Name = themeName
				err = db.Theme.Update(ctx, theme)
				if err != nil {
					return fmt.Errorf("failed to update theme: %w", err)
				}
			}
		}
		if len(newPreviews)+len(theme.Previews) > maxPreviewCount {
			return fmt.Errorf("too many previews")
		}
		commit := &database.Commit{
			ThemeID:   theme.ID,
			Version:   commitVersion,
			Message:   commitMessage,
			CreatedAt: time.Now(),
			CreatedBy: userID,
			Content:   commitContent,
		}
		err = db.Commit.Add(ctx, commit)
		if err != nil {
			return fmt.Errorf("failed to add commit: %w", err)
		}
		theme.LatestCommit = *commit
		err = db.Theme.SetLatestCommit(ctx, theme.ID, commit.Version)
		if err != nil {
			return fmt.Errorf("failed to update latest theme commit: %w", err)
		}
		for _, preview := range newPreviews {
			err = db.PreviewImage.Add(ctx, preview)
			if err != nil {
				return fmt.Errorf("failed to add preview image: %w", err)
			}
			theme.Previews = append(theme.Previews, preview.ID)
		}
		for _, previewID := range removedPreviews {
			err = db.PreviewImage.Delete(ctx, previewID)
			if err != nil {
				return fmt.Errorf("failed to delete preview image: %w", err)
			}
			theme.Previews = slices.DeleteFunc(theme.Previews, func(u uuid.UUID) bool {
				return u == previewID
			})
		}
		return nil
	})
	if err != nil {
		log.Err(err).Msg("Failed to save theme")
		// TODO write body
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/theme/%s", themeID))
	w.WriteHeader(http.StatusSeeOther)
}
