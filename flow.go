package flow

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"time"

	"strings"

	"github.com/oklog/ulid"

	"github.com/go-zoo/bone"
	"github.com/nerdynz/datastore"
	"github.com/nerdynz/security"
	"github.com/unrolled/render"
)

type Flow struct {
	respWrt      http.ResponseWriter
	req          *http.Request
	Renderer     *render.Render
	Padlock      *security.Padlock
	store        *datastore.Datastore
	scheme       string
	bucket       *bucket
	errLog       string
	hasPopulated bool
}

const NO_MASTER = "NO_MASTER"

// New manages every new request, set shortcuts here, be careful your within a "flowCtx" here
func New(w http.ResponseWriter, req *http.Request, renderer *render.Render, store *datastore.Datastore, key security.Key) *Flow {
	flow := &Flow{}
	flow.respWrt = w
	flow.req = req
	flow.Renderer = renderer
	flow.store = store
	flow.Padlock = security.New(req, store.Settings, key)
	flow.hasPopulated = false

	proto := "http://"
	if store.Settings.IsProduction() {
		// should be secure
		proto = "https://"
	}
	if req.Header.Get("X-Forwarded-Proto") == "https" {
		proto = "https://"
	}
	flow.scheme = proto
	req.URL.Scheme = proto // is this a bad idea???

	// if store.Settings.LoggingEnabled {
	// 	flow.Logger = datastore.NewLogger()
	// }
	return flow
}

func (flow *Flow) WebsiteBaseURL() string {
	return flow.store.Settings.Get("WEBSITE_BASE_URL")
}

func (flow *Flow) SiteID() int {
	return flow.Padlock.SiteID()
}

func (flow *Flow) SiteULID() (string, error) {
	return flow.Padlock.SiteULID()
}

// func (flow *flowCtx) GetCacheValue(key string) (string, error) {
// 	return flow.Store.GetCacheValue(key)
// }

// func (flow *flowCtx) SetCacheValue(key string, value interface{}, duration time.Duration) (string, error) {
// 	return flow.Store.SetCacheValue(key, value, duration)
// }

func (flow *Flow) Write(b []byte) (int, error) {
	flow.errLog += string(b)
	return len(b), nil
}

func (flow *Flow) catchAfterErr(err error) {
	// currently a NO-OP
}

func (flow *Flow) populateCommonVars() {
	flow.bucket = &bucket{
		vars: make(map[string]interface{}),
	}
	flow.hasPopulated = true
	loggedInUser, _, _ := flow.Padlock.LoggedInUser()
	flow.Add("IsLoggedIn", loggedInUser != nil)
	if loggedInUser != nil {
		flow.Add("LoggedInUser", loggedInUser)
	}
	flow.Add("websiteBaseURL", flow.scheme+flow.req.Host+"/")
	flow.Add("WebsiteBaseURL", flow.scheme+flow.req.Host+"/")
	flow.Add("currentURL", flow.req.URL.Path)
	flow.Add("CurrentURL", flow.req.URL.Path)
	flow.Add("currentFullURL", flow.scheme+flow.req.Host+flow.req.URL.Path)
	flow.Add("CurrentFullURL", flow.scheme+flow.req.Host+flow.req.URL.Path)
	flow.Add("Now", time.Now())
	flow.Add("Year", time.Now().Year())
}

type bucket struct {
	vars map[string]interface{}
}

func (flow *Flow) Add(key string, value interface{}) {
	if !flow.hasPopulated {
		flow.populateCommonVars()
	}
	flow.bucket.vars[key] = value
}

func (flow *Flow) AddRenderer(renderer *render.Render) {
	flow.Renderer = renderer
}

func (flow *Flow) URLValues(key string) ([]string, error) {
	err := flow.req.ParseForm()
	if err != nil {
		return nil, err
	}
	return flow.req.Form[key], nil
}

func (flow *Flow) URLIntValues(key string) ([]int, error) {
	ints := make([]int, 0)
	vals, err := flow.URLValues(key)
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

func (flow *Flow) URLULIDParam(key string) (string, error) {
	ul := strings.ToUpper(flow.URLParam(key))
	id, err := ulid.Parse(ul)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func (flow *Flow) URLParam(key string) string {
	// try route param
	value := bone.GetValue(flow.req, key)

	// try qs
	if value == "" {
		value = flow.req.URL.Query().Get(key)
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

func (flow *Flow) URLIntParam(key string) (int, error) {
	return strconv.Atoi(flow.URLParam(key))
}

func (flow *Flow) URLDateParam(key string) (time.Time, error) {
	var dt = flow.URLParam(key)
	return time.Parse(time.RFC3339Nano, dt)
}

func (flow *Flow) URLShortDateParam(key string) (time.Time, error) {
	var dt = flow.URLParam(key)
	return time.Parse("20060102", dt)
}

func (flow *Flow) URLBoolParam(key string) bool {
	val := flow.URLParam(key)
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
	b, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return b
}

func (flow *Flow) URLIntParamWithDefault(key string, deefault int) int {
	val := flow.URLParam(key)
	if val == "" {
		return deefault // default
	}
	c, err := strconv.Atoi(flow.URLParam(key))
	if err != nil {
		return deefault // default
	}
	return c
}

func (flow *Flow) URLUnique() string {
	val := flow.URLParam("uniqueid")
	if val == "" {
		val = flow.URLParam("ulid")
	}
	return strings.ToUpper(val)
}

func (flow *Flow) SetCookie(cookie *http.Cookie) {
	http.SetCookie(flow.respWrt, cookie)
}

func (flow *Flow) Redirect(newUrl string, status int) {
	if status == 301 || status == 302 || status == 303 || status == 304 || status == 401 {
		http.Redirect(flow.respWrt, flow.req, newUrl, status)
		return
	}
	flow.ErrorText(http.StatusInternalServerError, "Invalid Redirect", nil)
}

func (flow *Flow) StaticFile(status int, filepath string, mime string) {
	file, err := ioutil.ReadFile(filepath)
	if err != nil {
		flow.ErrorText(500, "Failed to load %s", err)
		return
	}
	flow.respWrt.Header().Add("content-type", mime)
	err = flow.Renderer.Data(flow.respWrt, status, file)
	flow.catchAfterErr(err)
}

func (flow *Flow) Data(status int, bytes []byte, mime string) {
	flow.respWrt.Header().Add("content-type", mime)
	err := flow.Renderer.Data(flow.respWrt, status, bytes)
	flow.catchAfterErr(err)
}

func (flow *Flow) File(bytes []byte, filename string, mime string) {
	flow.respWrt.Header().Set("Content-Type", mime)
	flow.respWrt.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	flow.respWrt.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	_, err := flow.respWrt.Write(bytes)
	flow.catchAfterErr(err)
}

func (flow *Flow) InlineFile(bytes []byte, filename string, mime string) {
	flow.respWrt.Header().Set("Content-Type", mime)
	flow.respWrt.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	flow.respWrt.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	_, err := flow.respWrt.Write(bytes)
	flow.catchAfterErr(err)
}

func (flow *Flow) PDF(bytes []byte) {
	flow.respWrt.Header().Set("Content-Type", "application/PDF")
	flow.respWrt.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	_, err := flow.respWrt.Write(bytes)
	flow.catchAfterErr(err)
}

func (flow *Flow) Excel(bytes []byte, filename string) {
	flow.respWrt.Header().Set("Content-Type", "application/vnd.ms-excel")
	flow.respWrt.Header().Set("Content-Disposition", `filename="`+filename+`.xlsx"`)
	_, err := flow.respWrt.Write(bytes)
	flow.catchAfterErr(err)
}

func (flow *Flow) Text(status int, str string) {
	err := flow.Renderer.Text(flow.respWrt, status, str)
	flow.catchAfterErr(err)
}

func (flow *Flow) HTMLalt(layout string, status int, master string) error {
	return flow.htmlAlt(flow.respWrt, layout, status, master)
}

func (flow *Flow) htmlAlt(w io.Writer, layout string, status int, master string) error {
	if !flow.hasPopulated {
		flow.populateCommonVars()
	}
	if flow.req.URL.Query().Get("dump") == "1" {
		return flow.Renderer.HTML(w, status, "error", flow.bucket.vars)
	}
	if flow.req.Header.Get("X-PJAX") == "true" {
		return flow.Renderer.HTML(w, status, layout, flow.bucket.vars, render.HTMLOptions{
			Layout: "pjax",
		})
	}
	if master == NO_MASTER {
		return flow.Renderer.HTML(w, status, layout, flow.bucket.vars, render.HTMLOptions{})
	}
	if master != "" {
		return flow.Renderer.HTML(w, status, layout, flow.bucket.vars, render.HTMLOptions{
			Layout: master,
		})
	}
	return flow.Renderer.HTML(w, status, layout, flow.bucket.vars)
}

func (flow *Flow) HTML(bucket bucket, layout string, status int) {
	err := flow.HTMLalt(layout, status, "")
	flow.catchAfterErr(err)
}
func (flow *Flow) HTMLAsText(layout string, status int) (*bytes.Buffer, error) {
	return flow.HTMLAsTextAlt(layout, status, "")
}
func (flow *Flow) HTMLAsTextAlt(layout string, status int, master string) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	err := flow.htmlAlt(buf, layout, status, master)
	return buf, err
}

func (flow *Flow) JSON(status int, data interface{}) {
	// render.JSON(flow.respWrt, status, data)
	err := flow.Renderer.JSON(flow.respWrt, status, data)
	flow.catchAfterErr(err)
}

func (flow *Flow) ErrorText(status int, friendly string, errs ...error) {
	if len(errs) > 0 {
		flow.errorOut(true, status, friendly, errs...)
	} else {
		flow.errorOut(true, status, friendly, nil)
	}
}

func (flow *Flow) ErrorJSON(status int, friendly string, errs ...error) {
	if len(errs) > 0 {
		flow.errorOut(false, status, friendly, errs...)
	} else {
		flow.errorOut(false, status, friendly, nil)
	}
}

func (flow *Flow) errorOut(isText bool, status int, friendly string, errs ...error) {
	//https: //stackoverflow.com/questions/24809287/how-do-you-get-a-golang-program-to-print-the-line-number-of-the-error-it-just-ca
	errStr := ""
	lineNumber := -1
	funcName := "Not Specified"
	fileName := "Not Specified"

	if len(errs) > 0 {
		for _, err := range errs {
			if err != nil {
				errStr += err.Error() + "\n"
			} else {
				errStr += "No Error Specified \n"
			}
		}
		// notice that we're using 1, so it will actually log the where // actually one is errorOut
		// the error happened, 0 = this function, we don't want that.
		pc, file, line, _ := runtime.Caller(2)
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

	flow.store.Logger.Error(data.nicelyFormatted())

	if isText {
		err := flow.Renderer.Text(flow.respWrt, status, data.nicelyFormatted())
		flow.catchAfterErr(err)
		return
	}
	w := flow.respWrt
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(data)
	flow.catchAfterErr(err)
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

func (flow *Flow) ErrorHTML(status int, friendly string, errs ...error) {
	errStr := ""
	flow.Add("FriendlyError", friendly)
	if len(errs) > 0 {
		for _, err := range errs {
			errStr += err.Error() + "\n"
		}
		flow.Add("NastyError", errStr)

		// notice that we're using 1, so it will actually log the where
		// the error happened, 0 = this function, we don't want that.
		pc, file, line, _ := runtime.Caller(1)
		lineNumber := line
		funcName := runtime.FuncForPC(pc).Name()
		fileName := file

		flow.Add("LineNumber", lineNumber)
		flow.Add("FuncName", funcName)
		flow.Add("FileName", fileName)

	}
	flow.Add("ErrorCode", status)
	flow.noTemplateHTML("error", status)
}

func (flow *Flow) noTemplateHTML(layout string, status int) {
	opt := render.HTMLOptions{
		Layout: "",
	}
	err := flow.Renderer.HTML(flow.respWrt, status, layout, flow.bucket.vars, opt)
	flow.catchAfterErr(err)
}

// func (flow *flowCtx) BroadcastToCurrentSite(t string, data interface{}) error {
// 	s := strconv.Itoa(flow.SiteID())
// 	return flow.Broadcast("room-"+s, t, data)
// }

// func (flow *flowCtx) BroadcastToSite(siteID int, t string, data interface{}) error {
// 	s := strconv.Itoa(siteID)
// 	return flow.Broadcast("room-"+s, t, data)
// }

// func (flow *flowCtx) Broadcast(room string, t string, data interface{}) error {
// 	bc := &broadcast{
// 		Type: strings.Title(t),
// 		Data: data,
// 	}
// 	b, err := json.Marshal(bc)
// 	if err != nil {
// 		return err
// 	}
// 	err = flow.Store.Websocket.Broadcast(room, string(b))
// 	return err
// }

// type broadcast struct {
// 	Type string
// 	Data interface{}
// }

// func (flow *flowCtx) SPA(status int, pageInfo *PageInfo, data interface{}) {
// 	pageInfo.DocumentTitle = pageInfo.Title
// 	if pageInfo.SiteInfo != nil {
// 		pageInfo.DocumentTitle = pageInfo.Title + " - " + pageInfo.SiteInfo.Tagline + " - " + pageInfo.SiteInfo.Sitename
// 	}
// 	// logrus.Info(strings.ToLower(flow.req.Header.Get("Accept")))
// 	if strings.Contains(strings.ToLower(flow.req.Header.Get("Accept")), "application/json") {
// 		flow.JSON(status, data)
// 	} else {
// 		url := flow.req.URL
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
// 		flow.respWrt.Header().Set("Content-Type", "text/html")
// 		flow.respWrt.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
//_, 		err = flow.respWrt.Write(buf.Bytes())
// flow.catchAfterErr(err)
// 	}
// }

// func (flow *flowCtx) Render(status int, buffer *bytes.Buffer) {
//_, 	err = flow.respWrt.WriteHeader(status)
// flow.catchAfterErr(err)
//_, 	err = flow.respWrt.Write(buffer.Bytes())
// flow.catchAfterErr(err)
// }

// func (flow *flowCtx) RenderPDF(bytes []byte) {
// 	flow.respWrt.Header().Set("Content-Type", "application/PDF")
// 	flow.respWrt.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
//_,	err = flow.respWrt.Write(bytes)
// flow.catchAfterErr(err)
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

// func NewBucket(flow *flowCtx) *ViewBucket {
// 	viewBag := ViewBucket{}
// 	viewBag.w = flow.respWrt
// 	viewBag.req = flow.req
// 	viewBag.store = flow.Store
// 	viewBag.renderer = flow.Renderer
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
