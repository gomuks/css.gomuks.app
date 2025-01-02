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
	"encoding/json"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog/hlog"
	"go.mau.fi/util/exerrors"
	_ "golang.org/x/image/webp"
	"maunium.net/go/mautrix/id"

	"css.gomuks.app/database"
)

type ThemePageData struct {
	Theme   *database.Theme    `json:"theme,omitempty"`
	Themes  []*database.Theme  `json:"themes,omitempty"`
	Commit  *database.Commit   `json:"commit,omitempty"`
	Commits []*database.Commit `json:"commits,omitempty"`
}

func sendResponse(w http.ResponseWriter, r *http.Request, template string, data *ThemePageData) {
	if r.Header.Get("Accept") == "application/json" {
		exerrors.PanicIfNotNil(json.NewEncoder(w).Encode(data))
	} else if r.Header.Get("Accept") == "text/css" && data.Theme != nil {
		w.Header().Set("Content-Type", "text/css")
		if data.Commit != nil {
			_, _ = w.Write([]byte(data.Commit.Content))
		} else {
			_, _ = w.Write([]byte(data.Theme.LatestCommit.Content))
		}
	} else {
		exerrors.PanicIfNotNil(Templates.ExecuteTemplate(w, "container.gohtml", &ContainerData{
			User: verifyCookie(r),
			Page: template,
			Data: data,
		}))
	}
}

func getIndexPage(w http.ResponseWriter, r *http.Request) {
	themes, err := db.Theme.GetAll(r.Context())
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get themes")
		w.WriteHeader(http.StatusInternalServerError)
		// TODO write body
		return
	}
	sendResponse(w, r, "index.gohtml", &ThemePageData{Themes: themes})
}

func getUserPage(w http.ResponseWriter, r *http.Request) {
	userID := id.UserID(r.PathValue("userID"))
	themes, err := db.Theme.GetByAdmin(r.Context(), userID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get themes")
		w.WriteHeader(http.StatusInternalServerError)
		// TODO write body
		return
	}
	sendResponse(w, r, "index.gohtml", &ThemePageData{Themes: themes})
}

func getValueWithSuffix(r *http.Request, key string) string {
	value := r.PathValue(key)
	if strings.HasSuffix(value, ".json") {
		value = value[:len(value)-5]
		r.Header.Set("Accept", "application/json")
	} else if strings.HasSuffix(value, ".css") {
		value = value[:len(value)-4]
		r.Header.Set("Accept", "text/css")
	}
	return value
}

func getThemePage(w http.ResponseWriter, r *http.Request) {
	themeID := database.ThemeID(getValueWithSuffix(r, "themeID"))
	theme, err := db.Theme.Get(r.Context(), themeID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get theme")
		w.WriteHeader(http.StatusInternalServerError)
		// TODO write body
		return
	} else if theme == nil {
		w.WriteHeader(http.StatusNotFound)
		// TODO write body
		return
	}
	var commit *database.Commit
	if versionStr := getValueWithSuffix(r, "version"); versionStr != "" {
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			// TODO write body
			return
		}
		commit, err = db.Commit.Get(r.Context(), themeID, version)
		if err != nil {
			hlog.FromRequest(r).Err(err).Msg("Failed to get commit")
			w.WriteHeader(http.StatusInternalServerError)
			// TODO write body
			return
		} else if commit == nil {
			w.WriteHeader(http.StatusNotFound)
			// TODO write body
			return
		}
	}
	sendResponse(w, r, "theme.gohtml", &ThemePageData{Theme: theme, Commit: commit})
}

func getThemeHistoryPage(w http.ResponseWriter, r *http.Request) {
	themeID := database.ThemeID(r.PathValue("themeID"))
	theme, err := db.Theme.Get(r.Context(), themeID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get theme")
		w.WriteHeader(http.StatusInternalServerError)
		// TODO write body
		return
	} else if theme == nil {
		w.WriteHeader(http.StatusNotFound)
		// TODO write body
		return
	}
	commits, err := db.Commit.GetAll(r.Context(), themeID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get commits")
		w.WriteHeader(http.StatusInternalServerError)
		// TODO write body
		return
	}
	sendResponse(w, r, "theme-history.gohtml", &ThemePageData{Theme: theme, Commits: commits})
}

func getThemeEditPage(w http.ResponseWriter, r *http.Request) {
	userID := verifyCookie(r)
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		// TODO write body
		return
	}
	themeID := database.ThemeID(r.PathValue("themeID"))
	var theme *database.Theme
	if themeID != "" {
		var err error
		theme, err = db.Theme.Get(r.Context(), themeID)
		if err != nil {
			hlog.FromRequest(r).Err(err).Msg("Failed to get theme")
			w.WriteHeader(http.StatusInternalServerError)
			// TODO write body
			return
		} else if theme == nil {
			w.WriteHeader(http.StatusNotFound)
			// TODO write body
			return
		} else if !slices.Contains(theme.Admins, userID) {
			w.WriteHeader(http.StatusForbidden)
			// TODO write body
			return
		}
	}
	sendResponse(w, r, "theme-edit.gohtml", &ThemePageData{Theme: theme})
}
