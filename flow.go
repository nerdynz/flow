package flow

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"time"

	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/go-zoo/bone"
	"github.com/nerdynz/datastore"
	"github.com/nerdynz/security"
	"github.com/nerdynz/view"
	"github.com/unrolled/render"
)

type Context struct {
	W            http.ResponseWriter
	Req          *http.Request
	Renderer     *render.Render
	Padlock      *security.Padlock
	Store        *datastore.Datastore
	Settings     datastore.Settings
	Protocol     string
	Bucket       map[string]interface{}
	errLog       string
	hasPopulated bool
}

const NO_MASTER = "NO_MASTER"

// New manages every new request, set shortcuts here, be careful your within a "context" here
func New(w http.ResponseWriter, req *http.Request, renderer *render.Render, store *datastore.Datastore, key security.Key) *Context {
	c := &Context{}
	c.W = w
	c.Req = req
	c.Renderer = renderer
	c.Store = store
	c.Settings = store.Settings
	c.Padlock = security.New(req, store.Settings, key)
	c.hasPopulated = false

	proto := "http://"
	if store.Settings.IsProduction() {
		// should be secure
		proto = "https://"
	}
	if c.Req.Header.Get("X-Forwarded-Proto") == "https" {
		proto = "https://"
	}
	c.Protocol = proto

	// if store.Settings.LoggingEnabled {
	// 	c.Logger = datastore.NewLogger()
	// }
	return c
}

func (c *Context) WebsiteBaseURL() string {
	return c.Store.Settings.Get("WEBSITE_BASE_URL")
}

func (c *Context) SiteID() int {
	return c.Padlock.SiteID() // short hand... will panic if used improperly
}

// func (c *Context) GetCacheValue(key string) (string, error) {
// 	return c.Store.GetCacheValue(key)
// }

// func (c *Context) SetCacheValue(key string, value interface{}, duration time.Duration) (string, error) {
// 	return c.Store.SetCacheValue(key, value, duration)
// }

func (c *Context) Write(b []byte) (int, error) {
	c.errLog += string(b)
	return len(b), nil
}

func (c *Context) populateCommonVars() {
	c.hasPopulated = true
	c.Bucket = make(Bucket)
	proto := "http://"
	if c.Settings.IsDevelopment() {
		c.Add("IsDev", true)
		proto = "http://"
	} else {
		proto = "https://"
	}
	if c.Settings.GetBool("IS_HTTPS") {
		proto = "https://"
	}
	loggedInUser, _, _ := c.Padlock.LoggedInUser()
	c.Add("IsLoggedIn", loggedInUser != nil)
	if loggedInUser != nil {
		c.Add("LoggedInUser", loggedInUser)
	}
	c.Add("websiteBaseURL", proto+c.Req.Host+"/")
	c.Add("currentURL", c.Req.URL.Path)
	c.Add("currentFullURL", proto+c.Req.Host+c.Req.URL.Path)
	c.Add("Now", time.Now())
	c.Add("Year", time.Now().Year())
}

type Bucket map[string]interface{}

func (c *Context) Add(key string, value interface{}) {
	if !c.hasPopulated {
		c.populateCommonVars()
	}
	c.Bucket[key] = value
}

func (ctx *Context) AddRenderer(renderer *render.Render) {
	ctx.Renderer = renderer
}

func (ctx *Context) URLValues(key string) ([]string, error) {
	err := ctx.Req.ParseForm()
	if err != nil {
		return nil, err
	}
	return ctx.Req.Form[key], nil
}

func (ctx *Context) URLIntValues(key string) ([]int, error) {
	ints := make([]int, 0)
	vals, err := ctx.URLValues(key)
	if err != nil {
		return ints, err
	}
	for _, val := range vals {
		iVal, err := strconv.Atoi(val)
		if err != nil {
			return ints, err
		}
		ints = append(ints, iVal)
	}
	return ints, nil
}

func (ctx *Context) URLParam(key string) string {
	// try route param
	value := bone.GetValue(ctx.Req, key)

	// try qs
	if value == "" {
		value = ctx.Req.URL.Query().Get(key)
	}

	// do we have a value
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

func (ctx *Context) URLDateParam(key string) (time.Time, error) {
	var dt = ctx.URLParam(key)
	return time.Parse(time.RFC3339Nano, dt)
}

func (ctx *Context) URLShortDateParam(key string) (time.Time, error) {
	var dt = ctx.URLParam(key)
	return time.Parse("20060102", dt)
}

func (ctx *Context) URLBoolParam(key string) bool {
	val := ctx.URLParam(key)
	if val == "true" {
		return true
	}
	if val == "yes" {
		return true
	}
	if val == "1" {
		return true
	}
	if val == "y" {
		return true
	}
	if val == "✓" {
		return true
	}
	return false
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

func (ctx *Context) URLUnique() string {
	val := ctx.URLParam("uniqueid")
	if val == "" {
		val = ctx.URLParam("ulid")
	}
	return strings.ToUpper(val)
}

func (ctx *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(ctx.W, cookie)
}

func (ctx *Context) Redirect(newUrl string, status int) {
	if status == 301 || status == 302 || status == 303 || status == 304 || status == 401 {
		http.Redirect(ctx.W, ctx.Req, newUrl, status)
		return
	}
	ctx.ErrorHTML(http.StatusInternalServerError, "Invalid Redirect", nil)
}

func (ctx *Context) File(bytes []byte, filename string, mime string) {
	ctx.W.Header().Set("Content-Type", mime)
	ctx.W.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	ctx.W.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	ctx.W.Write(bytes)
}

func (ctx *Context) InlineFile(bytes []byte, filename string, mime string) {
	ctx.W.Header().Set("Content-Type", mime)
	ctx.W.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	ctx.W.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	ctx.W.Write(bytes)
}

func (ctx *Context) PDF(bytes []byte) {
	ctx.W.Header().Set("Content-Type", "application/PDF")
	ctx.W.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	ctx.W.Write(bytes)
}

func (ctx *Context) Excel(bytes []byte, filename string) {
	ctx.W.Header().Set("Content-Type", "application/vnd.ms-excel")
	ctx.W.Header().Set("Content-Disposition", `filename="`+filename+`.xlsx"`)
	ctx.W.Write(bytes)
}

func (ctx *Context) Text(status int, str string) {
	ctx.Renderer.Text(ctx.W, status, str)
}

func (ctx *Context) HTMLalt(layout string, status int, master string) error {
	return ctx.htmlAlt(ctx.W, layout, status, master)
}
func (ctx *Context) htmlAlt(w io.Writer, layout string, status int, master string) error {
	if !ctx.hasPopulated {
		ctx.populateCommonVars()
	}
	if ctx.Req.URL.Query().Get("dump") == "1" {
		return ctx.Renderer.HTML(w, status, "error", ctx.Bucket)
	}
	if ctx.Req.Header.Get("X-PJAX") == "true" {
		return ctx.Renderer.HTML(w, status, layout, ctx.Bucket, render.HTMLOptions{
			Layout: "pjax",
		})
	}
	if master == NO_MASTER {
		return ctx.Renderer.HTML(w, status, layout, ctx.Bucket, render.HTMLOptions{})
	}
	if master != "" {
		return ctx.Renderer.HTML(w, status, layout, ctx.Bucket, render.HTMLOptions{
			Layout: master,
		})
	}
	return ctx.Renderer.HTML(w, status, layout, ctx.Bucket)
}

func (ctx *Context) HTML(layout string, status int) {
	ctx.HTMLalt(layout, status, "")
}

func (ctx *Context) HTMLAsText(layout string, status int) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	err := ctx.Renderer.HTML(buf, status, layout, ctx.Bucket)
	return buf, err
}
func (ctx *Context) HTMLAsTextAlt(layout string, status int, master string) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	err := ctx.htmlAlt(buf, layout, status, master)
	return buf, err
}

func (ctx *Context) JSON(status int, data interface{}) {
	// render.JSON(ctx.W, status, data)
	ctx.Renderer.JSON(ctx.W, status, data)
}

func (ctx *Context) ErrorText(status int, friendly string, errs ...error) {
	if errs != nil && len(errs) > 0 {
		ctx.errorOut(true, status, friendly, errs...)
	} else {
		ctx.errorOut(true, status, friendly, nil)
	}
}

func (ctx *Context) ErrorJSON(status int, friendly string, errs ...error) {
	if errs != nil && len(errs) > 0 {
		ctx.errorOut(false, status, friendly, errs...)
	} else {
		ctx.errorOut(false, status, friendly, nil)
	}
}

func (ctx *Context) errorOut(isText bool, status int, friendly string, errs ...error) {
	//https: //stackoverflow.com/questions/24809287/how-do-you-get-a-golang-program-to-print-the-line-number-of-the-error-it-just-ca
	errStr := ""
	lineNumber := -1
	funcName := "Not Specified"
	fileName := "Not Specified"

	if errs != nil && len(errs) > 0 {
		for _, err := range errs {
			if err != nil {
				errStr += err.Error() + "\n"
			} else {
				errStr += "No Error Specified \n"
			}
		}
		// notice that we're using 1, so it will actually log the where
		// the error happened, 0 = this function, we don't want that.
		pc, file, line, _ := runtime.Caller(1)
		lineNumber = line
		funcName = runtime.FuncForPC(pc).Name()
		fileName = file
	}

	data := &errorData{
		friendly,
		errStr,
		lineNumber,
		funcName,
		fileName,
	}

	if status != 400 {
		log.Error(data.nicelyFormatted())
	}
	if isText {
		ctx.Renderer.Text(ctx.W, status, data.nicelyFormatted())
		return
	}
	view.JSON(ctx.W, status, data)
}

type errorData struct {
	Friendly     string
	Error        string
	LineNumber   int
	FunctionName string
	FileName     string
}

func (e *errorData) nicelyFormatted() string {
	str := ""
	str += "Friendly Message: \n\t" + e.Friendly + "\n"
	str += "Error: \n\t" + e.Error + "\n"
	str += "FileName: \n\t" + e.FileName + "\n"
	str += "LineNumber: \n\t" + strconv.Itoa(e.LineNumber) + "\n"
	str += "FunctionName: \n\t" + e.FunctionName + "\n"
	return str
}

func (ctx *Context) ErrorHTML(status int, friendly string, errs ...error) {
	errStr := ""
	lineNumber := -1
	funcName := "Not Specified"
	fileName := "Not Specified"
	ctx.Add("FriendlyError", friendly)
	if errs != nil && len(errs) > 0 {
		for _, err := range errs {
			errStr += err.Error() + "\n"
		}
		ctx.Add("NastyError", errStr)

		// notice that we're using 1, so it will actually log the where
		// the error happened, 0 = this function, we don't want that.
		pc, file, line, _ := runtime.Caller(1)
		lineNumber = line
		funcName = runtime.FuncForPC(pc).Name()
		fileName = file

		ctx.Add("LineNumber", lineNumber)
		ctx.Add("FuncName", funcName)
		ctx.Add("FileName", fileName)

	}
	ctx.Add("ErrorCode", status)
	ctx.noTemplateHTML("error", status)
}

func (ctx *Context) noTemplateHTML(layout string, status int) {
	opt := render.HTMLOptions{
		Layout: "",
	}
	ctx.Renderer.HTML(ctx.W, status, layout, ctx.Bucket, opt)
}

// func (ctx *Context) BroadcastToCurrentSite(t string, data interface{}) error {
// 	s := strconv.Itoa(ctx.SiteID())
// 	return ctx.Broadcast("room-"+s, t, data)
// }

// func (ctx *Context) BroadcastToSite(siteID int, t string, data interface{}) error {
// 	s := strconv.Itoa(siteID)
// 	return ctx.Broadcast("room-"+s, t, data)
// }

// func (ctx *Context) Broadcast(room string, t string, data interface{}) error {
// 	bc := &broadcast{
// 		Type: strings.Title(t),
// 		Data: data,
// 	}
// 	b, err := json.Marshal(bc)
// 	if err != nil {
// 		return err
// 	}
// 	err = ctx.Store.Websocket.Broadcast(room, string(b))
// 	return err
// }

// type broadcast struct {
// 	Type string
// 	Data interface{}
// }

// func (ctx *Context) SPA(status int, pageInfo *PageInfo, data interface{}) {
// 	pageInfo.DocumentTitle = pageInfo.Title
// 	if pageInfo.SiteInfo != nil {
// 		pageInfo.DocumentTitle = pageInfo.Title + " - " + pageInfo.SiteInfo.Tagline + " - " + pageInfo.SiteInfo.Sitename
// 	}
// 	// logrus.Info(strings.ToLower(ctx.Req.Header.Get("Accept")))
// 	if strings.Contains(strings.ToLower(ctx.Req.Header.Get("Accept")), "application/json") {
// 		ctx.JSON(status, data)
// 	} else {
// 		url := ctx.Req.URL
// 		buf := bytes.NewBufferString(`<!DOCTYPE html>
// 		<html>
// 			<head>
// 				<title>` + pageInfo.DocumentTitle + `</title>
// 				<link rel="apple-touch-icon" sizes="180x180" href="/icons/apple-touch-icon.png">
// 				<link rel="icon" type="image/png" sizes="32x32" href="/icons/favicon-32x32.png">
// 				<link rel="icon" type="image/png" sizes="16x16" href="/icons/favicon-16x16.png">
// 				<link rel="manifest" href="/icons/manifest.json">
// 				<link rel="mask-icon" href="/icons/safari-pinned-tab.svg" color="#5bbad5">

// 				<meta charset="utf-8">
//     		<meta http-equiv="x-ua-compatible" content="ie=edge">
//     		<meta name="viewport" content="width=device-width, initial-scale=1">
// 				<meta name="theme-color" content="#ffffff">

// 				<meta name="description" content="` + pageInfo.Description + `">
// 				<meta name="robots" content="index, follow">
// 				<meta property="og:title" content="` + pageInfo.Title + `">

// 				<meta property="og:type" content="website">
// 				<meta property="og:description" content="` + pageInfo.Description + `">
// 				<meta property="og:image" content="` + pageInfo.Image + `">
// 				<meta property="og:url" content="` + url.RequestURI() + `">

// 				<meta name="twitter:title" content="` + pageInfo.Title + `">
// 				<meta name="twitter:card" content="summary">
// 				<meta name="twitter:url" content="` + url.RequestURI() + `">
// 				<meta name="twitter:image" content="` + pageInfo.Image + `">
// 				<meta name="twitter:description" content="` + pageInfo.Description + `">
// 			</head>
// 			<body>
// 			<script>var cache = cache || {}; cache.root = '` + url.Path + `'; cache.data = `) // continues after render
// 		json.NewEncoder(buf).Encode(data)
// 		buf.Write([]byte(`</script>
// 			<link href=/assets/css/app.e4957b7c640ca81c405e048b4179579f.css rel=stylesheet><div id=app></div><script type=text/javascript src=/assets/js/manifest.0dfb595b05cd8e35b550.js></script><script type=text/javascript src=/assets/js/vendor.52b3c6544255470e9492.js></script><script type=text/javascript src=/assets/js/app.0838ee22ac8e436d61f3.js></script>
// 			</body>
// 		`))
// 		buf.Write([]byte("</html>"))
// 		ctx.W.Header().Set("Content-Type", "text/html")
// 		ctx.W.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
// 		ctx.W.Write(buf.Bytes())
// 	}
// }

// func (ctx *Context) Render(status int, buffer *bytes.Buffer) {
// 	ctx.W.WriteHeader(status)
// 	ctx.W.Write(buffer.Bytes())
// }

// func (ctx *Context) RenderPDF(bytes []byte) {
// 	ctx.W.Header().Set("Content-Type", "application/PDF")
// 	ctx.W.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
// 	ctx.W.Write(bytes)
// }

// type SiteInfo struct {
// 	Tagline  string
// 	Sitename string
// }

// type PageInfo struct {
// 	Title         string
// 	Description   string
// 	URL           string
// 	Image         string
// 	DocumentTitle string
// 	SiteInfo      *SiteInfo
// }

// type ViewBucket struct {
// 	renderer *render.Render
// 	store    *datastore.Datastore
// 	w        http.ResponseWriter
// 	req      *http.Request
// 	Data     map[string]interface{}
// }

// func NewBucket(ctx *Context) *ViewBucket {
// 	viewBag := ViewBucket{}
// 	viewBag.w = ctx.W
// 	viewBag.req = ctx.Req
// 	viewBag.store = ctx.Store
// 	viewBag.renderer = ctx.Renderer
// 	viewBag.Data = make(map[string]interface{})
// 	viewBag.Add("Now", time.Now())
// 	viewBag.Add("Year", time.Now().Year())

// 	return &viewBag
// }

// func (viewBag *ViewBucket) Add(key string, value interface{}) {
// 	viewBag.Data[key] = value
// 	// spew.Dump(viewBag.data)
// }

// func (viewBag *ViewBucket) HTML(status int, templateName string) {

// 	// automatically show the flash message if it exists
// 	msg, _ := flash.GetFlash(viewBag.w, viewBag.req, "InfoMessage")
// 	viewBag.Add("InfoMessage", msg) // if its blank it can be blank but atleast it will exist

// 	viewBag.renderer.HTML(viewBag.w, status, templateName, viewBag.Data)
// }

// func (viewBag *ViewBucket) Text(status int, text string) {
// 	viewBag.renderer.Text(viewBag.w, status, text)
// }

// type Settings interface {
// 	Get(key string) string
// 	GetBool(key string) bool
// 	IsProduction() bool
// }
