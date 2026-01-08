package com.cvt.sdk.adapters;

import com.cvt.sdk.ValidationResult;

import java.util.Collections;
import java.util.Map;

/**
 * Represents a captured HTTP interaction from the adapter.
 */
public class CapturedInteraction {
    private final String method;
    private final String path;
    private final Map<String, String> requestHeaders;
    private final String requestBody;
    private final int statusCode;
    private final Map<String, String> responseHeaders;
    private final String responseBody;
    private final ValidationResult validationResult;
    private final long timestamp;

    public CapturedInteraction(
            String method,
            String path,
            Map<String, String> requestHeaders,
            String requestBody,
            int statusCode,
            Map<String, String> responseHeaders,
            String responseBody,
            ValidationResult validationResult) {
        this.method = method;
        this.path = path;
        this.requestHeaders = requestHeaders != null ? Map.copyOf(requestHeaders) : Collections.emptyMap();
        this.requestBody = requestBody;
        this.statusCode = statusCode;
        this.responseHeaders = responseHeaders != null ? Map.copyOf(responseHeaders) : Collections.emptyMap();
        this.responseBody = responseBody;
        this.validationResult = validationResult;
        this.timestamp = System.currentTimeMillis();
    }

    /**
     * @return The HTTP method (GET, POST, etc.)
     */
    public String getMethod() {
        return method;
    }

    /**
     * @return The request path
     */
    public String getPath() {
        return path;
    }

    /**
     * @return The request headers
     */
    public Map<String, String> getRequestHeaders() {
        return requestHeaders;
    }

    /**
     * @return The request body (may be null)
     */
    public String getRequestBody() {
        return requestBody;
    }

    /**
     * @return The response status code
     */
    public int getStatusCode() {
        return statusCode;
    }

    /**
     * @return The response headers
     */
    public Map<String, String> getResponseHeaders() {
        return responseHeaders;
    }

    /**
     * @return The response body (may be null)
     */
    public String getResponseBody() {
        return responseBody;
    }

    /**
     * @return The validation result (null if autoValidate was disabled)
     */
    public ValidationResult getValidationResult() {
        return validationResult;
    }

    /**
     * @return The timestamp when the interaction was captured
     */
    public long getTimestamp() {
        return timestamp;
    }

    /**
     * @return true if the interaction was validated and passed
     */
    public boolean isValid() {
        return validationResult != null && validationResult.isValid();
    }

    @Override
    public String toString() {
        return "CapturedInteraction{" +
                "method='" + method + '\'' +
                ", path='" + path + '\'' +
                ", statusCode=" + statusCode +
                ", valid=" + (validationResult != null ? validationResult.isValid() : "not validated") +
                '}';
    }
}
