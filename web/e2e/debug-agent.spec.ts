import { test, expect } from '@playwright/test'

test('debug agent new page', async ({ page }) => {
  page.on('console', msg => console.log('CONSOLE:', msg.type(), msg.text()))
  page.on('pageerror', err => console.log('PAGE ERROR:', err.message))

  await page.goto('/admin/agents/new')
  await page.waitForTimeout(5000)
  
  const url = page.url()
  console.log('URL after 5s:', url)
  
  const bodyHTML = await page.evaluate(() => document.body.innerHTML.slice(0, 1000))
  console.log('Body HTML:', bodyHTML)
  
  const title = await page.title()
  console.log('Title:', title)
})
