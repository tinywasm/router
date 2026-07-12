package mock

import (
	"bytes"
	"sync"

	"github.com/tinywasm/router"
)

// Context buffers the response and lets tests set the request fields.
// Unlike the base router.Context contract (single-goroutine ownership),
// this mock IS safe for concurrent use: its role is to let the test
// goroutine observe (ResponseBody) while a handler goroutine writes.
type Context struct {
	InMethod string
	InPath   string
	InBody   []byte

	Status   int
	mu       sync.RWMutex
	response bytes.Buffer
	headers  map[string]string
	cookies  map[string]router.Cookie
	values   map[string]any
	userID   string
}

// ResponseBody devuelve una copia del cuerpo de respuesta bufferizado.
func (c *Context) ResponseBody() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Devuelve una copia para evitar races con escrituras posteriores.
	src := c.response.Bytes()
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func (c *Context) Method() string {
	if c.InMethod != "" {
		return c.InMethod
	}
	return "GET"
}

func (c *Context) Path() string {
	if c.InPath != "" {
		return c.InPath
	}
	return "/"
}

func (c *Context) Body() []byte {
	return c.InBody
}

func (c *Context) GetHeader(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.headers == nil {
		return ""
	}
	return c.headers[key]
}

func (c *Context) SetHeader(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

func (c *Context) WriteStatus(code int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Status = code
}

func (c *Context) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.response.Write(b)
}

func (c *Context) SetValue(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = v
}

func (c *Context) Value(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.values == nil {
		return nil
	}
	return c.values[key]
}

func (c *Context) SetCookie(cookie router.Cookie) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cookies == nil {
		c.cookies = make(map[string]router.Cookie)
	}
	c.cookies[cookie.Name] = cookie
}

func (c *Context) Cookie(name string) (router.Cookie, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cookies == nil {
		return router.Cookie{}, false
	}
	cookie, ok := c.cookies[name]
	return cookie, ok
}

func (c *Context) SetUserID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userID = id
}

func (c *Context) UserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userID
}

var _ router.Context = (*Context)(nil)
