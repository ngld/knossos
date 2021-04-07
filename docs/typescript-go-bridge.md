# TypeScript <-> Go bridge

This document describes the interface used by `client-ui` to call `libknossos`
code. The basis for this is the Twirp RPC framework. It was designed to work
over HTTP(S) connections but we can emulate those through custom logic in the
`client` package.

## Overview

All available RPC methods (and messages / structures) have to be declared in
`packages/api/definitions/client.proto`. Messages can be declared in imported
files (like `mod.proto`) as well.

RPC methods are usually called through `await gs.client.methodName(...)` in
`client-ui`. A more detailed example and explanation follows.

The methods are implemented in `packages/libknossos/pkg/twirp/*.go` and are
defined as methods on the `*knossosServer` type.

## Specific example: GetModInfo

`GetModInfo` is declared in `client.proto` with the following line:

```proto
rpc GetModInfo (ModInfoRequest) returns (ModInfoResponse) {};
```

Like all other RPC methods, it accepts one message as parameter and returns a
different message.

Messages are like `struct`s in C; arbitrarly complex data structures that can
hold strictly typed values. From the messages declared in our `*.proto` files,
language specific definitions are generated (`type ... struct { }` for Go and
`interface ... { }` for TypeScript). If your IDE supports auto-complete for Go
or TypeScript, it should recognize these types and handle them as expected.

A typical RPC call looks like this:

```typescript
const result = await gs.client.getModInfo({
  id: params.modid,
  version: params.version ?? '',
});
```

`gs.client` is a reference to the RPC client. You can retrieve a reference to
`gs` by calling `useGlobalState()` from [`lib/state.tsx`][state.tsx] in a React
component.

`getModInfo` is the RPC method being called. `id` and `version` are fields
declared in the `ModInfoRequest` message.

This call is dispatched to the Go implementation in libknossos which looks like
this:

```go
func (kn *knossosServer) GetModInfo(ctx context.Context, req *client.ModInfoRequest) (*client.ModInfoResponse, error) {
  ...
}
```

The `req` parameter contains the `id` and `version` fields passed from TypeScript.
Since exported fields have to begin with an uppercase letter, they are called
`Id` and `Version` but that's the only difference.

Once the `GetModInfo()` function completes, the result is returned to TypeScript
where it can be retrieved from `result.response` in our example.

## The gory technical details

The inner workings are a bit complicated since each call has to go from
TypeScript -> C++ -> Go and back. The messages also have to be passed between
processes because the web page is running in Chromium's sandbox.

To simplify the interaction between TypeScript and C++, that interface uses
HTTP requests as transport. All RPC calls are sent to
`https://api.client.fsnebula.org/twirp` (which doesn't actually exist).
This URL is hard coded in [`packages/client-ui/src/lib/state.tsx`][state.tsx].

This request is then intercepted in
[`packages/client/launcher/browser/knossos_handler.cc`][knossos_handler.cc]
`KnossosHandler::GetResourceRequestHandler` which forwards the request to
`KnossosResourceHandler` in [`.../browser/knossos_resource_handler.cc`][knossos_resource_handler.cc].

This class does some safety checks and then passes the request body and URL to
`KnossosHandleRequest` which is declared in
[`packages/libknossos/api/cef_bridge.go`][cef_bridge.go]. This function does
some sanity checks, builds a HTTP request from the passed data and hands it over
to the Twirp request handler. This handler maps the URL to a given RPC method
and proceeds to call it or abort with an error.

The end result is captured by `KnossosHandleRequest`, written to a new
`KnossosResponse` object and returned to `KnossosResourceHandler` which then
passes it back to CEF. Finally, the response arrives back in the web page and
is processed by the Twirp client there.

[state.tsx]: ../packages/client-ui/src/lib/state.tsx
[knossos_handler.cc]: ../packages/client/launcher/browser/knossos_handler.cc
[knossos_resource_handler.cc]: ../packages/client/launcher/browser/knossos_resource_handler.cc
[cef_bridge.go]: ../packages/libknossos/api/cef_bridge.go
