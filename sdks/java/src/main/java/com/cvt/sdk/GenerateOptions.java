package com.cvt.sdk;

/**
 * Options for fixture generation.
 * Use the builder pattern to create instances with custom settings.
 */
public class GenerateOptions {
    private final int statusCode;
    private final boolean useExamples;
    private final String contentType;

    private GenerateOptions(Builder builder) {
        this.statusCode = builder.statusCode;
        this.useExamples = builder.useExamples;
        this.contentType = builder.contentType;
    }

    /**
     * @return The response status code to generate (0 = auto-select)
     */
    public int getStatusCode() {
        return statusCode;
    }

    /**
     * @return Whether to use schema examples when available
     */
    public boolean isUseExamples() {
        return useExamples;
    }

    /**
     * @return The content type for the generated fixture
     */
    public String getContentType() {
        return contentType;
    }

    /**
     * Creates a new builder for GenerateOptions.
     *
     * @return A new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for GenerateOptions.
     */
    public static class Builder {
        private int statusCode = 0;
        private boolean useExamples = true;
        private String contentType = "application/json";

        /**
         * Sets the response status code to generate.
         *
         * @param statusCode The HTTP status code (0 = auto-select)
         * @return This builder
         */
        public Builder statusCode(int statusCode) {
            this.statusCode = statusCode;
            return this;
        }

        /**
         * Sets whether to use schema examples when available.
         *
         * @param useExamples True to use examples from the schema
         * @return This builder
         */
        public Builder useExamples(boolean useExamples) {
            this.useExamples = useExamples;
            return this;
        }

        /**
         * Sets the content type for the generated fixture.
         *
         * @param contentType The content type (e.g., "application/json")
         * @return This builder
         */
        public Builder contentType(String contentType) {
            this.contentType = contentType;
            return this;
        }

        /**
         * Builds a new GenerateOptions instance.
         *
         * @return A new GenerateOptions
         */
        public GenerateOptions build() {
            return new GenerateOptions(this);
        }
    }
}
