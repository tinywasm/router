package router

// Cookie is the isomorphic representation of an HTTP cookie. It does not
// reference net/http: each concrete implementer maps it to its transport
// (net/http.Cookie on native; Set-Cookie header on edge/wasm).
type Cookie struct {
	Name     string   // e.g. "session_id", "user_pref"
	Value    string   // e.g. "abc123xyz789"
	Path     string   // e.g. "/", "/api"; omit for "/"
	Domain   string   // e.g. "example.com"; omit for current domain
	MaxAge   int      // >0 seconds; 0 = session; <0 = delete now
	Secure   bool     // true = HTTPS only
	HttpOnly bool     // true = no JavaScript access
	SameSite SameSite // SameSiteLax, SameSiteStrict, SameSiteNone
}

// SameSite types the SameSite policy — illegal state not representable (not a string).
type SameSite int

const (
	SameSiteDefault SameSite = iota // browser default behavior
	SameSiteLax                      // cross-site requests send cookie (default modern behavior)
	SameSiteStrict                   // never send cookie cross-site
	SameSiteNone                     // send cookie in all contexts (requires Secure=true)
)
