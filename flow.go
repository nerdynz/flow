package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	dat "gopkg.in/mgutz/dat.v1"

	"strings"

	"github.com/go-zoo/bone"
	"github.com/jaybeecave/base/flash"
	"github.com/nerdynz/datastore"
	"github.com/nerdynz/view"
	"github.com/unrolled/render"
)

type Context struct {
	W        http.ResponseWriter
	Req      *http.Request
	Store    *datastore.Datastore
	Renderer *render.Render
}

func New(w http.ResponseWriter, req *http.Request, store *datastore.Datastore) *Context {
	return &Context{
		W:     w,
		Req:   req,
		Store: store,
	}
}

func NewWithRenderer(w http.ResponseWriter, req *http.Request, store *datastore.Datastore, renderer *render.Render) *Context {
	return &Context{
		W:        w,
		Req:      req,
		Store:    store,
		Renderer: renderer,
	}
}

func (ctx *Context) AddRenderer(renderer *render.Render) {
	ctx.Renderer = renderer
}

func (ctx *Context) URLParam(key string) string {
	value := bone.GetValue(ctx.Req, key)
	if value != "" {
		newValue, err := url.QueryUnescape(value)
		if err == nil {
			value = strings.Replace(newValue, "%20", " ", -1)
		}
	}
	return value
}

func (ctx *Context) URLIntParam(key string) (int, error) {
	return strconv.Atoi(ctx.URLParam(key))
}

func (ctx *Context) URLIntParamWithDefault(key string, deefault int) int {
	val := ctx.URLParam(key)
	if val == "" {
		return deefault // default
	}
	c, err := strconv.Atoi(ctx.URLParam(key))
	if err != nil {
		return deefault // default
	}
	return c
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
				<link rel="apple-touch-icon" sizes="180x180" href="/icons/apple-touch-icon.png">
				<link rel="icon" type="image/png" sizes="32x32" href="/icons/favicon-32x32.png">
				<link rel="icon" type="image/png" sizes="16x16" href="/icons/favicon-16x16.png">
				<link rel="manifest" href="/icons/manifest.json">
				<link rel="mask-icon" href="/icons/safari-pinned-tab.svg" color="#5bbad5">
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
			<link href="/css/main.css" rel="stylesheet" />
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

type ViewBucket struct {
	renderer *render.Render
	store    *datastore.Datastore
	w        http.ResponseWriter
	req      *http.Request
	Data     map[string]interface{}
}

func NewBucket(ctx *Context) *ViewBucket {
	viewBag := ViewBucket{}
	viewBag.w = ctx.W
	viewBag.req = ctx.Req
	viewBag.store = ctx.Store
	viewBag.renderer = ctx.Renderer
	viewBag.Data = make(map[string]interface{})
	viewBag.Add("Now", time.Now())
	viewBag.Add("Year", time.Now().Year())

	return &viewBag
}

func (viewBag *ViewBucket) Add(key string, value interface{}) {
	viewBag.Data[key] = value
	// spew.Dump(viewBag.data)
}

func (viewBag *ViewBucket) LoadNavItems() {
	var navItems []*NavItem
	err := viewBag.store.DB.
		Select("title", "slug").
		From("pages").
		QueryStructs(&navItems)
	if err != nil {
		panic(err)
	}
	viewBag.Add("NavItems", navItems)
}

func (viewBag *ViewBucket) HTML(status int, templateName string) {

	// automatically show the flash message if it exists
	msg, _ := flash.GetFlash(viewBag.w, viewBag.req, "InfoMessage")
	viewBag.Add("InfoMessage", msg) // if its blank it can be blank but atleast it will exist

	viewBag.renderer.HTML(viewBag.w, status, templateName, viewBag.Data)
}

func (viewBag *ViewBucket) Text(status int, text string) {
	viewBag.renderer.Text(viewBag.w, status, text)
}

var TemplateFunctions = template.FuncMap{
	"javascript": javascriptTag,
	"stylesheet": stylesheetTag,
	"image":      imageTag,
	"imagepath":  imagePath,
	"content":    content,
	"htmlblock":  htmlblock,
	"navigation": navigation,
	"link":       link,
	"title":      title,

	"isBlank":    isBlank,
	"isNotBlank": isNotBlank,
	"formatDate": formatDate,
	"htmlsafe":   htmlSafe,
	"gt":         greaterThan,
}

func greaterThan(num int, amt int) bool {
	return num > amt
}

func content(contents ...string) template.HTML {
	var str string
	for _, content := range contents {
		str += "<div class='standard'>" + content + "</standard>"
	}
	return template.HTML(str)
}

func javascriptTag(names ...string) template.HTML {
	var str string
	for _, name := range names {
		str += "<script src='/js/" + name + ".js' type='text/javascript'></script>"
	}
	return template.HTML(str)
}

func stylesheetTag(names ...string) template.HTML {
	var str string
	for _, name := range names {
		str += "<link rel='stylesheet' href='/css/" + name + ".css' type='text/css' media='screen'  />\n"
	}
	return template.HTML(str)
}

func imagePath(name string) string {
	return "/images/" + name
}

func imageTag(name string, class string) template.HTML {
	return template.HTML("<image src='" + imagePath(name) + "' class='" + class + "' />")
}

func htmlSafe(str string) template.HTML {
	return template.HTML(str)
}

func htmlblock(page *Page, code string) template.HTML {
	html := "<div class='textblock editable' "
	html += " data-textblock='page-" + strconv.FormatInt(page.PageID, 10) + "-" + code + "'"
	html += " data-placeholder='#{placeholder}'> "
	html += getHTMLFromTextblock(page, code)
	html += "</div>"
	return template.HTML(html)
}

func link(text string, link string, viewBag *ViewBucket) template.HTML {
	class := "link link-" + strings.ToLower(text)
	if strings.ToLower(link) == viewBag.req.URL.Path {
		class += " active"
	}
	return template.HTML(fmt.Sprintf(`<a class="%v" href="%v">%v</a>`, class, link, text))
}

func title(text string) string {
	return strings.Title(text)
}

func navigation(viewBag *ViewBucket) template.HTML {
	html := ""
	if viewBag.Data["NavItems"] != nil {
		navItems := viewBag.Data["NavItems"].([]*NavItem)
		html = "<nav class='main-nav closed'>"
		for _, navItem := range navItems {
			html += "<a href='/" + navItem.Slug + "'>" + navItem.Title + "</a>"
		}
		html += "</nav>"
	}
	return template.HTML(html)
}

func isBlank(str string) bool {
	return str == ""
}

func isNotBlank(str string) bool {
	return !isBlank(str)
}

type Page struct {
	PageID     int64        `db:"page_id"`
	Title      string       `db:"title"`
	Body       string       `db:"body"`
	Slug       string       `db:"slug"`
	Template   string       `db:"template"`
	CreatedAt  dat.NullTime `db:"created_at"`
	UpdatedAt  dat.NullTime `db:"updated_at"`
	Textblocks []*Textblock
}

type NavItem struct {
	Title string `db:"title"`
	Slug  string `db:"slug"`
}

func (navItem *NavItem) getURL() string {
	return ""
}

type Textblock struct {
	TextblockID int64        `db:"textblock_id"`
	Code        string       `db:"code"`
	Body        string       `db:"body"`
	CreatedAt   dat.NullTime `db:"created_at"`
	UpdatedAt   dat.NullTime `db:"updated_at"`
	PageID      int64        `db:"page_id"`
}

func getHTMLFromTextblock(page *Page, code string) string {
	var body string
	for _, tb := range page.Textblocks {
		if tb.Code == code {
			body = tb.Body
		}
	}
	return body
}

func formatDate(time time.Time, layout string) string {
	return time.Format(layout)
}
