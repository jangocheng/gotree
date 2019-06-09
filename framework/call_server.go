// Copyright gotree Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License")
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
)

var trailingPort = regexp.MustCompile(`:([0-9]+)$`)

var osDefaultInheritEnv = map[string][]string{
	"darwin":  {"DYLD_LIBRARY_PATH"},
	"freebsd": {"LD_LIBRARY_PATH"},
	"hpux":    {"LD_LIBRARY_PATH", "SHLIB_PATH"},
	"irix":    {"LD_LIBRARY_PATH", "LD_LIBRARYN32_PATH", "LD_LIBRARY64_PATH"},
	"linux":   {"LD_LIBRARY_PATH"},
	"openbsd": {"LD_LIBRARY_PATH"},
	"solaris": {"LD_LIBRARY_PATH", "LD_LIBRARY_PATH_32", "LD_LIBRARY_PATH_64"},
	"windows": {"SystemRoot", "COMSPEC", "PATHEXT", "WINDIR"},
}

type gotreeHandler struct {
	Path string
	Root string
	Dir  string

	Env                       []string // extra environment variables to set, if any, as "key=value"
	InheritEnv                []string // environment variables to inherit from host, as "key"
	Args                      []string
	Stderr                    io.Writer
	PathLocationgotreeHandler http.Handler
}

func (h *gotreeHandler) stderr() io.Writer {
	if h.Stderr != nil {
		return h.Stderr
	}
	return os.Stderr
}

func (h *gotreeHandler) Serve(rw http.ResponseWriter, req *http.Request) {
	root := h.Root
	if root == "" {
		root = "/"
	}

	if len(req.TransferEncoding) > 0 && req.TransferEncoding[0] == "chunked" {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Chunked request bodies are not supported by CGI."))
		return
	}

	pathInfo := req.URL.Path
	if root != "/" && strings.HasPrefix(pathInfo, root) {
		pathInfo = pathInfo[len(root):]
	}

	port := "80"
	if matches := trailingPort.FindStringSubmatch(req.Host); len(matches) != 0 {
		port = matches[1]
	}

	env := []string{
		"SERVER_SOFTWARE=go",
		"SERVER_NAME=" + req.Host,
		"SERVER_PROTOCOL=GOTREE_RPC",
		"RPC_HOST=" + req.Host,
		"GATEWAY_INTERFACE=GOTREE_RPC",
		"REQUEST_METHOD=" + req.Method,
		"QUERY_STRING=" + req.URL.RawQuery,
		"REQUEST_URI=" + req.URL.RequestURI(),
		"PATH_INFO=" + pathInfo,
		"SCRIPT_NAME=" + root,
		"SCRIPT_FILENAME=" + h.Path,
		"SERVER_PORT=" + port,
	}

	if remoteIP, remotePort, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		env = append(env, "REMOTE_ADDR="+remoteIP, "REMOTE_HOST="+remoteIP, "REMOTE_PORT="+remotePort)
	} else {
		// could not parse ip:port, let's use whole RemoteAddr and leave REMOTE_PORT undefined
		env = append(env, "REMOTE_ADDR="+req.RemoteAddr, "REMOTE_HOST="+req.RemoteAddr)
	}

	if req.TLS != nil {
		env = append(env, "HTTPS=on")
	}

	for k, v := range req.Header {
		k = strings.Map(upperCaseAndUnderscore, k)
		if k == "PROXY" {
			// See Issue 16405
			continue
		}
		joinStr := ", "
		if k == "COOKIE" {
			joinStr = "; "
		}
		env = append(env, "HTTP_"+k+"="+strings.Join(v, joinStr))
	}

	if req.ContentLength > 0 {
		env = append(env, fmt.Sprintf("CONTENT_LENGTH=%d", req.ContentLength))
	}
	if ctype := req.Header.Get("Content-Type"); ctype != "" {
		env = append(env, "CONTENT_TYPE="+ctype)
	}

	envPath := os.Getenv("PATH")
	if envPath == "" {
		envPath = "/bin:/usr/bin:/usr/ucb:/usr/bsd:/usr/local/bin"
	}
	env = append(env, "PATH="+envPath)

	for _, e := range h.InheritEnv {
		if v := os.Getenv(e); v != "" {
			env = append(env, e+"="+v)
		}
	}

	for _, e := range osDefaultInheritEnv[runtime.GOOS] {
		if v := os.Getenv(e); v != "" {
			env = append(env, e+"="+v)
		}
	}

	if h.Env != nil {
		env = append(env, h.Env...)
	}

	env = removeLeadingDuplicates(env)

	var cwd, path string
	if h.Dir != "" {
		path = h.Path
		cwd = h.Dir
	} else {
		cwd, path = filepath.Split(h.Path)
	}
	if cwd == "" {
		cwd = "."
	}

	internalError := func(err error) {
		rw.WriteHeader(http.StatusInternalServerError)
		h.printf("CGI error: %v", err)
	}

	cmd := &exec.Cmd{
		Path:   path,
		Args:   append([]string{h.Path}, h.Args...),
		Dir:    cwd,
		Env:    env,
		Stderr: h.stderr(),
	}
	if req.ContentLength != 0 {
		cmd.Stdin = req.Body
	}
	stdoutRead, err := cmd.StdoutPipe()
	if err != nil {
		internalError(err)
		return
	}

	err = cmd.Start()
	if err != nil {
		internalError(err)
		return
	}
	if hook := testHookStartProcess; hook != nil {
		hook(cmd.Process)
	}
	defer cmd.Wait()
	defer stdoutRead.Close()

	linebody := bufio.NewReaderSize(stdoutRead, 1024)
	headers := make(http.Header)
	statusCode := 0
	headerLines := 0
	sawBlankLine := false
	for {
		line, isPrefix, err := linebody.ReadLine()
		if isPrefix {
			rw.WriteHeader(http.StatusInternalServerError)
			h.printf("cgi: long header line from subprocess.")
			return
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			h.printf("cgi: error reading headers: %v", err)
			return
		}
		if len(line) == 0 {
			sawBlankLine = true
			break
		}
		headerLines++
		parts := strings.SplitN(string(line), ":", 2)
		if len(parts) < 2 {
			h.printf("cgi: bogus header line: %s", string(line))
			continue
		}
		header, val := parts[0], parts[1]
		header = strings.TrimSpace(header)
		val = strings.TrimSpace(val)
		switch {
		case header == "Status":
			if len(val) < 3 {
				h.printf("cgi: bogus status (short): %q", val)
				return
			}
			code, err := strconv.Atoi(val[0:3])
			if err != nil {
				h.printf("cgi: bogus status: %q", val)
				h.printf("cgi: line was %q", line)
				return
			}
			statusCode = code
		default:
			headers.Add(header, val)
		}
	}
	if headerLines == 0 || !sawBlankLine {
		rw.WriteHeader(http.StatusInternalServerError)
		h.printf("cgi: no headers")
		return
	}

	if loc := headers.Get("Location"); loc != "" {
		if strings.HasPrefix(loc, "/") && h.PathLocationgotreeHandler != nil {
			h.handleInternalRedirect(rw, req, loc)
			return
		}
		if statusCode == 0 {
			statusCode = http.StatusFound
		}
	}

	if statusCode == 0 && headers.Get("Content-Type") == "" {
		rw.WriteHeader(http.StatusInternalServerError)
		h.printf("cgi: missing required Content-Type in headers")
		return
	}

	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	// Copy headers to rw's headers, after we've decided not to
	// go into handleInternalRedirect, which won't want its rw
	// headers to have been touched.
	for k, vv := range headers {
		for _, v := range vv {
			rw.Header().Add(k, v)
		}
	}

	rw.WriteHeader(statusCode)

	_, err = io.Copy(rw, linebody)
	if err != nil {
		h.printf("cgi: copy error: %v", err)
		cmd.Process.Kill()
	}
}

func (h *gotreeHandler) handleInternalRedirect(rw http.ResponseWriter, req *http.Request, path string) {
	url, err := req.URL.Parse(path)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		h.printf("cgi: error resolving local URI path %q: %v", path, err)
		return
	}
	// TODO: RFC 3875 isn't clear
	// suggests so: "Note that any message-body attached to the
	// method or just GET? What about incoming headers?
	// (e.g. Cookies) Which headers, if any, are copied into the
	// second request?
	newReq := &http.Request{
		Method:     "GET",
		URL:        url,
		Proto:      "GOTREE_RPC",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       url.Host,
		RemoteAddr: req.RemoteAddr,
		TLS:        req.TLS,
	}
	h.PathLocationgotreeHandler.ServeHTTP(rw, newReq)
}

func upperCaseAndUnderscore(r rune) rune {
	switch {
	case r >= 'a' && r <= 'z':
		return r - ('a' - 'A')
	case r == '-':
		return '_'
	case r == '=':
		// Maybe not part of the 'spec' but would mess up
		// the environment in any case, as Go represents the
		// environment as a slice of "key=value" strings.
		return '_'
	}
	// TODO: other transformations in spec or practice?
	return r
}

func (h *gotreeHandler) printf(format string, v ...interface{}) {
	Log().Notice(v...)
}

var testHookStartProcess func(*os.Process) // nil except for some tests

type GotreeService struct {
	GotreeBase
	sock   net.Listener
	goRpc  *Server
	status int32
}

func (this *GotreeService) Gotree() *GotreeService {
	this.GotreeBase.Gotree(this)
	this.status = 0
	this.goRpc = NewServer()
	this.goRpc.gs = this
	_runtimeStack = this.goRpc.runStack
	return this
}

// removeLeadingDuplicates remove leading duplicate in environments.
func removeLeadingDuplicates(env []string) (ret []string) {
	for i, e := range env {
		found := false
		if eq := strings.IndexByte(e, '='); eq != -1 {
			keq := e[:eq+1] // "key="
			for _, e2 := range env[i+1:] {
				if strings.HasPrefix(e2, keq) {
					found = true
					break
				}
			}
		}
		if !found {
			ret = append(ret, e)
		}
	}
	return
}

func (this *GotreeService) Run(addr string) {
	ip := strings.Split(addr, ":")[0]
	port, _ := strconv.Atoi(strings.Split(addr, ":")[1])
	for index := 0; index < 10; index++ {
		var err error
		this.sock, err = net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port+index))
		if err != nil && index == 9 {
			Log().Error(err.Error())
			panic(err.Error())
		}
		if err == nil {
			port += index
			Log().Notice("Bind addr:" + fmt.Sprintf("%s:%d", ip, port))
			break
		}
	}

	go this.accept()
	this.Event("ListenSuccess", strconv.Itoa(port))

	//Signal
	channel := make(chan os.Signal)
	signal.Notify(channel, syscall.SIGTERM, syscall.SIGINT, syscall.SIGSEGV, syscall.SIGFPE, syscall.SIGABRT, syscall.SIGILL)
	<-channel
	atomic.AddInt32(&this.status, 1)
	return
}

// callRequest returns the rpc request as represented in the current
// environment. This assumes the current program is being run
func callRequest() (*http.Request, error) {
	r, err := gotreeFromMap(callEnvMap(os.Environ()))
	if err != nil {
		return nil, err
	}
	if r.ContentLength > 0 {
		r.Body = ioutil.NopCloser(io.LimitReader(os.Stdin, r.ContentLength))
	}
	return r, nil
}

func callEnvMap(env []string) map[string]string {
	m := make(map[string]string)
	for _, kv := range env {
		if idx := strings.Index(kv, "="); idx != -1 {
			m[kv[:idx]] = kv[idx+1:]
		}
	}
	return m
}

func gotreeFromMap(params map[string]string) (*http.Request, error) {
	r := new(http.Request)
	r.Method = params["REQUEST_METHOD"]
	if r.Method == "" {
		return nil, errors.New("cgi: no REQUEST_METHOD in environment")
	}

	r.Proto = params["SERVER_PROTOCOL"]
	var ok bool
	r.ProtoMajor, r.ProtoMinor, ok = http.ParseHTTPVersion(r.Proto)
	if !ok {
		return nil, errors.New("cgi: invalid SERVER_PROTOCOL version")
	}

	r.Close = true
	r.Trailer = http.Header{}
	r.Header = http.Header{}

	r.Host = params["GOTREE_RPC_HOST"]

	if lenstr := params["CONTENT_LENGTH"]; lenstr != "" {
		clen, err := strconv.ParseInt(lenstr, 10, 64)
		if err != nil {
			return nil, errors.New("cgi: bad CONTENT_LENGTH in environment: " + lenstr)
		}
		r.ContentLength = clen
	}

	if ct := params["CONTENT_TYPE"]; ct != "" {
		r.Header.Set("Content-Type", ct)
	}

	for k, v := range params {
		if !strings.HasPrefix(k, "HTTP_") || k == "HTTP_HOST" {
			continue
		}
		r.Header.Add(strings.Replace(k[5:], "_", "-", -1), v)
	}

	// TODO: cookies

	uriStr := params["REQUEST_URI"]
	if uriStr == "" {
		// Fallback to SCRIPT_NAME, PATH_INFO and QUERY_STRING.
		uriStr = params["SCRIPT_NAME"] + params["PATH_INFO"]
		s := params["QUERY_STRING"]
		if s != "" {
			uriStr += "?" + s
		}
	}

	// There's apparently a de-facto standard for this.
	if s := params["GOTREE_RPC"]; s == "on" || s == "ON" || s == "1" {
		r.TLS = &tls.ConnectionState{HandshakeComplete: true}
	}

	if r.Host != "" {
		// Hostname is provided, so we can reasonably construct a.
		rawurl := r.Host + uriStr
		if r.TLS == nil {
			rawurl = "GOTREE_RPC://" + rawurl
		} else {
			rawurl = "GOTREE_RPC://" + rawurl
		}
		url, err := url.Parse(rawurl)
		if err != nil {
			return nil, errors.New("cgi: failed to parse host and REQUEST_URI into a URL: " + rawurl)
		}
		r.URL = url
	}
	// Fallback logic if we don't have a Host header or the URL
	// failed to parse
	if r.URL == nil {
		url, err := url.Parse(uriStr)
		if err != nil {
			return nil, errors.New("cgi: failed to parse REQUEST_URI into a URL: " + uriStr)
		}
		r.URL = url
	}

	// Request.RemoteAddr has its port set by Go's standard http
	// server, so we do here too.
	remotePort, _ := strconv.Atoi(params["REMOTE_PORT"]) // zero if unset or invalid
	r.RemoteAddr = net.JoinHostPort(params["REMOTE_ADDR"], strconv.Itoa(remotePort))

	return r, nil
}

// CancelController
func (this *GotreeService) CancelController(name string) {
	if name == "Health" {
		return
	}
	this.goRpc.UnRegister(name)
}

// RegController
func (this *GotreeService) RegController(controller interface{}) {
	type rname interface {
		CallName_() string
	}

	regname, ok := controller.(rname)
	if !ok {
		Exit("RegController:registration failed:" + reflect.TypeOf(controller).String())
	}

	err := this.goRpc.RegisterName(regname.CallName_(), controller)
	if err != nil {
		Exit(err.Error())
	}
}

func (this *GotreeService) Close() {
	atomic.AddInt32(&this.status, 1)
	if this.sock == nil {
		return
	}
	this.sock.Close()
	this.goRpc.Close()
}

func (this *GotreeService) accept() {
	for {
		conn, err := this.sock.Accept()
		if err != nil {
			continue
		}

		if atomic.LoadInt32(&this.status) == 2 {
			conn.Close()
			continue
		}
		go this.goRpc.ServeCodec(NewServerCodec(conn))
	}
}

// Actions
func (this *GotreeService) Actions() []string {
	return this.goRpc.ServiceNameList()
}

func (this *GotreeService) Runtime() RuntimeStack {
	return this.goRpc.runStack
}

func (this *GotreeService) Calls() int {
	return this.goRpc.runStack.Calls()
}

func (this *GotreeService) Hook(key string, value interface{}) {
	this.goRpc.runStack.Set(key, value)
}
