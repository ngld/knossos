export const API_URL =
  // @ts-expect-error We don't have type definitions for window.knIsApp
  process.env.NODE_ENV !== 'production' && !window.knIsApp
    ? 'http://localhost:8100'
    : 'https://api.client.fsnebula.org';
