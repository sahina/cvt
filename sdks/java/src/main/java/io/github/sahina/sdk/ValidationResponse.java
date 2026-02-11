package io.github.sahina.sdk;

import java.util.HashMap;
import java.util.Map;

/**
 * Represents an HTTP response to be validated against an OpenAPI schema.
 */
public class ValidationResponse {
    private final int statusCode;
    private final Map<String, String> headers;
    private final String body;

    private ValidationResponse(Builder builder) {
        this.statusCode = builder.statusCode;
        this.headers = builder.headers;
        this.body = builder.body;
    }

    public int getStatusCode() {
        return statusCode;
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
        private int statusCode = 200;
        private Map<String, String> headers = new HashMap<>();
        private String body = "";

        public Builder statusCode(int statusCode) {
            this.statusCode = statusCode;
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

        public ValidationResponse build() {
            return new ValidationResponse(this);
        }
    }
}
