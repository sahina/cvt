import type { EndpointUsage, RegisterConsumerOptions } from "./index";
import type { CapturedInteraction } from "./adapters/types";

/**
 * Configuration for auto-registration of consumers from captured interactions.
 */
export interface AutoRegisterConfig {
  /** Unique consumer identifier (required, e.g., "order-service") */
  consumerId: string;
  /** Consumer's version (required, e.g., "2.1.0") */
  consumerVersion: string;
  /** Deployment environment (required, e.g., "dev", "staging", "prod") */
  environment: string;
  /** Schema version being tested against (required, e.g., "1.0.0") */
  schemaVersion: string;
  /**
   * Override auto-extraction from URL hostname (optional).
   * If empty, schemaId is extracted from the mock URL hostname.
   * For example, "http://mock.user-api/users/123" extracts "user-api".
   */
  schemaId?: string;
}

/**
 * Extracts the schemaId from a mock URL.
 * For example: "http://mock.user-api/users/123" returns "user-api"
 */
export function extractSchemaIdFromUrl(urlStr: string): string | null {
  try {
    const url = new URL(urlStr);
    const host = url.hostname;
    if (!host) return null;

    // Strip "mock." prefix if present
    if (host.startsWith("mock.")) {
      return host.slice(5);
    }
    return host;
  } catch {
    return null;
  }
}

/**
 * Extracts the schemaId from captured interactions.
 * Returns an error if multiple different schemaIds are detected.
 */
export function extractSchemaIdFromInteractions(
  interactions: CapturedInteraction[],
): { schemaId: string | null; error: string | null } {
  const schemaIds = new Set<string>();

  for (const interaction of interactions) {
    const path = interaction.request.path;

    // If path starts with http:// or https://, extract hostname
    if (path.startsWith("http://") || path.startsWith("https://")) {
      const schemaId = extractSchemaIdFromUrl(path);
      if (schemaId) {
        schemaIds.add(schemaId);
      }
    }
  }

  if (schemaIds.size === 0) {
    return {
      schemaId: null,
      error:
        "could not extract schemaId from interactions; provide schemaId in config",
    };
  }

  if (schemaIds.size > 1) {
    const ids = Array.from(schemaIds).sort().join(", ");
    return {
      schemaId: null,
      error: `multiple schemas detected (${ids}); provide explicit schemaId in config`,
    };
  }

  return { schemaId: Array.from(schemaIds)[0], error: null };
}

/**
 * Normalizes a path by extracting just the path portion from a URL.
 */
export function normalizePathForEndpoint(pathOrUrl: string): string {
  // If it's a full URL, parse it to extract just the path
  if (pathOrUrl.startsWith("http://") || pathOrUrl.startsWith("https://")) {
    try {
      const url = new URL(pathOrUrl);
      pathOrUrl = url.pathname;
    } catch {
      // Keep original if parse fails
    }
  }

  // Remove query string if present
  const queryIndex = pathOrUrl.indexOf("?");
  if (queryIndex !== -1) {
    pathOrUrl = pathOrUrl.slice(0, queryIndex);
  }

  return pathOrUrl;
}

/**
 * Recursively extracts all field paths from a JSON body.
 * Uses dot notation for nested fields (e.g., "user.address.city").
 */
export function extractFieldsFromBody(
  body: unknown,
  prefix: string = "",
): string[] {
  if (body === null || body === undefined) {
    return [];
  }

  const fields: string[] = [];

  if (typeof body === "object" && !Array.isArray(body)) {
    for (const [key, value] of Object.entries(
      body as Record<string, unknown>,
    )) {
      const fieldPath = prefix ? `${prefix}.${key}` : key;
      fields.push(fieldPath);
      // Recursively extract nested fields
      fields.push(...extractFieldsFromBody(value, fieldPath));
    }
  } else if (Array.isArray(body) && body.length > 0) {
    // For arrays, extract fields from the first element as representative
    fields.push(...extractFieldsFromBody(body[0], prefix));
  }

  return fields;
}

/**
 * Merges two arrays, removing duplicates.
 */
function mergeArrays(a: string[], b: string[]): string[] {
  const seen = new Set([...a, ...b]);
  return Array.from(seen);
}

/**
 * Converts captured interactions to endpoint usage,
 * deduplicating by method+path and merging usedFields.
 */
export function mergeInteractionsToEndpoints(
  interactions: CapturedInteraction[],
): EndpointUsage[] {
  const endpointMap = new Map<string, EndpointUsage>();

  for (const interaction of interactions) {
    const path = normalizePathForEndpoint(interaction.request.path);
    const key = `${interaction.request.method}:${path}`;

    const fields = extractFieldsFromBody(interaction.response.body);

    const existing = endpointMap.get(key);
    if (existing) {
      // Merge fields (union)
      existing.usedFields = mergeArrays(existing.usedFields || [], fields);
    } else {
      endpointMap.set(key, {
        method: interaction.request.method,
        path,
        usedFields: fields,
      });
    }
  }

  // Convert to sorted array for deterministic output
  const entries = Array.from(endpointMap.entries()).sort(([a], [b]) =>
    a.localeCompare(b),
  );

  return entries.map(([, ep]) => ({
    ...ep,
    usedFields: (ep.usedFields || []).sort(),
  }));
}

/**
 * Builds consumer registration options from captured interactions.
 * Useful for preview/dry-run scenarios.
 */
export function buildConsumerFromInteractions(
  interactions: CapturedInteraction[],
  config: AutoRegisterConfig,
): { options: RegisterConsumerOptions | null; error: string | null } {
  // Validate required fields
  if (!config.consumerId) {
    return { options: null, error: "consumerId is required" };
  }
  if (!config.consumerVersion) {
    return { options: null, error: "consumerVersion is required" };
  }
  if (!config.environment) {
    return { options: null, error: "environment is required" };
  }
  if (!config.schemaVersion) {
    return { options: null, error: "schemaVersion is required" };
  }

  // Validate interactions
  if (interactions.length === 0) {
    return { options: null, error: "no interactions to register" };
  }

  // Extract schemaId from interactions or use provided override
  let schemaId = config.schemaId;
  if (!schemaId) {
    const result = extractSchemaIdFromInteractions(interactions);
    if (result.error) {
      return { options: null, error: result.error };
    }
    schemaId = result.schemaId!;
  }

  // Merge interactions into endpoint usage
  const usedEndpoints = mergeInteractionsToEndpoints(interactions);

  return {
    options: {
      consumerId: config.consumerId,
      consumerVersion: config.consumerVersion,
      schemaId,
      schemaVersion: config.schemaVersion,
      environment: config.environment,
      usedEndpoints,
    },
    error: null,
  };
}

// Re-export CapturedInteraction for convenience
export type { CapturedInteraction };
