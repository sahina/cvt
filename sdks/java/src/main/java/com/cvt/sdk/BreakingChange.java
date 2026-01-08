package com.cvt.sdk;

/**
 * Represents a breaking change detected between schema versions.
 */
public class BreakingChange {
    private final String type;
    private final String path;
    private final String method;
    private final String description;
    private final String oldValue;
    private final String newValue;

    /**
     * Creates a new BreakingChange instance.
     *
     * @param type        Type of breaking change (e.g., ENDPOINT_REMOVED)
     * @param path        API path affected (e.g., "/pet/{petId}")
     * @param method      HTTP method affected (e.g., "DELETE")
     * @param description Human-readable description
     * @param oldValue    Previous value (for context)
     * @param newValue    New value (for context)
     */
    public BreakingChange(String type, String path, String method, String description,
                          String oldValue, String newValue) {
        this.type = type;
        this.path = path;
        this.method = method;
        this.description = description;
        this.oldValue = oldValue;
        this.newValue = newValue;
    }

    /**
     * Gets the type of breaking change.
     *
     * @return The breaking change type (e.g., ENDPOINT_REMOVED, REQUIRED_FIELD_ADDED)
     */
    public String getType() {
        return type;
    }

    /**
     * Gets the API path affected by the breaking change.
     *
     * @return The API path (e.g., "/pet/{petId}")
     */
    public String getPath() {
        return path;
    }

    /**
     * Gets the HTTP method affected by the breaking change.
     *
     * @return The HTTP method (e.g., "DELETE", "POST")
     */
    public String getMethod() {
        return method;
    }

    /**
     * Gets the human-readable description of the breaking change.
     *
     * @return The description
     */
    public String getDescription() {
        return description;
    }

    /**
     * Gets the previous value (for context in type changes).
     *
     * @return The old value, or null if not applicable
     */
    public String getOldValue() {
        return oldValue;
    }

    /**
     * Gets the new value (for context in type changes).
     *
     * @return The new value, or null if not applicable
     */
    public String getNewValue() {
        return newValue;
    }

    @Override
    public String toString() {
        return String.format("BreakingChange{type='%s', path='%s', method='%s', description='%s'}",
                type, path, method, description);
    }
}
