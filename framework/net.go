// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package framework

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

const (
	// Defaults used by HandleHTTP
	DefaultRPCPath   = "/_goRPC_"
	DefaultDebugPath = "/debug/rpc"
)

// Precompute the reflect type for error. Can't use error directly
// because Typeof takes an empty interface value. This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

type methodType struct {
	sync.Mutex // protects counters
	method     reflect.Method
	ArgType    reflect.Type
	ReplyType  reflect.Type
	numCalls   uint
}

type service struct {
	name   string                 // name of service
	rcvr   reflect.Value          // receiver of methods for the service
	typ    reflect.Type           // type of the receiver
	method map[string]*methodType // registered methods
}

// Request is a header written before every RPC call. It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.
type Request struct {
	ServiceMethod string   // format: "Service.Method"
	Seq           uint64   // sequence number chosen by client
	next          *Request // for free list in Server
}

// Response is a header written before every RPC return. It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.
type Response struct {
	ServiceMethod string    // echoes that of the Request
	Seq           uint64    // echoes that of the request
	Error         string    // error, if any.
	next          *Response // for free list in Server
}

// Server represents an RPC Server.
type Server struct {
	mu          sync.RWMutex // protects the serviceMap
	serviceMap  map[string]*service
	reqLock     sync.Mutex // protects freeReq
	freeReq     *Request
	respLock    sync.Mutex // protects freeResp
	freeResp    *Response
	connmap     map[net.Conn]bool
	connmapRtex sync.RWMutex
	runStack    *RunStack
	gs          *GotreeService
}

// NewServer returns a new Server.
func NewServer() *Server {
	result := &Server{serviceMap: make(map[string]*service)}
	result.connmap = make(map[net.Conn]bool)
	result.runStack = new(RunStack).Gotree()
	return result
}

// DefaultServer is the default instance of *Server.
var DefaultServer = NewServer()

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// Register publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method of exported type
//	- two arguments, both of exported type
//	- the second argument is a pointer
//	- one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error using package log.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.
func (server *Server) Register(rcvr interface{}) error {
	return server.register(rcvr, "", false)
}

// RegisterName is like Register but uses the provided name for the type
// instead of the receiver's concrete type.
func (server *Server) RegisterName(name string, rcvr interface{}) error {
	return server.register(rcvr, name, true)
}

func (server *Server) UnRegister(name string) {
	delete(server.serviceMap, name)
}

func (server *Server) RegisterGotreeService(gs *GotreeService) {
	server.gs = gs
}

func (server *Server) ServiceNameList() []string {
	result := []string{}
	for k, _ := range server.serviceMap {
		result = append(result, k)
	}
	return result
}

func (server *Server) register(rcvr interface{}, name string, useName bool) error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.serviceMap == nil {
		server.serviceMap = make(map[string]*service)
	}
	s := new(service)
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	sname := reflect.Indirect(s.rcvr).Type().Name()
	if useName {
		sname = name
	}
	if sname == "" {
		s := "rpc.Register: no service name for type " + s.typ.String()
		//log.Print(s)
		return errors.New(s)
	}
	if !isExported(sname) && !useName {
		s := "rpc.Register: type " + sname + " is not exported"
		//log.Print(s)
		return errors.New(s)
	}
	if _, present := server.serviceMap[sname]; present {
		return errors.New("rpc: service already defined: " + sname)
	}
	s.name = sname

	// Install the methods
	s.method = suitableMethods(s.typ, true)

	if len(s.method) == 0 {
		str := ""

		// To help the user, see if a pointer receiver would work.
		method := suitableMethods(reflect.PtrTo(s.typ), false)
		if len(method) != 0 {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type"
		}
		//log.Print(str)
		return errors.New(str)
	}
	server.serviceMap[s.name] = s
	return nil
}

// suitableMethods returns suitable Rpc methods of typ, it will report
// error using log if reportErr is true.
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *args, *reply.
		if mtype.NumIn() != 3 {
			if reportErr {
				//log.Println("method", mname, "has wrong number of ins:", mtype.NumIn())
			}
			continue
		}
		// First arg need not be a pointer.
		argType := mtype.In(1)
		if !isExportedOrBuiltinType(argType) {
			if reportErr {
				//log.Println(mname, "argument type not exported:", argType)
			}
			continue
		}
		// Second arg must be a pointer.
		replyType := mtype.In(2)
		if replyType.Kind() != reflect.Ptr {
			if reportErr {
				//log.Println("method", mname, "reply type not a pointer:", replyType)
			}
			continue
		}
		// Reply type must be exported.
		if !isExportedOrBuiltinType(replyType) {
			if reportErr {
				//log.Println("method", mname, "reply type not exported:", replyType)
			}
			continue
		}
		// Method needs one out.
		if mtype.NumOut() != 1 {
			if reportErr {
				//log.Println("method", mname, "has wrong number of outs:", mtype.NumOut())
			}
			continue
		}
		// The return type of the method must be error.
		if returnType := mtype.Out(0); returnType != typeOfError {
			if reportErr {
				//log.Println("method", mname, "returns", returnType.String(), "not error")
			}
			continue
		}
		methods[mname] = &methodType{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

// A value sent as a placeholder for the server's response value when the server
// receives an invalid request. It is never decoded by the client since the Response
// contains an error when it is used.
var invalidRequest = struct{}{}

func (server *Server) sendResponse(sending *sync.Mutex, req *Request, reply interface{}, codec ServerCodec, errmsg string) {
	resp := server.getResponse()
	// Encode the response header
	resp.ServiceMethod = req.ServiceMethod
	if errmsg != "" {
		resp.Error = errmsg
		reply = invalidRequest
	}
	resp.Seq = req.Seq
	sending.Lock()
	codec.WriteResponse(resp, reply)
	sending.Unlock()
	server.freeResponse(resp)
}

func (m *methodType) NumCalls() (n uint) {
	m.Lock()
	n = m.numCalls
	m.Unlock()
	return n
}

func (s *service) call(server *Server, sending *sync.Mutex, mtype *methodType, req *Request, argv, replyv reflect.Value, codec ServerCodec) {
	defer func() {
		if perr := recover(); perr != nil {
			Log().Error("This remote call is a critical error. The " + fmt.Sprint(perr))
			server.sendResponse(sending, req, replyv.Interface(), codec, "panic:"+fmt.Sprint(perr))
			server.freeRequest(req)
		}
		atomic.AddInt32(&server.runStack.calls, -1)
		server.runStack.Remove()
	}()
	mtype.Lock()
	mtype.numCalls++
	mtype.Unlock()
	atomic.AddInt32(&server.runStack.calls, 1)

	newtype := reflect.TypeOf(s.rcvr.Interface()).Elem()
	t := reflect.New(newtype)
	values := t.MethodByName("Gotree").Call(nil)
	if len(values) < 1 {
		Log().Error("This remote call is not implemented Gotree. The " + newtype.String())
	}
	type callip interface {
		CallInvoke_(net.Conn, *GotreeService)
		OnCreate(method string, argv interface{})
		OnDestory(method string, reply interface{}, e error)
	}
	ci, ok := values[0].Interface().(callip)
	var errInter interface{}
	gseq := ""
	for {
		if err := FieldEmpty(argv); err != nil {
			errInter = err
			break
		}
		if argv.Kind() == reflect.Struct && argv.FieldByName("Gseq").IsValid() {
			gseq = argv.FieldByName("Gseq").String()
		}
		if gseq != "" {
			server.runStack.Set("gseq", gseq)
		}
		if server.gs.status == 1 {
			errInter = errors.New("ShutDownIng")
			break
		}
		if server.gs.status == 2 {
			codec.ConnInterface().Close()
			errInter = errors.New("ShutDownIng")
			break
		}
		if ok {
			ci.CallInvoke_(codec.ConnInterface(), server.gs)
			ci.OnCreate(mtype.method.Name, argv.Interface())
		}

		head := ""
		if argv.Kind() == reflect.Struct && argv.FieldByName("Head").IsValid() {
			head = argv.FieldByName("Head").String()
		}
		if head != "" {
			server.runStack.Set("head", head)
		}
		returnValues := t.MethodByName(mtype.method.Name).Call([]reflect.Value{argv, replyv})
		// The return value for the method is an error.
		errInter = returnValues[0].Interface()
		break
	}

	errmsg := ""
	if errInter != nil {
		errmsg = errInter.(error).Error()
		if gseq == "" && errmsg != "ShutDownIng" {
			Log().Warning("Controller:"+newtype.Name(), "function:"+mtype.method.Name, "error:"+errmsg)
		}
	}

	if ok {
		var ferr error
		if errmsg != "" {
			ferr = errors.New(errmsg)
		}
		ci.OnDestory(mtype.method.Name, replyv, ferr)
	}
	server.sendResponse(sending, req, replyv.Interface(), codec, errmsg)
	server.freeRequest(req)
}

// ServeCodec is like ServeConn but uses the specified codec to
// decode requests and encode responses.
func (server *Server) ServeCodec(codec ServerCodec) {
	sending := new(sync.Mutex)
	server.connmapRtex.Lock()
	server.connmap[codec.ConnInterface()] = true
	server.connmapRtex.Unlock()
	for {
		service, mtype, req, argv, replyv, keepReading, err := server.readRequest(codec)
		if err != nil {
			if !keepReading {
				break
			}
			// send a response if we actually managed to read a header.
			if req != nil {
				server.sendResponse(sending, req, invalidRequest, codec, err.Error())
				server.freeRequest(req)
			}
			continue
		}
		go service.call(server, sending, mtype, req, argv, replyv, codec)
	}
	codec.Close()
	server.connmapRtex.Lock()
	delete(server.connmap, codec.ConnInterface())
	server.connmapRtex.Unlock()
}

func (server *Server) Close() {
	defer server.connmapRtex.Unlock()
	server.connmapRtex.Lock()
	for item, _ := range server.connmap {
		item.Close()
	}
}

// ServeRequest is like ServeCodec but synchronously serves a single request.
// It does not close the codec upon completion.
func (server *Server) ServeRequest(codec ServerCodec) error {
	sending := new(sync.Mutex)
	service, mtype, req, argv, replyv, keepReading, err := server.readRequest(codec)
	if err != nil {
		if !keepReading {
			return err
		}
		// send a response if we actually managed to read a header.
		if req != nil {
			server.sendResponse(sending, req, invalidRequest, codec, err.Error())
			server.freeRequest(req)
		}
		return err
	}
	service.call(server, sending, mtype, req, argv, replyv, codec)
	return nil
}

func (server *Server) getRequest() *Request {
	server.reqLock.Lock()
	req := server.freeReq
	if req == nil {
		req = new(Request)
	} else {
		server.freeReq = req.next
		*req = Request{}
	}
	server.reqLock.Unlock()
	return req
}

func (server *Server) freeRequest(req *Request) {
	server.reqLock.Lock()
	req.next = server.freeReq
	server.freeReq = req
	server.reqLock.Unlock()
}

func (server *Server) getResponse() *Response {
	server.respLock.Lock()
	resp := server.freeResp
	if resp == nil {
		resp = new(Response)
	} else {
		server.freeResp = resp.next
		*resp = Response{}
	}
	server.respLock.Unlock()
	return resp
}

func (server *Server) freeResponse(resp *Response) {
	server.respLock.Lock()
	resp.next = server.freeResp
	server.freeResp = resp
	server.respLock.Unlock()
}

func (server *Server) readRequest(codec ServerCodec) (service *service, mtype *methodType, req *Request, argv, replyv reflect.Value, keepReading bool, err error) {
	service, mtype, req, keepReading, err = server.readRequestHeader(codec)
	if err != nil {
		if !keepReading {
			return
		}
		// discard body
		codec.ReadRequestBody(nil)
		return
	}

	// Decode the argument value.
	argIsValue := false // if true, need to indirect before calling.
	if mtype.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(mtype.ArgType.Elem())
	} else {
		argv = reflect.New(mtype.ArgType)
		argIsValue = true
	}
	// argv guaranteed to be a pointer now.
	if err = codec.ReadRequestBody(argv.Interface()); err != nil {
		return
	}
	if argIsValue {
		argv = argv.Elem()
	}

	replyv = reflect.New(mtype.ReplyType.Elem())
	return
}

func (server *Server) readRequestHeader(codec ServerCodec) (service *service, mtype *methodType, req *Request, keepReading bool, err error) {
	// Grab the request header.
	req = server.getRequest()
	err = codec.ReadRequestHeader(req)
	if err != nil {
		req = nil
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = errors.New("rpc: server cannot decode request: " + err.Error())
		return
	}

	// We read the header successfully. If we see an error now,
	// we can still recover and move on to the next request.
	keepReading = true

	dot := strings.LastIndex(req.ServiceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc: service/method request ill-formed: " + req.ServiceMethod)
		return
	}
	serviceName := req.ServiceMethod[:dot]
	methodName := req.ServiceMethod[dot+1:]

	// Look up the request.
	server.mu.RLock()
	service = server.serviceMap[serviceName]
	server.mu.RUnlock()
	if service == nil {
		err = errors.New("rpc: can't find service " + req.ServiceMethod)
		return
	}
	mtype = service.method[methodName]
	if mtype == nil {
		err = errors.New("rpc: can't find method " + req.ServiceMethod)
	}
	return
}

var (
	_testing bool = false
)

func Testing() bool {
	return _testing
}

// Register publishes the receiver's methods in the DefaultServer.
func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

// RegisterName is like Register but uses the provided name for the type
// instead of the receiver's concrete type.
func RegisterName(name string, rcvr interface{}) error {
	return DefaultServer.RegisterName(name, rcvr)
}

func UnRegister(name string) {
	DefaultServer.UnRegister(name)
}

// A ServerCodec implements reading of RPC requests and writing of
// RPC responses for the server side of an RPC session.
// The server calls ReadRequestHeader and ReadRequestBody in pairs
// to read requests from the connection, and it calls WriteResponse to
// write a response back. The server calls Close when finished with the
// connection. ReadRequestBody may be called with a nil
// argument to force the body of the request to be read and discarded.
type ServerCodec interface {
	ReadRequestHeader(*Request) error
	ReadRequestBody(interface{}) error
	// WriteResponse must be safe for concurrent use by multiple goroutines.
	WriteResponse(*Response, interface{}) error
	ConnInterface() net.Conn
	Close() error
}

// ServeCodec is like ServeConn but uses the specified codec to
// decode requests and encode responses.
func ServeCodec(codec ServerCodec) {
	DefaultServer.ServeCodec(codec)
}

// ServeRequest is like ServeCodec but synchronously serves a single request.
// It does not close the codec upon completion.
func ServeRequest(codec ServerCodec) error {
	return DefaultServer.ServeRequest(codec)
}

// Can connect to RPC service using HTTP CONNECT to rpcPath.
var connected = "200 Connected to Go RPC"

func ParseStruct(t reflect.Type, v reflect.Value, def string, sn string) (sInds []reflect.Value, eTyps []reflect.Type, defs []string, structName []string) {
	if t.Kind() == reflect.Slice && v.Len() == 0 && def != "null" {
		sInds = append(sInds, v)
		eTyps = append(eTyps, t)
		defs = append(defs, def)
		structName = append(structName, sn)
		return
	}

	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		return
	}

	if t.Kind() == reflect.Slice {
		for index := 0; index < v.Len(); index++ {
			resultInds, resultTyps, resultDefs, resultStructName := ParseStruct(t.Elem(), v.Index(index), def, sn)
			sInds = append(sInds, resultInds...)
			eTyps = append(eTyps, resultTyps...)
			defs = append(defs, resultDefs...)
			structName = append(structName, resultStructName...)
		}
		return
	}

	if t.Kind() != reflect.Struct {
		sInds = append(sInds, v)
		eTyps = append(eTyps, t)
		defs = append(defs, def)
		structName = append(structName, sn)
		return
	}

	for index := 0; index < t.NumField(); index++ {
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		def := t.Field(index).Tag.Get("opt")
		strName := t.Field(index).Name
		if sn != "" {
			strName = sn + "." + strName
		}
		resultInds, resultTyps, resultDefs, resultStructName := ParseStruct(t.Field(index).Type, v.Field(index), def, strName)
		sInds = append(sInds, resultInds...)
		eTyps = append(eTyps, resultTyps...)
		defs = append(defs, resultDefs...)
		structName = append(structName, resultStructName...)
	}

	return
}

func FieldEmpty(v reflect.Value) error {
	var (
		sInds      []reflect.Value
		eTyps      []reflect.Type
		defs       []string
		structName []string
	)

	t := v.Type()
	sInds, eTyps, defs, structName = ParseStruct(t, v, "", "")
	for index := 0; index < len(eTyps); index++ {
		fieldValue := fmt.Sprint(sInds[index])
		if fieldValue != "" && fieldValue != "0" && fieldValue != "[]" {
			continue
		}
		def := defs[index]
		sn := structName[index]
		if def != "null" {
			return errors.New("Non-empty parameter checking " + t.Name() + "." + sn + "(" + eTyps[index].String() + ")=" + fieldValue)
		}
	}

	return nil
}

// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

var errMissingParams = errors.New("jsonrpc: request body missing params")

type serverCodec struct {
	dec  *json.Decoder // for reading JSON values
	enc  *json.Encoder // for writing JSON values
	c    io.Closer
	conn net.Conn

	// temporary work space
	req serverRequest

	// JSON-RPC clients can use arbitrary json values as request IDs.
	// Package rpc expects uint64 request IDs.
	// We assign uint64 sequence numbers to incoming requests
	// but save the original request ID in the pending map.
	// When rpc responds, we use the sequence number in
	// the response to find the original request ID.
	mutex   sync.Mutex // protects seq, pending
	seq     uint64
	pending map[uint64]*json.RawMessage
}

func NewServerCodec(conn net.Conn) ServerCodec {
	return &serverCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]*json.RawMessage),
		conn:    conn,
	}
}

type serverRequest struct {
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params"`
	Id     *json.RawMessage `json:"id"`
}

func (r *serverRequest) reset() {
	r.Method = ""
	r.Params = nil
	r.Id = nil
}

type serverResponse struct {
	Id     *json.RawMessage `json:"id"`
	Result interface{}      `json:"result"`
	Error  interface{}      `json:"error"`
}

func (c *serverCodec) ReadRequestHeader(r *Request) error {
	c.req.reset()
	if err := c.dec.Decode(&c.req); err != nil {
		return err
	}
	r.ServiceMethod = c.req.Method

	// JSON request id can be any JSON value;
	// RPC package expects uint64.  Translate to
	// internal uint64 and save JSON on the side.
	c.mutex.Lock()
	c.seq++
	c.pending[c.seq] = c.req.Id
	c.req.Id = nil
	r.Seq = c.seq
	c.mutex.Unlock()

	return nil
}

func (c *serverCodec) ConnInterface() net.Conn {
	return c.conn
}

func (c *serverCodec) ReadRequestBody(x interface{}) error {
	if x == nil {
		return nil
	}
	if c.req.Params == nil {
		return errMissingParams
	}
	// JSON params is array value.
	// RPC params is struct.
	// Unmarshal into array containing struct for now.
	// Should think about making RPC more general.
	var params [1]interface{}
	params[0] = x
	return json.Unmarshal(*c.req.Params, &params)
}

var null = json.RawMessage([]byte("null"))

func (c *serverCodec) WriteResponse(r *Response, x interface{}) error {
	c.mutex.Lock()
	b, ok := c.pending[r.Seq]
	if !ok {
		c.mutex.Unlock()
		return errors.New("invalid sequence number in response")
	}
	delete(c.pending, r.Seq)
	c.mutex.Unlock()

	if b == nil {
		// Invalid request so no id. Use JSON null.
		b = &null
	}
	resp := serverResponse{Id: b}
	if r.Error == "" {
		resp.Result = x
	} else {
		resp.Error = r.Error
	}
	return c.enc.Encode(resp)
}

func (c *serverCodec) Close() error {
	return c.c.Close()
}
