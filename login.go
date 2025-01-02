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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/hlog"
	"maunium.net/go/mautrix/federation"
	"maunium.net/go/mautrix/id"
)

var fc = federation.NewClient("", nil)
var tokenSecret = os.Getenv("TOKEN_SECRET")

const CookieLifetime = 24 * time.Hour

func init() {
	if tokenSecret == "" {
		panic("TOKEN_SECRET env var is required")
	}
}

func makeToken(userID id.UserID, expiry time.Time) string {
	expiryTS := expiry.Unix()
	tokenData := make([]byte, 8+len(userID), 8+len(userID)+32)
	binary.BigEndian.PutUint64(tokenData, uint64(expiryTS))
	copy(tokenData[8:], userID)
	hasher := hmac.New(sha256.New, []byte(tokenSecret))
	hasher.Write(tokenData)
	return base64.RawURLEncoding.EncodeToString(hasher.Sum(tokenData))
}

func verifyToken(token string) id.UserID {
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(data) <= 40 {
		return ""
	}
	hasher := hmac.New(sha256.New, []byte(tokenSecret))
	hasher.Write(data[:len(data)-32])
	if !hmac.Equal(data[len(data)-32:], hasher.Sum(nil)) {
		return ""
	}
	expiryTS := time.Unix(int64(binary.BigEndian.Uint64(data)), 0)
	if time.Now().After(expiryTS) {
		return ""
	}
	return id.UserID(data[8 : len(data)-32])
}

func verifyCookie(r *http.Request) id.UserID {
	cookie, err := r.Cookie(cookieName)
	if cookie == nil || err != nil {
		return ""
	}
	return verifyToken(cookie.Value)
}

const cookieName = "gomuks-css-auth"

func handleRemoteLogin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	log := hlog.FromRequest(r)
	resp, err := fc.GetOpenIDUserInfo(r.Context(), q.Get("server_name"), q.Get("token"))
	if err != nil {
		log.Err(err).Msg("Failed to get OpenID user info")
		w.WriteHeader(http.StatusUnauthorized)
		// TODO write body
		return
	}
	cookieExpiry := time.Now().Add(CookieLifetime)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    makeToken(resp.Sub, cookieExpiry),
		Expires:  cookieExpiry,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusSeeOther)
}
