package com.cvt.sdk.producer;

import java.util.Map;

/**
 * Interface for validators that can validate HTTP interactions.
 * Can be implemented by gRPC client wrapper or custom validators.
 */
public interface Validator {
    /**
     * Validates an HTTP interaction against a schema.
     *
     * @param schemaId        The schema identifier
     * @param method          HTTP method (GET, POST, etc.)
     * @param path            Request path with query string
     * @param requestHeaders  Request headers
     * @param requestBody     Request body (as string or object)
     * @param statusCode      Response status code
     * @param responseHeaders Response headers
     * @param responseBody    Response body (as string or object)
     * @return Validation result
     */
    ProducerValidationResult validate(
            String schemaId,
            String method,
            String path,
            Map<String, String> requestHeaders,
            String requestBody,
            int statusCode,
            Map<String, String> responseHeaders,
            String responseBody);
}
