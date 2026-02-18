package server

import (
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type spaHandler struct {
	fs         fs.FS
	fileServer http.Handler
}

func newSPAHandler(frontend fs.FS) http.Handler {
	return &spaHandler{
		fs:         frontend,
		fileServer: http.FileServer(http.FS(frontend)),
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	reqPath := strings.TrimPrefix(r.URL.Path, "/")
	if reqPath == "" {
		reqPath = "index.html"
	}
	reqPath = path.Clean(reqPath)

	if fileExists(h.fs, reqPath) {
		h.fileServer.ServeHTTP(w, r)
		return
	}

	r2 := cloneRequest(r)
	r2.URL.Path = "/index.html"
	h.fileServer.ServeHTTP(w, r2)
}

func fileExists(fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return !st.IsDir()
}

func cloneRequest(r *http.Request) *http.Request {
	r2 := r.Clone(r.Context())
	if r.URL != nil {
		u := *r.URL
		r2.URL = &u
	} else {
		r2.URL = &url.URL{}
	}
	return r2
}
