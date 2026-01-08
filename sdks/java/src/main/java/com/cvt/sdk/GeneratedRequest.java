package com.cvt.sdk;

import java.util.Collections;
import java.util.Map;

/**
 * Represents a generated HTTP request from fixture generation.
 */
public class GeneratedRequest {
    private final String method;
    private final String path;
    private final Map<String, String> headers;
    private final Object body;

    public GeneratedRequest(String method, String path, Map<String, String> headers, Object body) {
        this.method = method;
        this.path = path;
        this.headers = headers != null ? Map.copyOf(headers) : Collections.emptyMap();
        this.body = body;
    }

    /**
     * @return The HTTP method (GET, POST, etc.)
     */
    public String getMethod() {
        return method;
    }

    /**
     * @return The API path
     */
    public String getPath() {
        return path;
    }

    /**
     * @return The request headers
     */
    public Map<String, String> getHeaders() {
        return headers;
    }

    /**
     * @return The request body (parsed from JSON)
     */
    public Object getBody() {
        return body;
    }

    @Override
    public String toString() {
        return "GeneratedRequest{" +
                "method='" + method + '\'' +
                ", path='" + path + '\'' +
                ", headers=" + headers +
                ", body=" + body +
                '}';
    }
}
