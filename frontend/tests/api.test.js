import { test } from 'node:test';
import assert from 'node:assert';
import { apiPost } from '../src/lib/api.js';

function mockJsonError(status, body) {
  return async () => ({
    ok: false,
    status,
    headers: new Headers({ 'content-type': 'application/json' }),
    json: async () => body,
  });
}

test('apiPost extracts message from structured error object instead of [object Object]', async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => { globalThis.fetch = originalFetch; });

  globalThis.fetch = mockJsonError(401, {
    success: false,
    data: null,
    error: { code: 'UNAUTHORIZED', message: 'Kata sandi/email salah' },
  });

  await assert.rejects(
    () => apiPost('/auth/login', { email: 'a@b.com', password: 'wrong' }),
    { message: 'Kata sandi/email salah' }
  );
});

test('apiPost keeps string error and HTTP status fallback working', async (t) => {
  const originalFetch = globalThis.fetch;
  t.after(() => { globalThis.fetch = originalFetch; });

  globalThis.fetch = mockJsonError(400, { error: 'Permintaan tidak valid' });
  await assert.rejects(() => apiPost('/test', {}), { message: 'Permintaan tidak valid' });

  globalThis.fetch = mockJsonError(500, {});
  await assert.rejects(() => apiPost('/test', {}), { message: 'HTTP 500' });
});
