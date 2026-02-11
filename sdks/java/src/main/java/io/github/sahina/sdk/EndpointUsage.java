package io.github.sahina.sdk;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

/**
 * Describes which endpoints and fields a consumer uses.
 */
public class EndpointUsage {
    private final String method;
    private final String path;
    private final List<String> usedFields;

    /**
     * Creates a new EndpointUsage instance.
     *
     * @param method     HTTP method (GET, POST, etc.)
     * @param path       API path (e.g., "/users/{id}")
     * @param usedFields Fields used in response (e.g., ["email", "name"])
     */
    public EndpointUsage(String method, String path, List<String> usedFields) {
        this.method = method;
        this.path = path;
        this.usedFields = usedFields != null ? new ArrayList<>(usedFields) : new ArrayList<>();
    }

    /**
     * Creates a new EndpointUsage instance without specifying used fields.
     *
     * @param method HTTP method (GET, POST, etc.)
     * @param path   API path (e.g., "/users/{id}")
     */
    public EndpointUsage(String method, String path) {
        this(method, path, null);
    }

    /**
     * Gets the HTTP method.
     *
     * @return The HTTP method
     */
    public String getMethod() {
        return method;
    }

    /**
     * Gets the API path.
     *
     * @return The API path
     */
    public String getPath() {
        return path;
    }

    /**
     * Gets the list of used fields.
     *
     * @return An unmodifiable list of used fields
     */
    public List<String> getUsedFields() {
        return Collections.unmodifiableList(usedFields);
    }

    @Override
    public String toString() {
        return "EndpointUsage{" +
                "method='" + method + '\'' +
                ", path='" + path + '\'' +
                ", usedFields=" + usedFields +
                '}';
    }
}
