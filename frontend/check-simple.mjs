import { chromium } from '@playwright/test'

const browser = await chromium.launch()
for (const path of ['/', '/mashup', '/karaoke', '/rooms']) {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } })
  await page.goto(`http://127.0.0.1:3000${path}`, { waitUntil: 'load' })
  await page.waitForTimeout(2000)
  await page.mouse.move(640, 500)
  await page.mouse.wheel(0, 600)
  await page.waitForTimeout(400)
  const r = await page.evaluate(() => ({
    docY: Math.round(window.scrollY),
    docOverY: document.documentElement.scrollHeight - document.documentElement.clientHeight,
    docOverX: document.documentElement.scrollWidth - document.documentElement.clientWidth,
    appY: Math.round(document.querySelector('.app')?.scrollTop ?? -1),
    navTop: Math.round(document.querySelector('nav')?.getBoundingClientRect().top ?? -999),
  }))
  console.log(path, JSON.stringify(r))
  await page.close()
}
{
  const page = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true })
  await page.goto('http://127.0.0.1:3000/', { waitUntil: 'load' })
  await page.waitForTimeout(2000)
  const r = await page.evaluate(() => ({
    docOverY: document.documentElement.scrollHeight - document.documentElement.clientHeight,
    docOverX: document.documentElement.scrollWidth - document.documentElement.clientWidth,
  }))
  console.log('mobile /:', JSON.stringify(r))
  await page.close()
}
await browser.close()