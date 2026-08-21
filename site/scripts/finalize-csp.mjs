import { resolve } from 'node:path'
import { finalizeCspDirectory } from './csp-html.mjs'

const output = resolve(process.cwd(), process.argv[2] ?? '.output/public')
const processed = await finalizeCspDirectory(output)
if (!processed.length) throw new Error(`no generated HTML files found under ${output}`)
console.log(`finalized CSP for ${processed.length} generated HTML files`)
