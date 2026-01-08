package com.cvt.sdk.producer;

import java.util.HashMap;
import java.util.Map;

/**
 * Response data for producer testing validation.
 */
public class TestResponseData {
    private final int statusCode;
    private final Map<String, String> headers;
    private final Object body;

    private TestResponseData(Builder builder) {
        this.statusCode = builder.statusCode;
        this.headers = builder.headers;
        this.body = builder.body;
    }

    /**
     * Gets the HTTP status code.
     *
     * @return Status code
     */
    public int getStatusCode() {
        return statusCode;
    }

    /**
     * Gets the response headers.
     *
     * @return Headers map
     */
    public Map<String, String> getHeaders() {
        return headers;
    }

    /**
     * Gets the response body.
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
     * Builder for TestResponseData.
     */
    public static class Builder {
        private int statusCode;
        private Map<String, String> headers = new HashMap<>();
        private Object body;

        /**
         * Sets the HTTP status code.
         *
         * @param statusCode The status code
         * @return This builder
         */
        public Builder statusCode(int statusCode) {
            this.statusCode = statusCode;
            return this;
        }

        /**
         * Sets the response headers.
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
         * Sets the response body.
         *
         * @param body The body (any JSON-serializable type)
         * @return This builder
         */
        public Builder body(Object body) {
            this.body = body;
            return this;
        }

        /**
         * Builds the TestResponseData.
         *
         * @return A new TestResponseData instance
         */
        public TestResponseData build() {
            return new TestResponseData(this);
        }
    }
}
