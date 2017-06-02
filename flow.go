package context

import (
	"net/http"
	"strconv"

	"github.com/go-zoo/bone"
	"github.com/nerdynz/datastore"
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
