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
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"go.mau.fi/util/exerrors"
	"go.mau.fi/util/exhttp"
	"go.mau.fi/util/exzerolog"
	"go.mau.fi/util/ptr"
	"go.mau.fi/util/requestlog"
	"go.mau.fi/zeroconfig"

	"css.gomuks.app/database"
)

func init() {
	if _, hasPort := os.LookupEnv("PORT"); !hasPort {
		exerrors.PanicIfNotNil(os.Setenv("PORT", "8080"))
	}
}

var defLog *zerolog.Logger
var db *database.Database

func main() {
	defLog = exerrors.Must((&zeroconfig.Config{
		Writers: []zeroconfig.WriterConfig{{
			Type:     zeroconfig.WriterTypeStdout,
			Format:   zeroconfig.LogFormatPrettyColored,
			MinLevel: ptr.Ptr(zerolog.InfoLevel),
		}, {
			Type:   zeroconfig.WriterTypeFile,
			Format: zeroconfig.LogFormatJSON,
			FileConfig: zeroconfig.FileConfig{
				Filename:   "/var/log/gomuks-css.log",
				MaxSize:    100 * 1024,
				MaxAge:     7,
				MaxBackups: 10,
			},
		}},
		MinLevel: ptr.Ptr(zerolog.TraceLevel),
	}).Compile())
	exzerolog.SetupDefaults(defLog)
	db = exerrors.Must(database.New(os.Getenv("DATABASE_URL"), defLog.With().Str("component", "database").Logger()))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", getIndexPage)
	mux.HandleFunc("GET /user/{userID}", getUserPage)
	mux.HandleFunc("GET /theme/{themeID}", getThemePage)
	mux.HandleFunc("GET /theme/{themeID}/commit/{version}", getThemePage)
	mux.HandleFunc("GET /theme/{themeID}/commits", getThemeHistoryPage)
	mux.HandleFunc("GET /theme/{themeID}/edit", getThemeEditPage)
	mux.HandleFunc("GET /theme/new", getThemeEditPage)
	mux.HandleFunc("POST /theme/commit", postThemeEditPage)
	mux.HandleFunc("GET /image/{imageID}", getImage)
	mux.HandleFunc("GET /login", handleRemoteLogin)
	mux.Handle("/static/", http.FileServer(http.FS(StaticFS)))

	server := http.Server{
		Addr: fmt.Sprintf("%s:%s", os.Getenv("HOST"), os.Getenv("PORT")),
		Handler: exhttp.ApplyMiddleware(
			mux,
			hlog.NewHandler(*defLog),
			requestlog.AccessLogger(true),
		),
	}

	ctx := defLog.WithContext(context.Background())
	exerrors.PanicIfNotNil(db.Upgrade(ctx))

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		exerrors.PanicIfNotNil(server.Shutdown(ctx))
		cancel()
	}()
	defLog.Info().Str("listen_address", server.Addr).Msg("Starting server")
	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func getImage(w http.ResponseWriter, r *http.Request) {
	imageID, err := uuid.Parse(r.PathValue("imageID"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	image, err := db.PreviewImage.Get(r.Context(), imageID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if image == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", image.MimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(image.Content)))
	w.Header().Set("Cache-Control", "max-age=2592000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image.Content)
}
