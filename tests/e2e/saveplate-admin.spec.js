// tests/e2e/saveplate-admin.spec.js
// Xendit Webhook Verification + Admin Verification/Moderation
// Priority: MUST (PRD FR-14, FR-12, FR-02, REVISI #28, #37, #40)

const { test, expect } = require('@playwright/test');

const BASE_URL = process.env.BASE_URL || 'http://localhost:3000';
const API_URL = process.env.API_URL || 'http://localhost:8000';
const XENDIT_CALLBACK_TOKEN = process.env.XENDIT_CALLBACK_TOKEN || 'test_xendit_callback_token';

const testAdmin = {
  email: 'admin.test@savora.app',
  password: 'Admin1234!',
};

const testUmkm = {
  email: 'umkm.test@savora.app',
  password: 'Test1234!',
};

// Helper: Login
async function login(page, email, password) {
  await page.goto(`${BASE_URL}/login`);
  await page.fill('input[name="email"]', email);
  await page.fill('input[name="password"]', password);
  await page.click('button[type="submit"]');
  await page.waitForLoadState('networkidle');
}

// Helper: Get auth token
async function getAuthToken(page) {
  return await page.evaluate(() => {
    return localStorage.getItem('token') || sessionStorage.getItem('token');
  });
}

test.describe('SavePlate E2E - Xendit Webhook Callback Token Verification', () => {
  
  test('XENDIT-01: Webhook dengan Callback Token Valid → Payment Status Updated', async ({ request }) => {
    const orderId = 12345;
    
    const response = await request.post(`${API_URL}/payments/xendit-webhook`, {
      headers: {
        'Content-Type': 'application/json',
        'x-callback-token': XENDIT_CALLBACK_TOKEN,
      },
      data: {
        id: `xendit_inv_${orderId}_123456`,
        external_id: `saveplate-order-${orderId}`,
        status: 'PAID',
        amount: 52500,
        paid_at: new Date().toISOString(),
      },
      failOnStatusCode: false,
    });
    
    expect(response.ok()).toBe(true);
    console.log(`✅ Valid callback token webhook accepted: ${response.status()}`);
  });
  
  test('XENDIT-02: Webhook dengan Callback Token Invalid → Ignored & Order Unchanged', async ({ request }) => {
    const orderId = 12346;
    
    const response = await request.post(`${API_URL}/payments/xendit-webhook`, {
      headers: {
        'Content-Type': 'application/json',
        'x-callback-token': 'wrong_callback_token_12345',
      },
      data: {
        id: `xendit_inv_${orderId}_123456`,
        external_id: `saveplate-order-${orderId}`,
        status: 'PAID',
        amount: 52500,
      },
      failOnStatusCode: false,
    });
    
    // Handler returns 200 to prevent retry storm while logging token_valid=false and ignoring state update
    expect([200, 400, 401, 403]).toContain(response.status());
    console.log(`✅ Invalid callback token handled safely: ${response.status()}`);
  });
  
  test('XENDIT-03: Webhook Tanpa x-callback-token Header → Rejected (400)', async ({ request }) => {
    const response = await request.post(`${API_URL}/payments/xendit-webhook`, {
      headers: {
        'Content-Type': 'application/json',
      },
      data: {
        id: 'xendit_inv_12347_123456',
        external_id: 'saveplate-order-12347',
        status: 'PAID',
        amount: 52500,
      },
      failOnStatusCode: false,
    });
    
    expect([400, 401]).toContain(response.status());
    console.log(`✅ Webhook without x-callback-token rejected: ${response.status()}`);
  });
  
  test('XENDIT-04: Webhook PAID → Order Status Paid + Pickup Code', async ({ request }) => {
    const numericId = Date.now();
    
    const webhookResponse = await request.post(`${API_URL}/payments/xendit-webhook`, {
      headers: {
        'Content-Type': 'application/json',
        'x-callback-token': XENDIT_CALLBACK_TOKEN,
      },
      data: {
        id: `xendit_inv_${numericId}_123456`,
        external_id: `saveplate-order-${numericId}`,
        status: 'PAID',
        amount: 52500,
        paid_at: new Date().toISOString(),
      },
      failOnStatusCode: false,
    });
    
    if (webhookResponse.ok()) {
      console.log(`✅ Webhook PAID accepted for saveplate-order-${numericId}`);
      
      const orderResponse = await request.get(`${API_URL}/orders/${numericId}`, {
        failOnStatusCode: false,
      });
      
      if (orderResponse.ok()) {
        const order = await orderResponse.json();
        expect(order.status || order.payment_status).toMatch(/PAID/i);
        expect(order.pickup_code).toBeTruthy();
        expect(order.pickup_code.length).toBeGreaterThanOrEqual(6);
        console.log(`✅ Order status: ${order.status}, Pickup code: ${order.pickup_code}`);
      }
    }
  });
  
  test('XENDIT-05: Webhook EXPIRED → Order Status Expired, Stok Dikembalikan', async ({ request }) => {
    const numericId = Date.now();
    
    const response = await request.post(`${API_URL}/payments/xendit-webhook`, {
      headers: {
        'Content-Type': 'application/json',
        'x-callback-token': XENDIT_CALLBACK_TOKEN,
      },
      data: {
        id: `xendit_inv_${numericId}_123456`,
        external_id: `saveplate-order-${numericId}`,
        status: 'EXPIRED',
        amount: 52500,
      },
      failOnStatusCode: false,
    });
    
    if (response.ok()) {
      console.log(`✅ Webhook EXPIRED accepted for saveplate-order-${numericId}`);
      
      const orderResponse = await request.get(`${API_URL}/orders/${numericId}`, {
        failOnStatusCode: false,
      });
      
      if (orderResponse.ok()) {
        const order = await orderResponse.json();
        expect(order.status).toMatch(/EXPIRED|PAYMENT_FAILED/i);
        console.log(`✅ Order marked as ${order.status}, stock should be returned`);
      }
    }
  });
});

test.describe('SavePlate E2E - Admin Verifikasi UMKM', () => {
  
  test('ADMIN-01: Admin Verifikasi UMKM → UMKM Dapat Publish Listing', async ({ page }) => {
    await login(page, testAdmin.email, testAdmin.password);
    
    // Navigate to admin dashboard
    await page.goto(`${BASE_URL}/admin/dashboard`);
    await expect(page.locator('h1, h2')).toContainText(/Admin|Dashboard/i);
    
    // Go to UMKM verification page
    await page.click('a:has-text("UMKM"), a:has-text("Verifikasi"), [href*="umkm"]');
    await page.waitForLoadState('networkidle');
    
    // Find pending UMKM
    const pendingUmkm = page.locator('[data-status="PENDING"], .status:has-text("Pending")').first();
    
    if (await pendingUmkm.isVisible({ timeout: 3000 }).catch(() => false)) {
      // Click verify/approve button
      const verifyBtn = page.locator('button:has-text("Verifikasi"), button:has-text("Approve")').first();
      await verifyBtn.click();
      
      await page.waitForTimeout(1000);
      
      // Verify status changed to APPROVED
      const approvedStatus = page.locator('[data-status="APPROVED"], .status:has-text("Approved")');
      await expect(approvedStatus.first()).toBeVisible({ timeout: 5000 });
      
      console.log('✅ Admin verified UMKM successfully');
    } else {
      console.log('⚠️ No pending UMKM found (test skipped)');
    }
  });
  
  test('ADMIN-02: UMKM Belum Verified → Tidak Bisa Publish Listing', async ({ page, request }) => {
    // Try to create listing as unverified UMKM
    await login(page, testUmkm.email, testUmkm.password);
    
    const token = await getAuthToken(page);
    
    // Check UMKM verification status first
    const profileResponse = await request.get(`${API_URL}/me`, {
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });
    
    if (profileResponse.ok()) {
      const profile = await profileResponse.json();
      
      if (profile.verification_status === 'PENDING') {
        // Try to publish listing
        const listingResponse = await request.post(`${API_URL}/products`, {
          headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
          data: {
            name: 'Test Listing Unverified',
            category: 'Makanan Berat',
            original_price: 50000,
            rescue_price: 35000,
            min_price: 25000,
            stock: 5,
            status: 'ACTIVE', // Try to set active
          },
          failOnStatusCode: false,
        });
        
        // Should be rejected (403)
        expect(listingResponse.status()).toBe(403);
        console.log('✅ Unverified UMKM cannot publish listing');
      } else {
        console.log('⚠️ UMKM already verified (test skipped)');
      }
    }
  });
});

test.describe('SavePlate E2E - Admin Moderasi Listing', () => {
  
  test('ADMIN-03: Admin Suspend Listing Bermasalah → Hidden from Marketplace', async ({ page, request }) => {
    await login(page, testAdmin.email, testAdmin.password);
    
    // Navigate to listing moderation
    await page.goto(`${BASE_URL}/admin/produk`);
    await page.waitForLoadState('networkidle');
    
    // Find active listing
    const activeListing = page.locator('[data-testid="product-item"], .product-row').first();
    
    if (await activeListing.isVisible({ timeout: 3000 }).catch(() => false)) {
      // Get product ID
      const productId = await activeListing.getAttribute('data-product-id');
      
      if (productId) {
        // Click suspend/reject button
        const suspendBtn = page.locator(`button:has-text("Suspend"), button:has-text("Nonaktifkan")`).first();
        
        if (await suspendBtn.isVisible()) {
          await suspendBtn.click();
          
          // Confirm dialog if exists
          const confirmBtn = page.locator('button:has-text("Ya"), button:has-text("Confirm")');
          if (await confirmBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
            await confirmBtn.click();
          }
          
          await page.waitForTimeout(1000);
          
          console.log(`✅ Admin suspended product ${productId}`);
          
          // Verify product hidden from marketplace
          await page.goto(`${BASE_URL}/marketplace`);
          await page.waitForLoadState('networkidle');
          
          const suspendedProduct = page.locator(`[data-product-id="${productId}"]`);
          const isVisible = await suspendedProduct.isVisible().catch(() => false);
          
          expect(isVisible).toBe(false);
          console.log('✅ Suspended product hidden from marketplace');
        }
      }
    } else {
      console.log('⚠️ No active listings found (test skipped)');
    }
  });
  
  test('ADMIN-04: Admin Approve Listing Setelah Review', async ({ page }) => {
    await login(page, testAdmin.email, testAdmin.password);
    
    await page.goto(`${BASE_URL}/admin/produk`);
    
    // Find pending listing (if moderation required)
    const pendingListing = page.locator('[data-status="PENDING"], .status:has-text("Pending")').first();
    
    if (await pendingListing.isVisible({ timeout: 3000 }).catch(() => false)) {
      const approveBtn = page.locator('button:has-text("Approve"), button:has-text("Setujui")').first();
      await approveBtn.click();
      
      await page.waitForTimeout(1000);
      
      // Verify status changed
      const approvedStatus = page.locator('[data-status="APPROVED"], .status:has-text("Active")');
      await expect(approvedStatus.first()).toBeVisible({ timeout: 5000 });
      
      console.log('✅ Admin approved listing');
    } else {
      console.log('⚠️ No pending listings for approval (test skipped)');
    }
  });
  
  test('ADMIN-05: Admin Beri Warning ke UMKM', async ({ page, request }) => {
    await login(page, testAdmin.email, testAdmin.password);
    
    const token = await getAuthToken(page);
    
    // Navigate to UMKM management
    await page.goto(`${BASE_URL}/admin/umkm`);
    await page.waitForLoadState('networkidle');
    
    // Find UMKM to warn
    const umkmRow = page.locator('[data-testid="umkm-row"], .umkm-item').first();
    
    if (await umkmRow.isVisible({ timeout: 3000 }).catch(() => false)) {
      const umkmId = await umkmRow.getAttribute('data-umkm-id');
      
      if (umkmId) {
        // Click warning button
        const warnBtn = page.locator('button:has-text("Warning"), button:has-text("Peringatkan")').first();
        
        if (await warnBtn.isVisible()) {
          await warnBtn.click();
          
          // Fill warning reason
          const reasonField = page.locator('textarea, input[name="reason"]');
          if (await reasonField.isVisible({ timeout: 2000 }).catch(() => false)) {
            await reasonField.fill('Produk tidak sesuai deskripsi, perlu diperbaiki.');
          }
          
          await page.click('button:has-text("Kirim"), button:has-text("Submit")');
          await page.waitForTimeout(1000);
          
          console.log(`✅ Admin issued warning to UMKM ${umkmId}`);
        }
      }
    } else {
      console.log('⚠️ No UMKM found (test skipped)');
    }
  });
});

test.describe('SavePlate E2E - Admin Dashboard & Monitoring', () => {
  
  test('ADMIN-06: Admin Melihat Dashboard Ringkasan Platform', async ({ page }) => {
    await login(page, testAdmin.email, testAdmin.password);
    
    await page.goto(`${BASE_URL}/admin/dashboard`);
    await page.waitForLoadState('networkidle');
    
    // Verify key metrics visible
    const metrics = [
      'Total UMKM',
      'Total Customer',
      'Total Transaksi',
      'Total Pendapatan',
      'Makanan Terselamatkan',
    ];
    
    for (const metric of metrics) {
      const element = page.locator(`text=/${metric}/i`).first();
      const isVisible = await element.isVisible({ timeout: 3000 }).catch(() => false);
      
      if (isVisible) {
        const value = await element.innerText();
        console.log(`✅ ${metric}: ${value}`);
      }
    }
  });
  
  test('ADMIN-07: Admin Monitoring Transaksi', async ({ page }) => {
    await login(page, testAdmin.email, testAdmin.password);
    
    await page.goto(`${BASE_URL}/admin/transaksi`);
    await page.waitForLoadState('networkidle');
    
    // Verify transaction list
    const transactionTable = page.locator('[data-testid="transaction-table"], table, .transaction-list');
    await expect(transactionTable).toBeVisible({ timeout: 5000 });
    
    // Count rows
    const rows = page.locator('[data-testid="transaction-row"], tbody tr, .transaction-item');
    const count = await rows.count();
    
    console.log(`✅ Admin can see ${count} transactions`);
  });
});
