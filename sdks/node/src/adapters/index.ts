/**
 * CVT SDK Adapters
 *
 * This module provides convenience adapters for popular HTTP clients
 * that automatically capture and validate HTTP interactions.
 *
 * @example
 * ```typescript
 * // Axios adapter
 * import { createAxiosAdapter } from '@cvt/sdk/adapters';
 * const adapter = createAxiosAdapter({ axios: api, validator });
 *
 * // Fetch adapter (native fetch API)
 * import { createFetchAdapter } from '@cvt/sdk/adapters';
 * const fetchAdapter = createFetchAdapter({ validator, baseURL: 'http://api.test' });
 * const response = await fetchAdapter.fetch('/pet/1');
 * ```
 */

export * from "./types";
export * from "./axios";
export * from "./fetch";
export * from "./mock";
