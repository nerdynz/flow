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
	data.DocumentTitle = data.Title + " - " + data.SiteInfo.Tagline + " - " + data.SiteInfo.Sitename
	// logrus.Info(strings.ToLower(ctx.Req.Header.Get("Accept")))
	if strings.Contains(strings.ToLower(ctx.Req.Header.Get("Accept")), "application/json") {
		ctx.JSON(status, data)
	} else {
		url := ctx.Req.URL
		buf := bytes.NewBufferString(`<!DOCTYPE html>
		<html>
			<head>
				<title>` + data.DocumentTitle + `</title>
				<link rel="apple-touch-icon" sizes="57x57" href="/icons/apple-icon-57x57.png">
				<link rel="apple-touch-icon" sizes="60x60" href="/icons/apple-icon-60x60.png">
				<link rel="apple-touch-icon" sizes="72x72" href="/icons/apple-icon-72x72.png">
				<link rel="apple-touch-icon" sizes="76x76" href="/icons/apple-icon-76x76.png">
				<link rel="apple-touch-icon" sizes="114x114" href="/icons/apple-icon-114x114.png">
				<link rel="apple-touch-icon" sizes="120x120" href="/icons/apple-icon-120x120.png">
				<link rel="apple-touch-icon" sizes="144x144" href="/icons/apple-icon-144x144.png">
				<link rel="apple-touch-icon" sizes="152x152" href="/icons/apple-icon-152x152.png">
				<link rel="apple-touch-icon" sizes="180x180" href="/icons/apple-icon-180x180.png">
				<link rel="icon" type="image/png" sizes="192x192"  href="/icons/android-icon-192x192.png">
				<link rel="icon" type="image/png" sizes="32x32" href="/icons/favicon-32x32.png">
				<link rel="icon" type="image/png" sizes="96x96" href="/icons/favicon-96x96.png">
				<link rel="icon" type="image/png" sizes="16x16" href="/icons/favicon-16x16.png">
				<link rel="manifest" href="/icons/manifest.json">
				<meta name="msapplication-TileColor" content="#ffffff">
				<meta name="msapplication-TileImage" content="/icons/ms-icon-144x144.png">
				<meta name="theme-color" content="#ffffff">
				
				<meta name="description" content="` + data.Description + `">
				<meta name="robots" content="index, follow">
				<meta property="og:title" content="` + data.Title + `">

				<meta property="og:type" content="website">
				<meta property="og:description" content="` + data.Description + `">
				<meta property="og:image" content="` + data.Image + `">
				<meta property="og:url" content="` + url.RequestURI() + `">

				<meta name="twitter:title" content="` + data.Title + `">
				<meta name="twitter:card" content="summary">
				<meta name="twitter:url" content="` + url.RequestURI() + `">
				<meta name="twitter:image" content="` + data.Image + `">
				<meta name="twitter:description" content="` + data.Description + `">
			</head>
			<body>
			<div id="application"></div>
			<script>var cache = cache || {}; cache.root = '` + url.Path + `'; cache.data = `) // continues after render

		json.NewEncoder(buf).Encode(data)
		buf.Write([]byte(`</script>
			<link href="/css/main.css" rel="stylesheet" media="none" onload="if(media!='all')media='all'" />
			<script data-main="js/app" src="/js/require.js"></script>
			</body>
		`))
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
	Sitename string
}

type PageInfo struct {
	Title         string
	Description   string
	URL           string
	Image         string
	DocumentTitle string
	SiteInfo      *SiteInfo
}
