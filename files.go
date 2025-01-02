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
	"embed"
	"html/template"
	"io/fs"

	"go.mau.fi/util/exerrors"
	"maunium.net/go/mautrix/id"
)

//go:embed templates
var TemplateFS embed.FS

//go:embed static
var StaticFS embed.FS

var templateFuncs = map[string]any{
	"add": func(a, b int) int { return a + b },
}

var Templates = exerrors.Must(template.New("templates").
	Funcs(templateFuncs).
	ParseFS(exerrors.Must(fs.Sub(TemplateFS, "templates")), "*.gohtml"))

type ContainerData struct {
	PageTitle string
	User      id.UserID
	Page      string
	Data      any
}
