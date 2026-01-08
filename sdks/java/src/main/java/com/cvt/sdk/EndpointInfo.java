package com.cvt.sdk;

/**
 * Information about an API endpoint in a schema.
 */
public class EndpointInfo {
    private final String method;
    private final String path;
    private final String operationId;
    private final String summary;

    public EndpointInfo(String method, String path, String operationId, String summary) {
        this.method = method;
        this.path = path;
        this.operationId = operationId;
        this.summary = summary;
    }

    /**
     * @return The HTTP method (GET, POST, etc.)
     */
    public String getMethod() {
        return method;
    }

    /**
     * @return The API path (e.g., /users/{id})
     */
    public String getPath() {
        return path;
    }

    /**
     * @return The OpenAPI operationId (if defined)
     */
    public String getOperationId() {
        return operationId;
    }

    /**
     * @return The OpenAPI summary (if defined)
     */
    public String getSummary() {
        return summary;
    }

    @Override
    public String toString() {
        return "EndpointInfo{" +
                "method='" + method + '\'' +
                ", path='" + path + '\'' +
                ", operationId='" + operationId + '\'' +
                ", summary='" + summary + '\'' +
                '}';
    }
}
