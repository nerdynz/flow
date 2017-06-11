package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"strings"

	"github.com/go-zoo/bone"
	"github.com/nerdynz/datastore"
	"github.com/nerdynz/view"
)

type Context struct {
	W     http.ResponseWriter
	Req   *http.Request
	Store *datastore.Datastore
}

func New(w http.ResponseWriter, req *http.Request, store *datastore.Datastore) *Context {
	return &Context{
		W:     w,
		Req:   req,
		Store: store,
	}
}

func (ctx *Context) URLParam(key string) string {
	return bone.GetValue(ctx.Req, key)
}

func (ctx *Context) URLIntParam(key string) (int, error) {
	return strconv.Atoi(ctx.URLParam(key))
}

func (ctx *Context) Publish(cat string, data interface{}) error {
	if ctx.Store.EventBus == nil {
		return errors.New("Long Polling Event Manager not initialised")
	}
	return ctx.Store.EventBus.Publish(cat, data)
}

func (ctx *Context) JSON(status int, data interface{}) {
	view.JSON(ctx.W, status, data)
}
func (ctx *Context) SPA(status int, data *PageInfo) {
	// logrus.Info(strings.ToLower(ctx.Req.Header.Get("Accept")))
	if strings.Contains(strings.ToLower(ctx.Req.Header.Get("Accept")), "application/json") {
		ctx.JSON(status, data)
	} else {
		path := ctx.Req.URL.Path
		buf := bytes.NewBufferString(`<!DOCTYPE html>
		<html>
			<title>` + data.Title + " - " + data.SiteInfo.Tagline + " - " + data.SiteInfo.SiteName + `</title>
			<link href="/css/main.css" rel="stylesheet" />
			<script data-main="js/app" src="/js/require.js"></script>
			<script>var cache = cache || {}; cache.root = '` + path + `'; cache.data = `) // continues after render
		json.NewEncoder(buf).Encode(data)
		buf.Write([]byte("console.log('data', cache.data)"))
		buf.Write([]byte("</script>"))
		buf.Write([]byte("</html>"))
		ctx.W.Header().Set("Content-Type", "text/html")
		ctx.W.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
		ctx.W.Write(buf.Bytes())
	}
}

func (ctx *Context) Render(status int, buffer *bytes.Buffer) {
	ctx.W.WriteHeader(status)
	ctx.W.Write(buffer.Bytes())
}

func (ctx *Context) RenderPDF(bytes []byte) {
	ctx.W.Header().Set("Content-Type", "application/PDF")
	ctx.W.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	ctx.W.Write(bytes)
}

type SiteInfo struct {
	Tagline  string
	SiteName string
}

type PageInfo struct {
	Title    string
	SiteInfo *SiteInfo
}
