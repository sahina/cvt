package com.cvt.sdk;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

/**
 * Options for registering a consumer.
 */
public class RegisterConsumerOptions {
    private final String consumerId;
    private final String consumerVersion;
    private final String schemaId;
    private final String schemaVersion;
    private final String environment;
    private final List<EndpointUsage> usedEndpoints;

    private RegisterConsumerOptions(Builder builder) {
        this.consumerId = builder.consumerId;
        this.consumerVersion = builder.consumerVersion;
        this.schemaId = builder.schemaId;
        this.schemaVersion = builder.schemaVersion;
        this.environment = builder.environment;
        this.usedEndpoints = builder.usedEndpoints != null
                ? new ArrayList<>(builder.usedEndpoints)
                : new ArrayList<>();
    }

    /**
     * Creates a new builder.
     *
     * @return A new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Gets the consumer ID.
     *
     * @return The consumer ID
     */
    public String getConsumerId() {
        return consumerId;
    }

    /**
     * Gets the consumer version.
     *
     * @return The consumer version
     */
    public String getConsumerVersion() {
        return consumerVersion;
    }

    /**
     * Gets the schema ID.
     *
     * @return The schema ID
     */
    public String getSchemaId() {
        return schemaId;
    }

    /**
     * Gets the schema version.
     *
     * @return The schema version
     */
    public String getSchemaVersion() {
        return schemaVersion;
    }

    /**
     * Gets the environment.
     *
     * @return The environment
     */
    public String getEnvironment() {
        return environment;
    }

    /**
     * Gets the used endpoints.
     *
     * @return An unmodifiable list of used endpoints
     */
    public List<EndpointUsage> getUsedEndpoints() {
        return Collections.unmodifiableList(usedEndpoints);
    }

    /**
     * Builder for RegisterConsumerOptions.
     */
    public static class Builder {
        private String consumerId;
        private String consumerVersion;
        private String schemaId;
        private String schemaVersion;
        private String environment;
        private List<EndpointUsage> usedEndpoints;

        /**
         * Sets the consumer ID.
         *
         * @param consumerId Unique consumer identifier (e.g., "order-service")
         * @return This builder
         */
        public Builder consumerId(String consumerId) {
            this.consumerId = consumerId;
            return this;
        }

        /**
         * Sets the consumer version.
         *
         * @param consumerVersion Consumer's version (e.g., "2.1.0")
         * @return This builder
         */
        public Builder consumerVersion(String consumerVersion) {
            this.consumerVersion = consumerVersion;
            return this;
        }

        /**
         * Sets the schema ID.
         *
         * @param schemaId Schema this consumer depends on
         * @return This builder
         */
        public Builder schemaId(String schemaId) {
            this.schemaId = schemaId;
            return this;
        }

        /**
         * Sets the schema version.
         *
         * @param schemaVersion Schema version consumer was tested against
         * @return This builder
         */
        public Builder schemaVersion(String schemaVersion) {
            this.schemaVersion = schemaVersion;
            return this;
        }

        /**
         * Sets the environment.
         *
         * @param environment Environment (dev, staging, prod)
         * @return This builder
         */
        public Builder environment(String environment) {
            this.environment = environment;
            return this;
        }

        /**
         * Sets the used endpoints.
         *
         * @param usedEndpoints Which endpoints the consumer uses
         * @return This builder
         */
        public Builder usedEndpoints(List<EndpointUsage> usedEndpoints) {
            this.usedEndpoints = usedEndpoints;
            return this;
        }

        /**
         * Builds a new RegisterConsumerOptions instance.
         *
         * @return A new RegisterConsumerOptions
         */
        public RegisterConsumerOptions build() {
            return new RegisterConsumerOptions(this);
        }
    }
}
