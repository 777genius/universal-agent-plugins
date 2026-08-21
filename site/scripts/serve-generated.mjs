import { createReadStream, existsSync, statSync } from 'node:fs'
import { createServer } from 'node:http'
import { extname, resolve, sep } from 'node:path'

const root = resolve(process.cwd(), '.output/public')
const base = '/universal-agent-plugins/'
const mime = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.ico': 'image/x-icon',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.txt': 'text/plain; charset=utf-8',
  '.xml': 'application/xml; charset=utf-8',
}

if (!existsSync(resolve(root, 'index.html'))) {
  throw new Error(`generated Pages output is missing at ${root}`)
}

createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://127.0.0.1')
  if (!url.pathname.startsWith(base)) {
    response.writeHead(url.pathname === '/' ? 302 : 404, url.pathname === '/' ? { location: base } : undefined)
    response.end()
    return
  }

  let relative
  try {
    relative = decodeURIComponent(url.pathname.slice(base.length))
  } catch {
    response.writeHead(400).end()
    return
  }
  let file = resolve(root, relative)
  if (file !== root && !file.startsWith(`${root}${sep}`)) {
    response.writeHead(403).end()
    return
  }
  if (existsSync(file) && statSync(file).isDirectory()) file = resolve(file, 'index.html')
  if (!existsSync(file) && existsSync(`${file}.html`)) file = `${file}.html`
  if (!existsSync(file) || !statSync(file).isFile()) {
    response.writeHead(404).end()
    return
  }

  response.writeHead(200, {
    'cache-control': 'no-store',
    'content-type': mime[extname(file)] ?? 'application/octet-stream',
  })
  if (request.method === 'HEAD') response.end()
  else createReadStream(file).pipe(response)
}).listen(4173, '127.0.0.1')
