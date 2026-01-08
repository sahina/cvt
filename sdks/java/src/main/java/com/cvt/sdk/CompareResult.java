package com.cvt.sdk;

import java.util.List;

/**
 * Result of comparing two schema versions for breaking changes.
 */
public class CompareResult {
    private final boolean compatible;
    private final List<BreakingChange> breakingChanges;

    /**
     * Creates a new CompareResult instance.
     *
     * @param compatible      True if no breaking changes were detected
     * @param breakingChanges List of breaking changes detected
     */
    public CompareResult(boolean compatible, List<BreakingChange> breakingChanges) {
        this.compatible = compatible;
        this.breakingChanges = breakingChanges;
    }

    /**
     * Checks if the schemas are compatible (no breaking changes).
     *
     * @return True if compatible, false if breaking changes were detected
     */
    public boolean isCompatible() {
        return compatible;
    }

    /**
     * Gets the list of breaking changes detected.
     *
     * @return List of breaking changes, empty if compatible
     */
    public List<BreakingChange> getBreakingChanges() {
        return breakingChanges;
    }

    /**
     * Gets the number of breaking changes detected.
     *
     * @return The count of breaking changes
     */
    public int getBreakingChangeCount() {
        return breakingChanges != null ? breakingChanges.size() : 0;
    }

    @Override
    public String toString() {
        return String.format("CompareResult{compatible=%s, breakingChanges=%d}",
                compatible, getBreakingChangeCount());
    }
}
