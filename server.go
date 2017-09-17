package main

import (
	"bytes"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo"
)

type Template struct{ templates *template.Template }

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func main() {
	e := echo.New()
	db, err := sql.Open("sqlite3", "./database.sqlite")
	if err != nil {
		e.Logger.Error(err)
	}
	defer db.Close()

	e.Renderer = &Template{templates: template.Must(template.ParseGlob("assets/templates/*.html"))}

	e.File("/favicon.ico", "assets/public/favicon.ico")
	e.File("/robots.txt", "assets/public/robots.txt")
	e.File("/style.css", "assets/public/style.css")
	e.File("/index.html", "assets/public/index.html")
	e.File("/new", "assets/public/new.html")
	e.File("/", "assets/public/index.html")

	e.GET("/TOS.md", func(c echo.Context) error {
		n, code := md2html(c, "TOS")
		return c.Render(code, "Page", n)
	})
	e.GET("/:id", func(c echo.Context) error {
		n, code := load(c, db)
		return c.Render(code, "Note", n)
	})
	e.GET("/:id/export", func(c echo.Context) error {
		n, code := load(c, db)
		return c.String(code, n.Text)
	})
	e.GET("/:id/stats", func(c echo.Context) error {
		n, code := load(c, db)
		buf := bytes.NewBuffer([]byte{})
		e.Renderer.Render(buf, "Stats", n, c)
		n.Content = template.HTML(buf.String())
		return c.Render(code, "Note", n)
	})

	e.POST("/note", func(c echo.Context) error {
		vals, err := c.FormParams()
		if err != nil {
			return err
		}
		if get(vals, "tos") != "on" {
			code := http.StatusPreconditionFailed
			return c.Render(code, "Note", errPage(code))
		}
		text := get(vals, "text")
		if 10 > len(text) || len(text) > 50000 {
			code := http.StatusBadRequest
			return c.Render(code, "Note",
				errPage(code, "note length not accepted"))
		}
		n := Note{
			Text:     text,
			Password: get(vals, "password"),
		}
		id, err := save(c, db, &n)
		if err != nil {
			c.Logger().Error(err)
			code := http.StatusServiceUnavailable
			return c.Render(code, "Note", errPage(code))
		}
		c.Logger().Infof("new note %q created", n.ID)
		return c.Redirect(http.StatusMovedPermanently, "/"+id)
	})

	e.Logger.Fatal(e.Start(":3000"))
}

func get(vals url.Values, key string) string {
	if list, found := vals[key]; found {
		if len(list) == 1 {
			return list[0]
		}
	}
	return ""
}

func md2html(c echo.Context, name string) (Note, int) {
	path := "assets/markdown/" + name + ".md"
	mdContent, err := ioutil.ReadFile(path)
	if err != nil {
		c.Logger().Errorf("couldn't open markdown page %q: %v", path, err)
		code := http.StatusServiceUnavailable
		return errPage(code), code
	}
	return Note{Title: name, Content: mdTmplHTML(mdContent)}, http.StatusOK
}