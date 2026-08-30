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
	"cmp"
	"encoding/json"
	_ "image/jpeg"
	_ "image/png"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog/hlog"
	"go.mau.fi/util/exerrors"
	_ "golang.org/x/image/webp"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"

	"css.gomuks.app/database"
)

type ThemePageData struct {
	Theme   *database.Theme    `json:"theme,omitempty"`
	Themes  []*database.Theme  `json:"themes,omitempty"`
	Commit  *database.Commit   `json:"commit,omitempty"`
	Commits []*database.Commit `json:"commits,omitempty"`

	StatusCode int    `json:"-"`
	ErrCode    string `json:"errcode,omitempty"`
	Error      string `json:"error,omitempty"`
}

func sendErrorResponse(w http.ResponseWriter, r *http.Request, error mautrix.RespError) {
	sendResponse(w, r, "", "error", &ThemePageData{
		StatusCode: error.StatusCode,
		ErrCode:    error.ErrCode,
		Error:      error.Err,
	})
}

func sendResponse(w http.ResponseWriter, r *http.Request, pageTitle, template string, data *ThemePageData) {
	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if (data.Commit != nil || data.Commits != nil) && data.Theme != nil {
			data.Theme.LatestCommit = nil
		}
		if template == "error" {
			w.WriteHeader(data.StatusCode)
		}
		exerrors.PanicIfNotNil(json.NewEncoder(w).Encode(data))
	} else if r.Header.Get("Accept") == "text/css" {
		if data.Theme == nil || data.Commits != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var fresh bool
		if strings.Trim(r.Header.Get("If-None-Match"), "\"") == strconv.Itoa(data.Theme.LatestCommit.Version) {
			fresh = true
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
		if data.Commit != nil {
			w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
			if fresh {
				w.WriteHeader(http.StatusNotModified)
			} else {
				_, _ = w.Write([]byte(data.Commit.Content))
			}
		} else {
			w.Header().Set("Cache-Control", "public,max-age=3600,stale-if-error=604800")
			w.Header().Set("ETag", fmt.Sprintf(`"%d"`, data.Theme.LatestCommit.Version))
			if fresh {
				w.WriteHeader(http.StatusNotModified)
			} else {
				_, _ = w.Write([]byte(data.Theme.LatestCommit.Content))
			}
		}
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if template == "error" {
			w.WriteHeader(data.StatusCode)
		}
		exerrors.PanicIfNotNil(Templates.ExecuteTemplate(w, "container.gohtml", &ContainerData{
			User:      readCookie(r),
			PageTitle: pageTitle,
			Page:      template,
			Data:      data,
		}))
	}
}

func getIndexPage(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, ".json") {
		r.Header.Set("Accept", "application/json")
	}
	themes, err := db.Theme.GetAll(r.Context())
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get themes")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get all themes"))
		return
	}
	sortThemes(themes, r.URL.Query().Get("sort"), r.URL.Query().Get("dir"))
	sendResponse(w, r, "", "index", &ThemePageData{Themes: themes})
}

func sortThemes(themes []*database.Theme, sortBy, direction string) {
	switch sortBy {
	case "create":
		slices.SortFunc(themes, func(a, b *database.Theme) int {
			return a.CreatedAt.Compare(b.CreatedAt)
		})
	case "update":
		slices.SortFunc(themes, func(a, b *database.Theme) int {
			return a.LatestCommit.CreatedAt.Compare(b.LatestCommit.CreatedAt)
		})
	case "alpha":
		slices.SortFunc(themes, func(a, b *database.Theme) int {
			return cmp.Compare(a.Name, b.Name)
		})
	case "", "random":
		rand.Shuffle(len(themes), func(i, j int) {
			themes[i], themes[j] = themes[j], themes[i]
		})
	}
	if direction == "desc" {
		slices.Reverse(themes)
	}
}

func getUserPage(w http.ResponseWriter, r *http.Request) {
	userID := id.UserID(getValueWithSuffix(r, "userID"))
	themes, err := db.Theme.GetByAdmin(r.Context(), userID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get themes")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get themes of user %q", userID))
		return
	}
	sortThemes(themes, r.URL.Query().Get("sort"), r.URL.Query().Get("dir"))
	sendResponse(w, r, string(userID), "index", &ThemePageData{Themes: themes})
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
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get theme %q", themeID))
		return
	} else if theme == nil {
		sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Theme %q not found", themeID))
		return
	}
	var commit *database.Commit
	title := theme.Name
	if versionStr := getValueWithSuffix(r, "version"); versionStr != "" {
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			sendErrorResponse(w, r, mautrix.MInvalidParam.WithMessage("Invalid version value %q", versionStr))
			return
		}
		commit, err = db.Commit.Get(r.Context(), themeID, version)
		if err != nil {
			hlog.FromRequest(r).Err(err).Msg("Failed to get commit")
			sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get commit %d of theme %q", version, themeID))
			return
		} else if commit == nil {
			sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Commit %d of theme %q not found", version, themeID))
			return
		}
		title += " - v" + versionStr
	}
	sendResponse(w, r, title, "theme", &ThemePageData{Theme: theme, Commit: commit})
}

func getThemeHistoryPage(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, ".json") {
		r.Header.Set("Accept", "application/json")
	}
	themeID := database.ThemeID(r.PathValue("themeID"))
	theme, err := db.Theme.Get(r.Context(), themeID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get theme")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get theme %q", themeID))
		return
	} else if theme == nil {
		sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Theme %q not found", themeID))
		return
	}
	commits, err := db.Commit.GetAll(r.Context(), themeID)
	if err != nil {
		hlog.FromRequest(r).Err(err).Msg("Failed to get commits")
		sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get commits of theme %q", themeID))
		return
	}
	for _, commit := range commits {
		commit.ThemeID = ""
	}
	sendResponse(w, r, theme.Name+" - history", "theme-history", &ThemePageData{Theme: theme, Commits: commits})
}

func getThemeEditPage(w http.ResponseWriter, r *http.Request) {
	userID := verifyCookie(w, r)
	if userID == "" {
		return
	}
	themeID := database.ThemeID(r.PathValue("themeID"))
	var theme *database.Theme
	pageTitle := "new theme"
	if themeID != "" {
		var err error
		theme, err = db.Theme.Get(r.Context(), themeID)
		if err != nil {
			hlog.FromRequest(r).Err(err).Msg("Failed to get theme")
			sendErrorResponse(w, r, mautrix.MUnknown.WithMessage("Failed to get theme %q", themeID))
			return
		} else if theme == nil {
			sendErrorResponse(w, r, mautrix.MNotFound.WithMessage("Theme %q not found", themeID))
			return
		} else if !slices.Contains(theme.Admins, userID) {
			sendErrorResponse(w, r, mautrix.MForbidden.WithMessage("You're not an admin of %q", themeID))
			return
		}
		pageTitle = "edit " + theme.Name
	}
	sendResponse(w, r, pageTitle, "theme-edit", &ThemePageData{Theme: theme})
}
