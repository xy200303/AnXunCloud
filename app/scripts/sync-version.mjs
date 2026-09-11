import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const appDir = path.resolve(scriptDir, '..')
const packagePath = path.join(appDir, 'package.json')
const manifestPath = path.join(appDir, 'manifest.json')
const packageInfo = JSON.parse(fs.readFileSync(packagePath, 'utf8'))
const version = String(packageInfo.version || '').trim()
if (!/^\d+\.\d+\.\d+(?:[-+].*)?$/.test(version)) {
  throw new Error(`Invalid app version: ${version}`)
}

const coreVersion = version.split(/[+-]/, 1)[0]
const parts = coreVersion.split('.').map((part) => Number(part))
const versionCode = parts[0] * 100 + parts[1] * 10 + parts[2]
let manifest = fs.readFileSync(manifestPath, 'utf8')
manifest = manifest.replace(/("versionName"\s*:\s*")[^"]*(")/, `$1${version}$2`)
manifest = manifest.replace(/("versionCode"\s*:\s*)"?(\d+)"?/, `$1"${versionCode}"`)
fs.writeFileSync(manifestPath, manifest)
console.log(`Synchronized manifest version: ${version} (${versionCode})`)
