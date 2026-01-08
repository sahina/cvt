package com.cvt.sdk;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

/**
 * Information about a registered consumer.
 */
public class ConsumerInfo {
    private final String consumerId;
    private final String consumerVersion;
    private final String schemaId;
    private final String schemaVersion;
    private final String environment;
    private final long registeredAt;
    private final long lastValidatedAt;
    private final List<EndpointUsage> usedEndpoints;

    /**
     * Creates a new ConsumerInfo instance.
     *
     * @param consumerId      Unique consumer identifier (e.g., "order-service")
     * @param consumerVersion Consumer's version (e.g., "2.1.0")
     * @param schemaId        Schema this consumer depends on
     * @param schemaVersion   Schema version consumer was tested against
     * @param environment     Environment (dev, staging, prod)
     * @param registeredAt    Unix timestamp of registration
     * @param lastValidatedAt Unix timestamp of last successful validation
     * @param usedEndpoints   Which endpoints the consumer uses
     */
    public ConsumerInfo(
            String consumerId,
            String consumerVersion,
            String schemaId,
            String schemaVersion,
            String environment,
            long registeredAt,
            long lastValidatedAt,
            List<EndpointUsage> usedEndpoints) {
        this.consumerId = consumerId;
        this.consumerVersion = consumerVersion;
        this.schemaId = schemaId;
        this.schemaVersion = schemaVersion;
        this.environment = environment;
        this.registeredAt = registeredAt;
        this.lastValidatedAt = lastValidatedAt;
        this.usedEndpoints = usedEndpoints != null ? new ArrayList<>(usedEndpoints) : new ArrayList<>();
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
     * Gets the registration timestamp.
     *
     * @return Unix timestamp of registration
     */
    public long getRegisteredAt() {
        return registeredAt;
    }

    /**
     * Gets the last validation timestamp.
     *
     * @return Unix timestamp of last successful validation
     */
    public long getLastValidatedAt() {
        return lastValidatedAt;
    }

    /**
     * Gets the used endpoints.
     *
     * @return An unmodifiable list of used endpoints
     */
    public List<EndpointUsage> getUsedEndpoints() {
        return Collections.unmodifiableList(usedEndpoints);
    }

    @Override
    public String toString() {
        return "ConsumerInfo{" +
                "consumerId='" + consumerId + '\'' +
                ", consumerVersion='" + consumerVersion + '\'' +
                ", schemaId='" + schemaId + '\'' +
                ", schemaVersion='" + schemaVersion + '\'' +
                ", environment='" + environment + '\'' +
                ", registeredAt=" + registeredAt +
                ", lastValidatedAt=" + lastValidatedAt +
                ", usedEndpoints=" + usedEndpoints +
                '}';
    }
}
