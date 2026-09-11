"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const http = require("node:http");
const { EventEmitter } = require("node:events");
const { PassThrough, Duplex } = require("node:stream");
const test = require("node:test");
const v = require("../lib/verifier");
const c = require("../scripts/dual-authoring-candidate");
const BODY = Buffer.from("bounded injected stream\n");
const PIN = c.metadata(BODY);
const URL = "https://github.com/owner/repo/releases/download/exact/binary";

function transport(makeResponse, timeout = false) {
  return (url, options) => {
    assert.deepEqual(Object.keys(options.headers).sort(), ["Accept", "User-Agent"]);
    const request = new EventEmitter();
    request.destroy = error => request.emit("error", error);
    request.setTimeout = (ms, callback) => { assert.equal(ms, 30_000); if (timeout) process.nextTick(callback); };
    if (!timeout) process.nextTick(() => {
      const response = new PassThrough(); response.statusCode = 200; response.headers = {};
      makeResponse(response, request, url);
    });
    return request;
  };
}
function destination() {
  return path.join(fs.mkdtempSync(path.join(os.tmpdir(), "verifier-stream-")), "download");
}

test("canonical downloader closes before verified success with or without Content-Length", async () => {
  for (const declared of [false, true]) {
    const file = destination();
    await v.downloadFile(URL, file, PIN, { request: transport((res, req) => {
      if (declared) res.headers["content-length"] = String(BODY.length);
      req.emit("response", res); res.end(BODY);
    }) });
    assert.deepEqual(fs.readFileSync(file), BODY); assert.equal(fs.statSync(file).mode & 0o777, 0o600);
    fs.unlinkSync(file); assert.equal(fs.existsSync(file), false);
  }
});

for (const delta of [0, -1, 1]) {
  test(`real HTTP parser preserves padded Content-Length with size delta ${delta}`, async () => {
    const length = "000" + (BODY.length + delta);
    const file = destination();
    let parsed;
    const request = (url, options) => {
      assert.equal(url.protocol, "https:");
      assert.equal(url.hostname, "github.com");
      assert.deepEqual(Object.keys(options.headers).sort(), ["Accept", "User-Agent"]);
      let sent = false;
      const socket = new Duplex({
        read() {},
        write(chunk, encoding, callback) {
          callback();
          if (sent) return;
          sent = true;
          process.nextTick(() => {
            this.push(Buffer.concat([Buffer.from(
              `HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Length: ${length}\r\n\r\n`
            ), BODY]));
            this.push(null);
          });
        }
      });
      socket.setTimeout = () => socket;
      const req = http.get({ hostname: "fixture.invalid", headers: options.headers,
        createConnection: () => socket });
      req.prependOnceListener("response", res => { parsed = res.headers["content-length"]; });
      return req;
    };
    const download = v.downloadFile(URL, file, PIN, { request });
    if (delta === 0) {
      await download;
      assert.deepEqual(fs.readFileSync(file), BODY);
      assert.equal(fs.statSync(file).mode & 0o777, 0o600);
      fs.unlinkSync(file);
    } else {
      await assert.rejects(download, /size does not match embedded metadata/);
      assert.equal(fs.existsSync(file), false);
    }
    assert.equal(parsed, length);
  });
}

test("declared length rejects zero, malformed, ambiguous and inexact values", async () => {
  for (const length of ["0", "000", "NaN", "abc", "-1", "+23", "1.5", "01", "9999999999999999999999", "3", "", " 22", "22,22"]) {
    const file = destination();
    await assert.rejects(v.downloadFile(URL, file, PIN, { request: transport((res, req) => {
      res.headers["content-length"] = length; req.emit("response", res); res.end(BODY);
    }) }), /size/);
    assert.equal(fs.existsSync(file), false);
  }
});

test("declared zero and unsafe sizes reject even when embedded size matches numerically", async () => {
  for (const length of ["000", "09007199254740992"]) {
    const file = destination();
    await assert.rejects(v.downloadFile(URL, file, { ...PIN, size: Number(length) }, {
      request: transport((res, req) => {
        res.headers["content-length"] = length; req.emit("response", res); res.end(BODY);
      })
    }), /size does not match embedded metadata/);
    assert.equal(fs.existsSync(file), false);
  }
});

test("stream overflow, truncation, hash failure, response and request errors settle after output closes", async () => {
  for (const scenario of ["overflow", "truncated", "hash", "response error", "request error", "premature close"]) {
    const file = destination(); let closed = false;
    const create = fs.createWriteStream;
    fs.createWriteStream = (...args) => { const stream = create(...args); stream.on("close", () => { closed = true; }); return stream; };
    try {
      await assert.rejects(v.downloadFile(URL, file, PIN, { request: transport((res, req) => {
        req.emit("response", res);
        if (scenario === "overflow") res.end(Buffer.concat([BODY, BODY]));
        if (scenario === "truncated") res.end(BODY.subarray(1));
        if (scenario === "hash") res.end(Buffer.alloc(BODY.length));
        if (scenario === "response error") res.destroy(new Error("injected response error"));
        if (scenario === "request error") req.emit("error", new Error("injected request error"));
        if (scenario === "premature close") res.destroy();
      }) }));
      assert.equal(closed, true, scenario);
      if (fs.existsSync(file)) fs.unlinkSync(file);
    } finally { fs.createWriteStream = create; }
  }
});

test("timeout, HTTP status, redirects, URL credentials and exclusive output are bounded", async () => {
  await assert.rejects(v.downloadFile(URL, destination(), PIN, { request: transport(() => {}, true) }), /timed out/);
  for (const status of [403, 404, 500]) await assert.rejects(v.downloadFile(URL, destination(), PIN, {
    request: transport((res, req) => { res.statusCode = status; req.emit("response", res); res.end(); })
  }), /HTTP/);
  let requests = 0;
  await assert.rejects(v.downloadFile(URL, destination(), PIN, { request: transport((res, req) => {
    requests++; res.statusCode = 302; res.headers.location = URL; req.emit("response", res); res.end();
  }) }), /too many/); assert.equal(requests, 6);
  for (const location of ["https://[", "http://github.com/path", "https://user:pass@github.com/path", "https://evil.invalid/path"]) {
    await assert.rejects(v.downloadFile(URL, destination(), PIN, { request: transport((res, req) => {
      res.statusCode = 302; res.headers.location = location; req.emit("response", res); res.end();
    }) }));
  }
  const file = destination(); fs.writeFileSync(file, "unowned sentinel");
  await assert.rejects(v.downloadFile(URL, file, PIN, { request: transport((res, req) => {
    req.emit("response", res); res.end(BODY);
  }) }), /EEXIST/); assert.equal(fs.readFileSync(file, "utf8"), "unowned sentinel");
});

test("hash and original facade cache helpers remain canonical", async () => {
  const file = destination(); fs.writeFileSync(file, BODY);
  assert.equal(await v.sha256File(file), PIN.sha256);
  assert.equal(await v.validCachedBinary(file, PIN.sha256), true);
  fs.unlinkSync(file); assert.equal(await v.validCachedBinary(file, PIN.sha256), false);
  assert.equal(typeof fsp.open, "function");
});

test("download owner receives opened inode on partial failure, and cancellation closes output", async () => {
  const file = destination(), controller = new AbortController();
  let opened;
  await assert.rejects(v.downloadFile(URL, file, PIN, {
    signal: controller.signal,
    onOpen: stat => { opened = stat; controller.abort(); },
    request: transport((res, req) => { req.emit("response", res); res.write(BODY.subarray(0, 2)); })
  }), /cancelled/);
  assert.ok(opened);
  const named = fs.lstatSync(file);
  assert.equal(named.ino, opened.ino); assert.equal(named.dev, opened.dev);
  fs.unlinkSync(file); assert.equal(fs.existsSync(file), false);
});

test("download open observer failure settles after close without hiding its owned inode", async () => {
  const file = destination(); let opened;
  await assert.rejects(v.downloadFile(URL, file, PIN, {
    onOpen: stat => { opened = stat; throw new Error("injected owner observation failure"); },
    request: transport((res, req) => { req.emit("response", res); res.end(BODY); })
  }), /owner observation failure/);
  assert.equal(fs.lstatSync(file).ino, opened.ino);
  fs.unlinkSync(file);
});

for (const scenario of [
  { redirects: 1, cancel: true },
  { redirects: 3, cancel: true },
  { redirects: 3, cancel: true, oldError: true },
  { redirects: 3, cancel: false, oldError: true }
]) {
  test(`redirect chain closes before settlement ${JSON.stringify(scenario)}`, async t => {
    const file = destination(), controller = new AbortController();
    const requests = [], responses = [], sockets = [], events = [];
    let output, opened;
    const create = fs.createWriteStream;
    t.mock.method(fs, "createWriteStream", (...args) => {
      output = create(...args);
      output.once("close", () => events.push("close"));
      return output;
    });
    const request = (url, options) => {
      assert.deepEqual(Object.keys(options.headers).sort(), ["Accept", "User-Agent"]);
      assert.equal(url.hostname, requests.length ? "release-assets.githubusercontent.com" : "github.com");
      const number = requests.length;
      let sent = false;
      const socket = new Duplex({
        read() {},
        write(chunk, encoding, callback) {
          callback();
          if (sent) return;
          sent = true;
          process.nextTick(() => this.push(Buffer.from(number < scenario.redirects
            ? "HTTP/1.1 302 Found\r\nLocation: https://release-assets.githubusercontent.com/binary\r\nContent-Length: 100\r\nConnection: close\r\n\r\nx"
            : `HTTP/1.1 200 OK\r\nContent-Length: ${BODY.length}\r\nConnection: close\r\n\r\n${BODY.subarray(0, 1)}`)));
        }
      });
      socket.setTimeout = () => socket;
      sockets.push(socket);
      const req = http.get({ hostname: "fixture.invalid", headers: options.headers,
        createConnection: () => socket });
      req.once("response", res => responses.push(res));
      req.on("error", () => events.push(`request-error-${number}`));
      requests.push(req);
      return req;
    };
    try {
      const download = v.downloadFile(URL, file, PIN, {
        request, signal: controller.signal,
        onOpen: stat => {
          opened = stat;
          events.push("open");
          if (scenario.oldError) requests[0].destroy(new Error("superseded request failed"));
          // Let the real old ClientRequest error fire while its descendant is open.
          setImmediate(() => {
            if (scenario.cancel) controller.abort();
            else { sockets.at(-1).push(BODY.subarray(1)); sockets.at(-1).push(null); }
          });
        }
      }).then(() => {
        events.push("resolved");
        assert.equal(output.closed, true, "output must close before resolution");
      }, error => {
        events.push("rejected");
        assert.equal(output.closed, true, "output must close before rejection");
        throw error;
      });
      if (scenario.cancel) await assert.rejects(download, /cancelled/);
      else { await download; assert.deepEqual(fs.readFileSync(file), BODY); }
      assert.equal(requests.length, scenario.redirects + 1);
      assert.equal(responses.length, requests.length);
      for (const res of responses.slice(0, -1)) assert.equal(res.complete, false);
      if (scenario.oldError) assert.ok(events.includes("request-error-0"));
      assert.ok(events.indexOf("close") < events.indexOf(scenario.cancel ? "rejected" : "resolved"));
      const named = fs.lstatSync(file);
      assert.equal(named.ino, opened.ino); assert.equal(named.dev, opened.dev);
    } finally {
      controller.abort();
      for (const req of requests) req.destroy();
      if (output && !output.closed) await new Promise(resolve => output.once("close", resolve));
      if (fs.existsSync(file)) fs.unlinkSync(file);
    }
  });
}
