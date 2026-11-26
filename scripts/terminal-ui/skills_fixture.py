"""Disposable sources and native protocol responders for public Skills proof.

The local publisher uses verified HTTPS with Linux SSL_CERT_FILE. Other systems
use the separately pinned public GitHub source. No transport/policy is replaced.
Native responders qualify installer behavior, not vendor acceptance or models.
"""
from contextlib import suppress
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import socket
import ssl
import subprocess
import sys
import threading
from types import MappingProxyType

from harness import Fixture, check

SOURCE_SHA = 'ce789138a393bffd7e6d001c0e4f8aec05e47773'
GITHUB_REPOSITORY = '777genius/universal-agent-plugins'
GITHUB_SOURCE = GITHUB_REPOSITORY + '@' + SOURCE_SHA
GITHUB_PATH = 'tests/fixtures/uap_apm/skills/benchmark-note'
GITHUB_SKILL_BLOB = '1341ca3c10621a9e5e25304cca19144c6b54fa3b'
SKILL_BODY = ('---\nname: console-proof\ndescription: Read an inert fixture note\n---\n'
              'Read reference.txt. Do not execute commands or contact services.\n')


class _PublisherServer(ThreadingHTTPServer):
    """Accept raw TCP; a stalled TLS peer cannot block other request threads."""
    daemon_threads = False
    handshake_timeout = 1.0

    def __init__(self, address, handler, context):
        self.context = context
        self.connections = set()
        self.connection_lock = threading.Lock()
        self.closing = False
        super().__init__(address, handler)

    def process_request_thread(self, request, client_address):
        connection = request
        try:
            with self.connection_lock:
                if self.closing:
                    return
                self.connections.add(request)
            request.settimeout(self.handshake_timeout)
            connection = self.context.wrap_socket(
                request, server_side=True, do_handshake_on_connect=False)
            with self.connection_lock:
                self.connections.discard(request)
                if self.closing:
                    return
                self.connections.add(connection)
            connection.do_handshake()
            connection.settimeout(5)
            self.finish_request(connection, client_address)
        except (OSError, ssl.SSLError):
            pass  # Untrusted, stalled and canceled peers are not successful requests.
        except Exception:
            self.handle_error(connection, client_address)
        finally:
            with self.connection_lock:
                self.connections.discard(request)
                self.connections.discard(connection)
            self.shutdown_request(connection)

    def server_close(self):
        with self.connection_lock:
            self.closing = True
            connections = tuple(self.connections)
        for connection in connections:
            with suppress(OSError):
                connection.shutdown(socket.SHUT_RDWR)
            with suppress(OSError):
                connection.close()
        # Non-daemon request threads are joined only after their sockets close.
        super().server_close()


class Publisher:
    """Complete publisher, with original byte evidence and explicit failures."""
    def __init__(self, root):
        check(sys.platform == 'linux', 'local publisher trust requires Linux')
        self.root = Path(root)
        self.root.mkdir()
        self.requests = []
        self.release = threading.Event()
        self.release.set()
        self.status = 200
        self.files = {'SKILL.md': SKILL_BODY.encode(),
                      'reference.txt': b'skills-console-proof-v1\n'}
        key, cert = self.root / 'key.pem', self.root / 'ca.pem'
        openssl = shutil.which('openssl')
        check(openssl is not None, 'openssl is required for the Linux TLS fixture')
        result = subprocess.run(
            [openssl, 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
             '-keyout', str(key), '-out', str(cert), '-days', '2',
             '-subj', '/CN=127.0.0.1', '-addext', 'subjectAltName=IP:127.0.0.1'],
            capture_output=True, timeout=20)
        check(result.returncode == 0, 'fixture certificate creation failed')
        key.chmod(0o600)
        publisher = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                publisher.requests.append(self.path)
                if not publisher.release.wait(15):
                    self.send_error(503)
                    return
                if publisher.status != 200:
                    self.send_error(publisher.status)
                    return
                prefix = '/publisher/.well-known/agent-skills/'
                if self.path == prefix + 'index.json':
                    body = json.dumps({'skills': [{
                        'name': 'console-proof', 'description': 'Inert terminal fixture',
                        'files': list(publisher.files)}]}).encode()
                elif self.path.startswith(prefix + 'console-proof/'):
                    body = publisher.files.get(self.path.removeprefix(prefix + 'console-proof/'))
                    if body is None:
                        self.send_error(404)
                        return
                else:
                    self.send_error(404)
                    return
                self.send_response(200)
                self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                try:
                    self.wfile.write(body)
                except (BrokenPipeError, ConnectionResetError, ssl.SSLError):
                    pass  # Cancellation is recorded, never counted as a pass.

            def log_message(self, *_args):
                pass

        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(cert, key)
        self.server = _PublisherServer(('127.0.0.1', 0), Handler, context)
        self.ca = cert
        self.url = f'https://127.0.0.1:{self.server.server_port}/publisher'
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def close(self):
        self.release.set()
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)
        check(not self.thread.is_alive(), 'publisher did not shut down')


def public_git_environment(fixture):
    """Expose Git without parent credential, agent or global config inheritance."""
    git = shutil.which('git')
    check(git is not None, 'public GitHub Skills cases require real Git')
    fixture.env['PATH'] += os.pathsep + str(Path(git).resolve().parent)
    for key in ('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'NO_PROXY'):
        fixture.env.pop(key, None)
    # Windows Git locates its own DLL/libexec layout from its installed launcher.
    # The parent PATH is not inherited; HOME remains private and Git is further
    # isolated by production acquisition.
    return git


class SkillsFixture(Fixture):
    def __init__(self, root, native_binary=None, scanner_binary=None, source='publisher'):
        super().__init__(root, scanner_binary)
        self.publisher = None
        self.source_kind = source
        if source == 'publisher':
            self.publisher = Publisher(self.root / 'publisher')
            self.env['SSL_CERT_FILE'] = str(self.publisher.ca)
            self.env['NO_PROXY'] = '127.0.0.1'
            self.source = self.publisher.url
            self.source_flags = ['--skill', 'console-proof']
            self.skill_name, self.skill_bytes = 'console-proof', SKILL_BODY.encode()
            self.skill_blob = None
            self.expected_source = MappingProxyType(
                {'website': self.source, 'kind': 'skill', 'skill': self.skill_name})
        elif source == 'github':
            public_git_environment(self)
            self.source = GITHUB_SOURCE
            self.source_flags = ['--path', GITHUB_PATH]
            self.skill_name, self.skill_bytes = 'benchmark-note', None
            self.skill_blob = GITHUB_SKILL_BLOB
            self.expected_source = MappingProxyType({
                'git': GITHUB_REPOSITORY, 'commit': SOURCE_SHA, 'path': GITHUB_PATH,
                'kind': 'skill', 'skill': self.skill_name})
        else:
            raise AssertionError('unknown source fixture')
        if native_binary:
            suffix = '.exe' if os.name == 'nt' else ''
            shutil.copy2(native_binary, self.bin / ('codex' + suffix))
            (self.bin / ('codex' + suffix)).chmod(0o700)
            if os.name == 'nt':
                self.env['PATHEXT'] = '.EXE'
                shutil.copy2(native_binary, self.bin / 'cursor.exe')
        self.before = self.mutations()

    def args(self, binary, targets='codex', yes=True, json_output=True):
        args = [str(binary), 'skills', 'add', self.source, *self.source_flags,
                '--target', targets, '--no-ui']
        if yes:
            args.append('--yes')
        if json_output:
            args += ['--format', 'json']
        return args

    def control(self, name, value='1'):
        (self.root / name).write_text(value, encoding='utf-8')

    def calls(self):
        path = self.root / 'native-calls.jsonl'
        if not path.exists():
            return []
        with path.open(encoding='utf-8', newline='\n') as stream:
            return [json.loads(line) for line in stream]

    def close(self):
        if self.publisher:
            self.publisher.close()


def build_native_fixture(go, output):
    """Compile once per lane; copy the responder into private client paths."""
    output = Path(output)
    source = output.with_suffix('.go')
    source.write_text(NATIVE_SOURCE, encoding='utf-8')
    try:
        result = subprocess.run(
            [str(go), 'build', '-p=1', '-o', str(output), str(source)],
            capture_output=True, timeout=120,
            env=dict(os.environ, GOTOOLCHAIN='local', GOWORK='off'))
        check(result.returncode == 0,
              'native fixture build failed: ' + result.stderr.decode()[-4000:])
    finally:
        source.unlink(missing_ok=True)
    return output


NATIVE_SOURCE = r'''package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"
)

type object = map[string]any
var root string

func read(path string) object {
    b, err := os.ReadFile(path)
    if os.IsNotExist(err) { return object{} }
    if err != nil { panic(err) }
    value := object{}
    if err := json.Unmarshal(b, &value); err != nil { panic(err) }
    return value
}

func write(path string, value any) {
    body, err := json.Marshal(value)
    if err != nil { panic(err) }
    if err := os.WriteFile(path, body, 0600); err != nil { panic(err) }
}

func fail(name string) bool {
    path := filepath.Join(root, name)
    body, err := os.ReadFile(path)
    if os.IsNotExist(err) { return false }
    if err != nil { panic(err) }
    if string(body) != "always" {
        if err := os.Remove(path); err != nil { panic(err) }
    }
    fmt.Fprintln(os.Stderr, "synthetic native protocol refused operation")
    return true
}

func output(value any) {
    if err := json.NewEncoder(os.Stdout).Encode(value); err != nil { os.Exit(96) }
}

func clone(source, destination string) {
    err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
        if err != nil { return err }
        if entry.Type() & os.ModeSymlink != 0 { return fmt.Errorf("fixture refuses symlink") }
        relative, err := filepath.Rel(source, path)
        if err != nil { return err }
        target := filepath.Join(destination, relative)
        if entry.IsDir() { return os.MkdirAll(target, 0700) }
        body, err := os.ReadFile(path)
        if err != nil { return err }
        return os.WriteFile(target, body, 0600)
    })
    if err != nil { panic(err) }
}

func main() {
    executable, err := os.Executable()
    if err != nil { panic(err) }
    root = filepath.Dir(filepath.Dir(executable))
    args := os.Args[1:]
    base := strings.ToLower(filepath.Base(executable))
    // Capability URLs are deliberately neither stored nor logged.
    if base == "xdg-open" || base == "open" || base == "rundll32.exe" {
        if len(args) == 0 { os.Exit(97) }
        write(filepath.Join(root, "browser-opened"), object{"opened":true})
        return
    }
    log, err := os.OpenFile(filepath.Join(root, "native-calls.jsonl"),
        os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil { panic(err) }
    if err := json.NewEncoder(log).Encode(args); err != nil { panic(err) }
    if err := log.Close(); err != nil { panic(err) }
    if len(args) == 1 && args[0] == "--version" {
        fmt.Println("codex-cli 0.153.4")
        return
    }
    if len(args) > 0 && args[0] == "app-server" { rpc(); return }
    statePath := filepath.Join(root, "native-registry.json")
    state := read(statePath)
    if len(args) == 3 && strings.Join(args, " ") == "plugin list --json" {
        if fail("fail-list") { os.Exit(7) }
        items, found := state["installed"]
        if !found { items = []any{} }
        output(object{"installed":items})
        return
    }
    if len(args) == 5 && strings.Join(args[:3], " ") == "plugin marketplace add" {
        path := args[3]
        relative, err := filepath.Rel(filepath.Join(root,"installer","managed","clients","codex"),path)
        if err != nil || relative == ".." || strings.HasPrefix(relative,".."+string(os.PathSeparator)) {
            panic("fixture marketplace escaped disposable managed root")
        }
        manifest := read(filepath.Join(path, ".agents", "plugins", "marketplace.json"))
        state["marketplace"], state["path"] = manifest["name"], path
        write(statePath, state)
        output(object{})
        return
    }
    if len(args) == 4 && strings.Join(args[:2], " ") == "plugin add" {
        if fail("fail-add") { os.Exit(7) }
        parts := strings.Split(args[2], "@")
        if len(parts) != 2 || state["marketplace"] != parts[1] { os.Exit(97) }
        path, _ := state["path"].(string)
        manifest := read(filepath.Join(path,"plugin.json"))
        version, _ := manifest["version"].(string)
        if version == "" { version = "0.0.0" }
        cache := filepath.Join(os.Getenv("CODEX_HOME"),"plugins","cache",parts[1],parts[0],version)
        clone(path, cache)
        state["installed"] = []any{object{
            "pluginId":args[2], "name":parts[0], "marketplaceName":parts[1],
            "installed":true, "enabled":true, "version":version,
            "source":object{"source":"local","path":path}}}
        state["cache"], state["name"], state["spec"] = cache, parts[0], args[2]
        write(statePath,state)
        pauseAfterNativeAdd()
        output(object{})
        return
    }
    if len(args) >= 3 && strings.Join(args[:2]," ") == "plugin remove" {
        state["installed"] = []any{}
        write(statePath,state)
        output(object{})
        return
    }
    if len(args) >= 4 && strings.Join(args[:3]," ") == "plugin marketplace remove" {
        delete(state,"marketplace")
        write(statePath,state)
        output(object{})
        return
    }
    if len(args) >= 4 && strings.Join(args[:3]," ") == "plugin marketplace update" {
        output(object{})
        return
    }
    fmt.Fprintln(os.Stderr,"unsupported synthetic protocol",args)
    os.Exit(97)
}

func pauseAfterNativeAdd() {
    control := filepath.Join(root,"pause-add")
    nonce, err := os.ReadFile(control)
    if os.IsNotExist(err) { return }
    if err != nil { panic(err) }
    executable, err := os.Executable()
    if err != nil { panic(err) }
    marker := filepath.Join(root,"native-add-paused")
    write(marker+".pending", object{
        "pid":os.Getpid(),"ppid":os.Getppid(),"executable":executable,"nonce":string(nonce)})
    if err := os.Rename(marker+".pending",marker); err != nil { panic(err) }
    deadline := time.Now().Add(30*time.Second)
    for time.Now().Before(deadline) {
        if _, err := os.Stat(control); os.IsNotExist(err) { return }
        time.Sleep(20*time.Millisecond)
    }
    panic("fixture pause deadline expired")
}

func rpc() {
    scanner := bufio.NewScanner(io.LimitReader(os.Stdin,1<<20))
    scanner.Buffer(make([]byte,4096),1<<20)
    for scanner.Scan() {
        value := object{}
        if json.Unmarshal(scanner.Bytes(),&value) != nil { os.Exit(97) }
        id, present := value["id"]
        method, _ := value["method"].(string)
        if !present { continue }
        state := read(filepath.Join(root,"native-registry.json"))
        switch method {
        case "initialize":
            output(object{"id":id,"result":object{
                "userAgent":"codex/0.153.4","codexHome":os.Getenv("CODEX_HOME")}})
        case "skills/list":
            path,_ := state["path"].(string)
            cache,_ := state["cache"].(string)
            name,_ := state["name"].(string)
            skills := []any{}
            err := filepath.WalkDir(filepath.Join(cache,"skills"),
                func(path string, entry os.DirEntry, err error) error {
                    if err != nil { return err }
                    if !entry.IsDir() && entry.Name() == "SKILL.md" {
                        skills = append(skills,object{
                            "name":name+":"+filepath.Base(filepath.Dir(path)),
                            "path":path,"scope":"user","enabled":true,"pluginId":state["spec"]})
                    }
                    return nil
                })
            if err != nil { panic(err) }
            output(object{"id":id,"result":object{"data":[]any{
                object{"cwd":path,"errors":[]any{},"skills":skills}}}})
        default:
            output(object{"id":id,"error":object{"code":-32601,
                "message":"unsupported synthetic method"}})
        }
    }
    if err := scanner.Err(); err != nil { panic(err) }
}
'''
