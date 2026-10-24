// This process is a transport/lifecycle fixture. It never installs a plugin.
package main

import (
 "encoding/json"
 "fmt"
 "io"
 "net"
 "net/http"
 "os"
 "os/signal"
 "strings"
 "syscall"
)

func main() {
 if len(os.Args)!=3 || os.Args[1]!="manage" || os.Args[2]!="serve" { os.Exit(2) }
 mode:=os.Getenv("UAP_SDK_TEST_MODE")
 if mode=="exit" { fmt.Fprintln(os.Stderr,"private stderr: "+strings.Repeat("a",64)); os.Exit(7) }
 if mode=="malformed" { fmt.Println("this is not JSON"); _,_=io.Copy(io.Discard,os.Stdin); return }
 if mode=="oversized" { fmt.Println(strings.Repeat("x",70000)); _,_=io.Copy(io.Discard,os.Stdin); return }
 var config map[string]any
 if err:=json.NewDecoder(os.Stdin).Decode(&config);err!=nil { os.Exit(3) }
 if mode=="timeout" { _,_=io.Copy(io.Discard,os.Stdin);return }
 if mode=="stubborn" { signal.Ignore(syscall.SIGTERM) }
 listener,err:=net.Listen("tcp4","127.0.0.1:0");if err!=nil { panic(err) }
 origin:="http://"+listener.Addr().String()
 token:=strings.Repeat("a",64)
 context:="captured-fixture-context"
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /api/v1/session",func(w http.ResponseWriter,r *http.Request){
  if r.Header.Get("Authorization")!="Bearer "+token || r.Header.Get("Origin")!=origin { w.WriteHeader(403);return }
  w.Header().Set("Content-Type","application/json")
  sessionContext:=context;if mode=="mismatch" { sessionContext="wrong-context" }
  _=json.NewEncoder(w).Encode(map[string]any{"apiVersion":1,"engineVersion":"fixture","contextId":sessionContext,"scope":"user","targets":[]any{},"capabilities":[]any{map[string]any{"action":"install","supported":true}}})
 })
 server:=http.Server{Handler:mux}
 go func(){_=server.Serve(listener)}()
 version:=1;if mode=="version"{version=2}
 bootstrapURL:=origin;if mode=="remote"{bootstrapURL="http://example.com:1234"}
 _=json.NewEncoder(os.Stdout).Encode(map[string]any{"apiVersion":version,"url":bootstrapURL,"origin":bootstrapURL,"token":token,"engineVersion":"fixture","contextId":context})
 _,_=io.Copy(io.Discard,os.Stdin)
 if mode=="stubborn" { select{} }
 _=server.Close()
}
