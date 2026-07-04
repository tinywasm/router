package router

// Cookie es la representación isomórfica de una cookie HTTP. No referencia
// net/http: cada implementador concreto la mapea a su transporte (net/http.Cookie
// en nativo; cabecera Set-Cookie en edge/wasm).
type Cookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	MaxAge   int  // >0 segundos; 0 = sesión; <0 = borrar ahora
	Secure   bool
	HttpOnly bool
	SameSite SameSite
}

// SameSite tipa la política SameSite — estado ilegal no representable (no un string).
type SameSite int

const (
	SameSiteDefault SameSite = iota
	SameSiteLax
	SameSiteStrict
	SameSiteNone
)
