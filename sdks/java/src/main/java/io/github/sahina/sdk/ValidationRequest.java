package io.github.sahina.sdk;

import java.util.HashMap;
import java.util.Map;

/**
 * Represents an HTTP request to be validated against an OpenAPI schema.
 */
public class ValidationRequest {
    private final String method;
    private final String path;
    private final Map<String, String> headers;
    private final String body;

    private ValidationRequest(Builder builder) {
        this.method = builder.method;
        this.path = builder.path;
        this.headers = builder.headers;
        this.body = builder.body;
    }

    public String getMethod() {
        return method;
    }

    public String getPath() {
        return path;
    }

    public Map<String, String> getHeaders() {
        return headers;
    }

    public String getBody() {
        return body;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private String method;
        private String path;
        private Map<String, String> headers = new HashMap<>();
        private String body = "";

        public Builder method(String method) {
            this.method = method;
            return this;
        }

        public Builder path(String path) {
            this.path = path;
            return this;
        }

        public Builder headers(Map<String, String> headers) {
            this.headers = new HashMap<>(headers);
            return this;
        }

        public Builder header(String key, String value) {
            this.headers.put(key, value);
            return this;
        }

        public Builder body(String body) {
            this.body = body;
            return this;
        }

        public ValidationRequest build() {
            if (method == null || method.isEmpty()) {
                throw new IllegalArgumentException("HTTP method is required");
            }
            if (path == null || path.isEmpty()) {
                throw new IllegalArgumentException("Path is required");
            }
            return new ValidationRequest(this);
        }
    }
}
