package io.github.sahina.sdk;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

/**
 * Result of can-i-deploy check.
 */
public class CanIDeployResult {
    private final boolean safeToDeploy;
    private final String summary;
    private final List<BreakingChange> breakingChanges;
    private final List<ConsumerImpact> affectedConsumers;

    /**
     * Creates a new CanIDeployResult instance.
     *
     * @param safeToDeploy      True if safe to deploy
     * @param summary           Human-readable summary
     * @param breakingChanges   All breaking changes in the new version
     * @param affectedConsumers Impact on each affected consumer
     */
    public CanIDeployResult(
            boolean safeToDeploy,
            String summary,
            List<BreakingChange> breakingChanges,
            List<ConsumerImpact> affectedConsumers) {
        this.safeToDeploy = safeToDeploy;
        this.summary = summary;
        this.breakingChanges = breakingChanges != null ? new ArrayList<>(breakingChanges) : new ArrayList<>();
        this.affectedConsumers = affectedConsumers != null ? new ArrayList<>(affectedConsumers) : new ArrayList<>();
    }

    /**
     * Checks if it's safe to deploy.
     *
     * @return True if safe to deploy
     */
    public boolean isSafeToDeploy() {
        return safeToDeploy;
    }

    /**
     * Gets the summary.
     *
     * @return Human-readable summary
     */
    public String getSummary() {
        return summary;
    }

    /**
     * Gets the breaking changes.
     *
     * @return An unmodifiable list of all breaking changes in the new version
     */
    public List<BreakingChange> getBreakingChanges() {
        return Collections.unmodifiableList(breakingChanges);
    }

    /**
     * Gets the affected consumers.
     *
     * @return An unmodifiable list of affected consumers
     */
    public List<ConsumerImpact> getAffectedConsumers() {
        return Collections.unmodifiableList(affectedConsumers);
    }

    @Override
    public String toString() {
        return "CanIDeployResult{" +
                "safeToDeploy=" + safeToDeploy +
                ", summary='" + summary + '\'' +
                ", breakingChanges=" + breakingChanges +
                ", affectedConsumers=" + affectedConsumers +
                '}';
    }
}
