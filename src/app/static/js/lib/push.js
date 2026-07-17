import { get, post } from '/static/js/lib/api.js';

function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; i++) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

// Registers the service worker on every page load. Safe to call
// unconditionally and repeatedly — registration is idempotent, and this
// alone requests no permission and creates no subscription.
export async function registerServiceWorker() {
  if (!('serviceWorker' in navigator)) return null;
  return navigator.serviceWorker.register('/sw.js');
}

// Requests notification permission and creates a push subscription. Must be
// called from a user gesture (e.g. a button click) — iOS Safari requires
// this and running inside the installed home-screen app, not a regular tab.
export async function enablePushNotifications() {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
    return { ok: false, error: 'Push notifications are not supported in this browser.' };
  }

  const permission = await Notification.requestPermission();
  if (permission !== 'granted') {
    return { ok: false, error: 'Notification permission was not granted.' };
  }

  const registration = await navigator.serviceWorker.ready;

  const keyRes = await get('/api/push_public_key');
  if (!keyRes.ok || !keyRes.data?.key) {
    return { ok: false, error: 'Could not load push key from server.' };
  }

  const subscription = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(keyRes.data.key),
  });

  const sub = subscription.toJSON();
  const res = await post('/api/push_subscribe', {
    endpoint: sub.endpoint,
    keys: { p256dh: sub.keys.p256dh, auth: sub.keys.auth },
  });

  if (!res.ok) {
    return { ok: false, error: res.error ?? 'Failed to save subscription.' };
  }
  return { ok: true };
}
