package main

// #include "cef_bridge.h"
//
// #define KNOSSOS_LOG_DEBUG 1
// #define KNOSSOS_LOG_INFO 2
// #define KNOSSOS_LOG_WARNING 3
// #define KNOSSOS_LOG_ERROR 4
// #define KNOSSOS_LOG_FATAL 5
//
// EXTERN void KnossosFreeKnossosResponse(KnossosResponse* response);
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/rotisserie/eris"
	"google.golang.org/protobuf/proto"

	"github.com/ngld/knossos/packages/api/client"
	"github.com/ngld/knossos/packages/api/common"
	"github.com/ngld/knossos/packages/libarchive"
	"github.com/ngld/knossos/packages/libknossos/pkg/api"
	"github.com/ngld/knossos/packages/libknossos/pkg/helpers"
	"github.com/ngld/knossos/packages/libknossos/pkg/storage"
	"github.com/ngld/knossos/packages/libknossos/pkg/twirp"
)

var (
	ready       = false
	logLevelMap = map[api.LogLevel]C.uint8_t{
		api.LogDebug: C.KNOSSOS_LOG_DEBUG,
		api.LogInfo:  C.KNOSSOS_LOG_INFO,
		api.LogWarn:  C.KNOSSOS_LOG_WARNING,
		api.LogError: C.KNOSSOS_LOG_ERROR,
		api.LogFatal: C.KNOSSOS_LOG_FATAL,
	}

	staticRoot   string
	settingsPath string
	logCb        C.KnossosLogCallback
	messageCb    C.KnossosMessageCallback
	server       http.Handler
)

func makeResponse() *C.KnossosResponse {
	return (*C.KnossosResponse)(C.malloc(C.size_t(unsafe.Sizeof(C.KnossosResponse{}))))
}

func Log(level api.LogLevel, msg string, args ...interface{}) {
	finalMsg := fmt.Sprintf(msg, args...)
	cMsg := C.CString(finalMsg)

	C.call_log_cb(logCb, logLevelMap[level], cMsg, C.int(len(finalMsg)))
	C.free(unsafe.Pointer(cMsg))
}

// KnossosInit has to be called exactly once before calling any other exported function.
//export KnossosInit
func KnossosInit(params *C.KnossosInitParams) bool {
	staticRoot = C.GoStringN(params.resource_path, params.resource_len)
	settingsPath = C.GoStringN(params.settings_path, params.settings_len)
	logCb = params.log_cb
	messageCb = params.message_cb
	ready = true

	var err error
	server, err = twirp.NewServer()
	if err != nil {
		Log(api.LogError, "Failed to init twirp: %+v", err)
		return false
	}

	Log(api.LogInfo, "Loading settings from %s", settingsPath)

	ctx := api.WithKnossosContext(context.Background(), api.KnossosCtxParams{
		SettingsPath: settingsPath,
		ResourcePath: staticRoot,
		LogCallback:  Log,
	})
	err = storage.Open(ctx)
	if err != nil {
		Log(api.LogError, "Failed to open the DB: %+v", err)
	}

	err = helpers.Init(ctx)
	if err != nil {
		Log(api.LogError, "Failed to init runtime: %+v", err)
	}

	Log(api.LogInfo, "LibArchive version: %d", libarchive.Version())

	return true
}

func serveRequest(ctx context.Context, twirpResp *memoryResponse, req *http.Request) {
	defer func() {
		r := recover()
		if r != nil {
			err, ok := r.(error)
			if !ok {
				err = errors.New(fmt.Sprint(r))
			}
			err = eris.Wrap(err, "Most recent call last:\n")

			api.Log(ctx, api.LogError, "panic for request %s: %s", req.URL, eris.ToString(err, true))

			// Write a more detailed error to the response body
			twirpResp.resp.Reset()

			msg := fmt.Sprintf("panic for request %s: %s", req.URL, eris.ToString(err, true))
			encodedMsg, err := json.Marshal(msg)
			if err == nil {
				twirpResp.resp.WriteString(`{"code":"internal","meta":{"cause":"panic"},"msg":`)
				twirpResp.resp.Write(encodedMsg)
				twirpResp.resp.WriteString("}")
			}
		}
	}()

	server.ServeHTTP(twirpResp, req)
}

//nolint:golint // golint doesn't understand cgo
func handleLocalFile(fileRef *common.FileRef) (*C.KnossosResponse, error) {
	localPath := ""
	for _, item := range fileRef.Urls {
		if strings.HasPrefix(item, "file://") {
			localPath = filepath.FromSlash(item[7:])
			break
		}
	}

	if localPath != "" {
		data, err := ioutil.ReadFile(localPath)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to read %s", localPath)
		}

		resp := makeResponse()
		resp.status_code = C.int(200)
		resp.header_count = C.uint8_t(0)

		if len(data) > 0 {
			resp.response_data = C.CBytes(data)
		}
		resp.response_length = C.size_t(len(data))
		return resp, nil
	} else {
		resp := makeResponse()
		resp.status_code = C.int(404)
		resp.header_count = C.uint8_t(0)
		resp.response_length = 0
		return resp, nil
	}
}

// KnossosHandleRequest handles an incoming request from CEF
//export KnossosHandleRequest
//nolint:golint // golint doesn't understand cgo
func KnossosHandleRequest(urlPtr *C.char, urlLen C.int, bodyPtr unsafe.Pointer, bodyLen C.int) *C.KnossosResponse {
	var body []byte
	if bodyLen > 0 {
		body = C.GoBytes(bodyPtr, bodyLen)
	}
	reqURL := C.GoStringN(urlPtr, urlLen)

	ctx, cancel := context.WithCancel(context.Background())

	ctx = api.WithKnossosContext(ctx, api.KnossosCtxParams{
		SettingsPath:    settingsPath,
		ResourcePath:    staticRoot,
		LogCallback:     Log,
		MessageCallback: DispatchMessage,
	})

	var err error
	if strings.HasPrefix(reqURL, "https://api.client.fsnebula.org/ref/") {
		cancel()

		var fileRef *common.FileRef
		fileId := reqURL[36:]
		fileRef, err = storage.GetFile(ctx, fileId)
		if err == nil {
			var resp *C.KnossosResponse
			resp, err = handleLocalFile(fileRef)
			if err == nil {
				return resp
			}
		}
	}

	if err == nil {
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
		req.Header["Content-Type"] = []string{"application/protobuf"}

		if err == nil {
			twirpResp := newMemoryResponse()

			serveRequest(ctx, twirpResp, req)

			// Cancel any background operation still attached to the request context
			cancel()

			resp := makeResponse()
			resp.status_code = C.int(twirpResp.statusCode)
			resp.header_count = C.uint8_t(len(twirpResp.headers))
			resp.headers = C.make_header_array(resp.header_count)

			idx := C.uint8_t(0)
			for k, v := range twirpResp.headers {
				hdr := &((*[256]C.KnossosHeader)(unsafe.Pointer(resp.headers)))[idx]
				hdr.header_len = C.size_t(len(k))
				hdr.header_name = C.CString(k)

				value := strings.Join(v, ", ")
				hdr.value_len = C.size_t(len(value))
				hdr.value = C.CString(value)

				idx++
			}

			body := twirpResp.resp.Bytes()
			if len(body) > 0 {
				resp.response_data = C.CBytes(body)
			}
			resp.response_length = C.size_t(len(body))

			return resp
		}
	}

	// Cleanup the unused context
	cancel()

	resp := makeResponse()
	resp.status_code = 503
	resp.header_count = 1
	hdr := C.make_header_array(1)
	resp.headers = hdr

	hdr.header_len = C.size_t(len("Content-Type"))
	hdr.header_name = C.CString("Content-Type")

	hdr.value_len = C.size_t(len("text/plain"))
	hdr.value = C.CString("text/plain")

	response := fmt.Sprintf("Error: %+v", err)
	resp.response_length = C.size_t(len(response))
	resp.response_data = unsafe.Pointer(C.CString(response))

	return resp
}

// DispatchMessage forwards the passed message to the hosting application
func DispatchMessage(event *client.ClientSentEvent) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return eris.Wrap(err, "Failed to marshal event")
	}

	dataPtr := C.CBytes(data)
	C.call_message_cb(messageCb, dataPtr, C.int(len(data)))
	C.free(dataPtr)
	return nil
}

func main() {}
