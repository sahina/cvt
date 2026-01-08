package com.cvt.sdk;

/**
 * Configuration for auto-registration of consumers from captured interactions.
 */
public class AutoRegisterConfig {
    private final String consumerId;
    private final String consumerVersion;
    private final String environment;
    private final String schemaVersion;
    private final String schemaId;

    private AutoRegisterConfig(Builder builder) {
        this.consumerId = builder.consumerId;
        this.consumerVersion = builder.consumerVersion;
        this.environment = builder.environment;
        this.schemaVersion = builder.schemaVersion;
        this.schemaId = builder.schemaId;
    }

    /**
     * @return Unique consumer identifier (e.g., "order-service")
     */
    public String getConsumerId() {
        return consumerId;
    }

    /**
     * @return Consumer's version (e.g., "2.1.0")
     */
    public String getConsumerVersion() {
        return consumerVersion;
    }

    /**
     * @return Deployment environment (e.g., "dev", "staging", "prod")
     */
    public String getEnvironment() {
        return environment;
    }

    /**
     * @return Schema version being tested against (e.g., "1.0.0")
     */
    public String getSchemaVersion() {
        return schemaVersion;
    }

    /**
     * @return Override for schemaId auto-extraction (may be null)
     */
    public String getSchemaId() {
        return schemaId;
    }

    /**
     * Creates a new builder for AutoRegisterConfig.
     *
     * @return A new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for AutoRegisterConfig.
     */
    public static class Builder {
        private String consumerId;
        private String consumerVersion;
        private String environment;
        private String schemaVersion;
        private String schemaId;

        /**
         * Sets the consumer identifier (required).
         *
         * @param consumerId Unique consumer identifier (e.g., "order-service")
         * @return This builder
         */
        public Builder consumerId(String consumerId) {
            this.consumerId = consumerId;
            return this;
        }

        /**
         * Sets the consumer version (required).
         *
         * @param consumerVersion Consumer's version (e.g., "2.1.0")
         * @return This builder
         */
        public Builder consumerVersion(String consumerVersion) {
            this.consumerVersion = consumerVersion;
            return this;
        }

        /**
         * Sets the environment (required).
         *
         * @param environment Deployment environment (e.g., "dev", "staging", "prod")
         * @return This builder
         */
        public Builder environment(String environment) {
            this.environment = environment;
            return this;
        }

        /**
         * Sets the schema version (required).
         *
         * @param schemaVersion Schema version being tested against (e.g., "1.0.0")
         * @return This builder
         */
        public Builder schemaVersion(String schemaVersion) {
            this.schemaVersion = schemaVersion;
            return this;
        }

        /**
         * Sets the schema ID override (optional).
         * If not set, schemaId is auto-extracted from URL hostnames.
         * For example, "http://mock.user-api/users/123" extracts "user-api".
         *
         * @param schemaId Override schemaId
         * @return This builder
         */
        public Builder schemaId(String schemaId) {
            this.schemaId = schemaId;
            return this;
        }

        /**
         * Builds the AutoRegisterConfig.
         *
         * @return A new AutoRegisterConfig instance
         */
        public AutoRegisterConfig build() {
            return new AutoRegisterConfig(this);
        }
    }
}
