const { execFileSync } = require('node:child_process')
const assert = require('node:assert/strict')
const { chromium } = require(process.env.NODE_PATH + '/playwright')

async function main() {
  assert.match(execFileSync('ffprobe', ['-version'], { encoding: 'utf8' }), /ffprobe version/)
  assert.match(execFileSync('convert', ['-version'], { encoding: 'utf8' }), /ImageMagick/)
  const browser = await chromium.launch({ headless: true })
  try {
    const page = await browser.newPage()
    await page.setContent('<main><h1>Media runtime verified</h1></main>')
    assert.equal(await page.locator('h1').innerText(), 'Media runtime verified')
    assert.ok((await page.screenshot()).length > 0)
    console.log('Chromium rendering, FFprobe, and ImageMagick verified')
  } finally {
    await browser.close()
  }
}

main().catch(error => { console.error(error); process.exitCode = 1 })
