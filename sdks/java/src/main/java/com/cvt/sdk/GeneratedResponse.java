package com.cvt.sdk;

import java.util.Collections;
import java.util.Map;

/**
 * Represents a generated HTTP response from fixture generation.
 */
public class GeneratedResponse {
    private final int statusCode;
    private final Map<String, String> headers;
    private final Object body;

    public GeneratedResponse(int statusCode, Map<String, String> headers, Object body) {
        this.statusCode = statusCode;
        this.headers = headers != null ? Map.copyOf(headers) : Collections.emptyMap();
        this.body = body;
    }

    /**
     * @return The HTTP status code
     */
    public int getStatusCode() {
        return statusCode;
    }

    /**
     * @return The response headers
     */
    public Map<String, String> getHeaders() {
        return headers;
    }

    /**
     * @return The response body (parsed from JSON)
     */
    public Object getBody() {
        return body;
    }

    @Override
    public String toString() {
        return "GeneratedResponse{" +
                "statusCode=" + statusCode +
                ", headers=" + headers +
                ", body=" + body +
                '}';
    }
}
