import { test, expect } from '@playwright/test'

test.describe('Settings Page', () => {
  test('should load and save settings', async ({ page }) => {
    await page.goto('http://localhost:3000/settings')
    await expect(page.getByText('Settings')).toBeVisible()
    await expect(page.getByText('Server Configuration')).toBeVisible()
    // Örnek: Host inputunu değiştir ve kaydet
    const hostInput = page.getByLabel('Host')
    await hostInput.fill('127.0.0.1')
    await page.getByRole('button', { name: /Save Changes/i }).click()
    // Başarıyla kaydedildiğine dair bir feedback beklenebilir (ör: toast)
    // await expect(page.getByText('Ayarlar kaydedildi')).toBeVisible()
  })
})
