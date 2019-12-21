package httpContext

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"
)

type Response struct {
	http.ResponseWriter
	req *http.Request
	// 是否调用过Write
	Started bool
	// 回复状态
	status      int
	Gzip        *gzip.Writer
	IsOpenGzip  bool
	NeedGzipLen int
	isGzip      bool
	MsgData     map[string]interface{}
	//Cookies []*http.Cookie

	sessionFunc        ISession
	session            map[string]interface{}
	isGetSession       bool
	SessionName        string
	SessionCookieName  string
	SessionAliveTime   time.Duration
	isUpdateSessionKey bool
	sessionIsUpdate    bool
}

// Session接口
type ISession interface {
	GetSession(KeyValue string) (map[string]interface{}, error)
	GetSessionKeyValue() (string, error)
	SetSession(SessionName string, m map[string]interface{}, duration time.Duration) error
	UpdateDataTime(SessionName string, duration time.Duration) error
}

func (r *Response) getSessionKeyValue() (string, error) {
	r.isUpdateSessionKey = true
	return r.sessionFunc.GetSessionKeyValue()
}

func (r *Response) RemoveSession() {
	r.session = nil
	r.isGetSession = false
	r.sessionIsUpdate = false
	r.sessionFunc.UpdateDataTime(r.SessionName, time.Duration(0))
	r.SessionName = ""
}

func (r *Response) GetSession(key string) (interface{}, bool) {
	if r.SessionName == "" {
		return nil, false
	}
	if !r.isGetSession {
		var err error
		if err = r.getSession(); err == nil {
			r.isGetSession = true
			if r.session == nil {
				return nil, false
			}
		}
	}
	v, ok := r.session[key]
	return v, ok
}

func (r *Response) getSession() error {
	if r.SessionName == "" {
		return errors.New("没有设置Session")
	}
	var err error
	if !r.isGetSession {
		if r.session, err = r.sessionFunc.GetSession(r.SessionName); err == nil {
			if r.session == nil {
				r.SessionName = ""
			}
			r.isGetSession = true
		}
	}
	return err
}

func (r *Response) UpdateSession() error {
	if r.SessionName == "" {
		return nil
	}
	if r.sessionIsUpdate {
		return r.sessionFunc.SetSession(r.SessionName, r.session, r.SessionAliveTime)
	} else {
		return r.sessionFunc.UpdateDataTime(r.SessionName, r.SessionAliveTime)
	}
}

func (r *Response) SetSession(key string, val interface{}) {
	if r.SessionName == "" {
		if SessionName, err := r.sessionFunc.GetSessionKeyValue(); err == nil {
			r.SessionName = SessionName
		}
	}
	r.sessionIsUpdate = true
	r.getSession()
	r.session[key] = val
}

// 设置Cookie
func (r *Response) SetCookie(c *http.Cookie) {
	http.SetCookie(r, c)
}

// 设置Cookie
func (r *Response) SetCookieUseKeyValue(key string, val string) {
	http.SetCookie(r, &http.Cookie{Name: key, Value: val})
}

// 通过cookie名字移除Cookie
func (r *Response) RemoveCookieByName(name string) {
	if ck, err := r.req.Cookie(name); err != http.ErrNoCookie {
		ck.Expires = time.Now()
		http.SetCookie(r, ck)
	}
}
func (r *Response) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *Response) ReadStatusCode() int {
	return r.status
}

// 通过cookie移除Cookie
func (r *Response) RemoveCookie(ck *http.Cookie) {
	ck.Expires = time.Now()
	http.SetCookie(r, ck)
}

type buf struct {
	r *Response
}

func (buf *buf) Write(p []byte) (n int, err error) {
	return buf.r.ResponseWriter.Write(p)
}

// 写入前端的数据
func (r *Response) Write(b []byte) (int, error) {
	defer func() {
		r.Started = true
	}()
	if r.isGzip || (r.IsOpenGzip && r.NeedGzipLen < len(b) && !r.Started) {
		if !r.Started {
			if r.SessionName != "" && r.isUpdateSessionKey {
				r.SetSession(r.SessionCookieName, r.SessionName)
			}
			r.isGzip = true
			r.ResponseWriter.Header().Set("Content-Encoding", "gzip")
			if r.ResponseWriter.Header().Get("Content-Type") == "" {
				r.ResponseWriter.Header().Set("Content-Type", http.DetectContentType(b))
			}
			r.ResponseWriter.WriteHeader(r.status)
		}
		if r.Gzip == nil {
			buf := buf{r: r}
			r.Gzip = gzip.NewWriter(&buf)
		}
		return r.Gzip.Write(b)
	}
	if !r.Started {
		if r.SessionName != "" && r.isUpdateSessionKey {
			r.SetSession(r.SessionCookieName, r.SessionName)
		}
		r.ResponseWriter.WriteHeader(r.status)
	}
	return r.ResponseWriter.Write(b)
}

// 写入前端的json数据
func (r *Response) WriteJson(i interface{}) error {
	if b, err := json.Marshal(i); err == nil {
		contentType := http.DetectContentType(b)
		f := regexp.MustCompile(`(;[\ ]?charset=.*)`)
		t := f.FindAllStringSubmatch(contentType, 1)
		contentType = "application/json"
		if len(t) > 0 && len(t[0]) > 0 {
			contentType = contentType + t[0][0]
		}
		r.ResponseWriter.Header().Set("Content-Type", contentType)
		_, err = r.Write(b)
		return err
	} else {
		return err
	}
}

func (r *Response) Redirect(redirectPath string) {
	http.Redirect(r, r.req, redirectPath, 302)
}
