package com.cvt.sdk;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

/**
 * Impact of schema changes on a specific consumer.
 */
public class ConsumerImpact {
    private final String consumerId;
    private final String consumerVersion;
    private final String currentSchemaVersion;
    private final String environment;
    private final boolean willBreak;
    private final List<BreakingChange> relevantChanges;

    /**
     * Creates a new ConsumerImpact instance.
     *
     * @param consumerId           Consumer identifier
     * @param consumerVersion      Consumer version
     * @param currentSchemaVersion Schema version consumer was tested against
     * @param environment          Environment
     * @param willBreak            True if consumer will be affected
     * @param relevantChanges      Breaking changes that affect this consumer
     */
    public ConsumerImpact(
            String consumerId,
            String consumerVersion,
            String currentSchemaVersion,
            String environment,
            boolean willBreak,
            List<BreakingChange> relevantChanges) {
        this.consumerId = consumerId;
        this.consumerVersion = consumerVersion;
        this.currentSchemaVersion = currentSchemaVersion;
        this.environment = environment;
        this.willBreak = willBreak;
        this.relevantChanges = relevantChanges != null ? new ArrayList<>(relevantChanges) : new ArrayList<>();
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
     * Gets the current schema version the consumer was tested against.
     *
     * @return The current schema version
     */
    public String getCurrentSchemaVersion() {
        return currentSchemaVersion;
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
     * Checks if the consumer will break.
     *
     * @return True if the consumer will be affected
     */
    public boolean isWillBreak() {
        return willBreak;
    }

    /**
     * Gets the relevant breaking changes.
     *
     * @return An unmodifiable list of breaking changes that affect this consumer
     */
    public List<BreakingChange> getRelevantChanges() {
        return Collections.unmodifiableList(relevantChanges);
    }

    @Override
    public String toString() {
        return "ConsumerImpact{" +
                "consumerId='" + consumerId + '\'' +
                ", consumerVersion='" + consumerVersion + '\'' +
                ", currentSchemaVersion='" + currentSchemaVersion + '\'' +
                ", environment='" + environment + '\'' +
                ", willBreak=" + willBreak +
                ", relevantChanges=" + relevantChanges +
                '}';
    }
}
