package io.github.sahina.sdk.producer;

import java.util.HashMap;
import java.util.Map;

/**
 * Request context for producer testing validation.
 * Provides optional request context for path parameter extraction.
 */
public class TestRequestContext {
    private final String method;
    private final String path;
    private final Map<String, String> headers;
    private final Object body;

    private TestRequestContext(Builder builder) {
        this.method = builder.method;
        this.path = builder.path;
        this.headers = builder.headers;
        this.body = builder.body;
    }

    /**
     * Gets the HTTP method.
     *
     * @return HTTP method
     */
    public String getMethod() {
        return method;
    }

    /**
     * Gets the request path.
     *
     * @return Request path
     */
    public String getPath() {
        return path;
    }

    /**
     * Gets the request headers.
     *
     * @return Headers map
     */
    public Map<String, String> getHeaders() {
        return headers;
    }

    /**
     * Gets the request body.
     *
     * @return Body (can be any JSON-serializable type)
     */
    public Object getBody() {
        return body;
    }

    /**
     * Creates a new builder.
     *
     * @return A new builder
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for TestRequestContext.
     */
    public static class Builder {
        private String method;
        private String path;
        private Map<String, String> headers = new HashMap<>();
        private Object body;

        /**
         * Sets the HTTP method.
         *
         * @param method The HTTP method
         * @return This builder
         */
        public Builder method(String method) {
            this.method = method;
            return this;
        }

        /**
         * Sets the request path.
         *
         * @param path The request path
         * @return This builder
         */
        public Builder path(String path) {
            this.path = path;
            return this;
        }

        /**
         * Sets the request headers.
         *
         * @param headers The headers map
         * @return This builder
         */
        public Builder headers(Map<String, String> headers) {
            this.headers = headers != null ? new HashMap<>(headers) : new HashMap<>();
            return this;
        }

        /**
         * Adds a header.
         *
         * @param name  Header name
         * @param value Header value
         * @return This builder
         */
        public Builder header(String name, String value) {
            this.headers.put(name, value);
            return this;
        }

        /**
         * Sets the request body.
         *
         * @param body The body (any JSON-serializable type)
         * @return This builder
         */
        public Builder body(Object body) {
            this.body = body;
            return this;
        }

        /**
         * Builds the TestRequestContext.
         *
         * @return A new TestRequestContext instance
         */
        public TestRequestContext build() {
            return new TestRequestContext(this);
        }
    }
}
