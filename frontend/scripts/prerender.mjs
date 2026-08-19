import { readFile, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const scriptDirectory = dirname(fileURLToPath(import.meta.url))
const frontendDirectory = resolve(scriptDirectory, '..')
const templatePath = resolve(frontendDirectory, 'dist', 'index.html')
const serverEntryPath = resolve(frontendDirectory, 'dist-ssr', 'entry-server.js')

const [{ render }, template] = await Promise.all([
  import(pathToFileURL(serverEntryPath).href),
  readFile(templatePath, 'utf8'),
])

const root = '<div id="root"></div>'
if (!template.includes(root)) {
  throw new Error('Nu am găsit elementul root în șablonul HTML.')
}

const prerendered = template.replace(root, `<div id="root">${render()}</div>`)
await writeFile(templatePath, prerendered)
