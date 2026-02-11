package io.github.sahina.sdk;

/**
 * Represents a complete generated fixture containing both request and response.
 */
public class GeneratedFixture {
    private final GeneratedRequest request;
    private final GeneratedResponse response;

    public GeneratedFixture(GeneratedRequest request, GeneratedResponse response) {
        this.request = request;
        this.response = response;
    }

    /**
     * @return The generated HTTP request
     */
    public GeneratedRequest getRequest() {
        return request;
    }

    /**
     * @return The generated HTTP response
     */
    public GeneratedResponse getResponse() {
        return response;
    }

    @Override
    public String toString() {
        return "GeneratedFixture{" +
                "request=" + request +
                ", response=" + response +
                '}';
    }
}
