package mock

import (
	"bytes"

	"github.com/tinywasm/router"
)

// Context bufferiza la respuesta y deja fijar la petición desde el test.
type Context struct {
	InMethod string
	InPath   string
	InBody   []byte

	Status   int
	response bytes.Buffer
	headers  map[string]string
	cookies  map[string]router.Cookie
	values   map[string]any
}

// ResponseBody devuelve el cuerpo de respuesta bufferizado.
func (c *Context) ResponseBody() []byte {
	return c.response.Bytes()
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
	if c.headers == nil {
		return ""
	}
	return c.headers[key]
}

func (c *Context) SetHeader(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

func (c *Context) WriteStatus(code int) {
	c.Status = code
}

func (c *Context) Write(b []byte) (int, error) {
	return c.response.Write(b)
}

func (c *Context) SetValue(key string, v any) {
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = v
}

func (c *Context) Value(key string) any {
	if c.values == nil {
		return nil
	}
	return c.values[key]
}

func (c *Context) SetCookie(cookie router.Cookie) {
	if c.cookies == nil {
		c.cookies = make(map[string]router.Cookie)
	}
	c.cookies[cookie.Name] = cookie
}

func (c *Context) Cookie(name string) (router.Cookie, bool) {
	if c.cookies == nil {
		return router.Cookie{}, false
	}
	cookie, ok := c.cookies[name]
	return cookie, ok
}

var _ router.Context = (*Context)(nil)
