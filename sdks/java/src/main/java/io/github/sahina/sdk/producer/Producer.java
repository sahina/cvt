package io.github.sahina.sdk.producer;

import java.util.Map;
import java.util.concurrent.atomic.AtomicLong;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * Producer handles server-side validation of HTTP requests and responses.
 *
 * <p>Example:
 * <pre>{@code
 * ProducerConfig config = ProducerConfig.builder()
 *         .schemaId("my-api")
 *         .validator(myValidator)
 *         .mode(ValidationMode.STRICT)
 *         .build();
 *
 * Producer producer = new Producer(config);
 *
 * // Validate request
 * ProducerValidationResult result = producer.validateRequest(
 *         "GET", "/users", headers, null);
 * if (!result.isValid()) {
 *     // Handle failure
 * }
 * }</pre>
 */
public class Producer {
    private static final Logger LOGGER = Logger.getLogger(Producer.class.getName());

    private final ProducerConfig config;

    // Metrics
    private static final AtomicLong requestValidations = new AtomicLong(0);
    private static final AtomicLong requestValidationsPassed = new AtomicLong(0);
    private static final AtomicLong requestValidationsFailed = new AtomicLong(0);
    private static final AtomicLong responseValidations = new AtomicLong(0);
    private static final AtomicLong responseValidationsPassed = new AtomicLong(0);
    private static final AtomicLong responseValidationsFailed = new AtomicLong(0);
    private static final AtomicLong requestsRejected = new AtomicLong(0);

    /**
     * Creates a new Producer with the given configuration.
     *
     * @param config The producer configuration
     */
    public Producer(ProducerConfig config) {
        this.config = config;
    }

    /**
     * Check if a path should be validated.
     *
     * @param path The path to check
     * @return true if the path should be validated
     */
    public boolean shouldValidatePath(String path) {
        return config.shouldValidatePath(path);
    }

    /**
     * Validates an incoming HTTP request.
     *
     * @param method  HTTP method (GET, POST, etc.)
     * @param path    Request path including query string
     * @param headers Request headers
     * @param body    Request body (as string)
     * @return Validation result
     */
    public ProducerValidationResult validateRequest(
            String method,
            String path,
            Map<String, String> headers,
            String body) {

        if (!config.isValidateRequest()) {
            return ProducerValidationResult.success("request");
        }

        try {
            ProducerValidationResult result = config.getValidator().validate(
                    config.getSchemaId(),
                    method,
                    path,
                    headers,
                    body,
                    200, // Minimal valid response for request-only validation
                    null,
                    "{}");

            return new ProducerValidationResult(result.isValid(), result.getErrors(), "request");
        } catch (Exception e) {
            LOGGER.log(Level.SEVERE, "[" + config.getLogPrefix() + "] Request validation error", e);
            return ProducerValidationResult.success("request"); // Continue on error
        }
    }

    /**
     * Validates an outgoing HTTP response.
     *
     * @param method          HTTP method of the original request
     * @param path            Request path including query string
     * @param requestHeaders  Original request headers
     * @param requestBody     Original request body
     * @param statusCode      Response status code
     * @param responseHeaders Response headers
     * @param responseBody    Response body (as string)
     * @return Validation result
     */
    public ProducerValidationResult validateResponse(
            String method,
            String path,
            Map<String, String> requestHeaders,
            String requestBody,
            int statusCode,
            Map<String, String> responseHeaders,
            String responseBody) {

        if (!config.isValidateResponse()) {
            return ProducerValidationResult.success("response");
        }

        try {
            ProducerValidationResult result = config.getValidator().validate(
                    config.getSchemaId(),
                    method,
                    path,
                    requestHeaders,
                    requestBody,
                    statusCode,
                    responseHeaders,
                    responseBody);

            return new ProducerValidationResult(result.isValid(), result.getErrors(), "response");
        } catch (Exception e) {
            LOGGER.log(Level.SEVERE, "[" + config.getLogPrefix() + "] Response validation error", e);
            return ProducerValidationResult.success("response"); // Continue on error
        }
    }

    /**
     * Handles request validation failure based on mode.
     *
     * @param result  The validation result
     * @param request The original request object (framework-specific)
     * @return Object array: [shouldContinue (Boolean), customResponse (Object or null)]
     */
    public Object[] handleRequestFailure(ProducerValidationResult result, Object request) {
        // Call custom handler if configured
        if (config.getOnRequestFailure() != null) {
            try {
                Object customResponse = config.getOnRequestFailure().apply(result, request);
                if (customResponse != null) {
                    return new Object[]{false, customResponse};
                }
            } catch (Exception e) {
                LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Error in onRequestFailure handler", e);
            }
        }

        switch (config.getMode()) {
            case STRICT:
                recordRejection();
                return new Object[]{false, null};

            case WARN:
                logValidationFailure("request", request, result);
                return new Object[]{true, null};

            case SHADOW:
                recordValidationMetrics("request", result);
                return new Object[]{true, null};

            default:
                return new Object[]{true, null};
        }
    }

    /**
     * Handles response validation failure based on mode.
     *
     * @param result   The validation result
     * @param request  The original request object
     * @param response The response object
     */
    public void handleResponseFailure(ProducerValidationResult result, Object request, Object response) {
        // Call custom handler if configured
        if (config.getOnResponseFailure() != null) {
            try {
                config.getOnResponseFailure().accept(result);
                return;
            } catch (Exception e) {
                LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Error in onResponseFailure handler", e);
            }
        }

        switch (config.getMode()) {
            case STRICT:
            case WARN:
                // Log the failure (can't modify response - already sent)
                logValidationFailure("response", request, result);
                break;

            case SHADOW:
                // Metrics only
                recordValidationMetrics("response", result);
                break;
        }
    }

    /**
     * @return The validation mode
     */
    public ValidationMode getMode() {
        return config.getMode();
    }

    private void logValidationFailure(String type, Object request, ProducerValidationResult result) {
        LOGGER.log(Level.WARNING,
                String.format("[%s] %s validation failed: %s",
                        config.getLogPrefix(), type, result.getErrors()));
    }

    // Static metrics methods

    /**
     * Records validation metrics.
     *
     * @param type   Type of validation ("request" or "response")
     * @param result The validation result
     */
    public static void recordValidationMetrics(String type, ProducerValidationResult result) {
        if ("request".equals(type)) {
            requestValidations.incrementAndGet();
            if (result.isValid()) {
                requestValidationsPassed.incrementAndGet();
            } else {
                requestValidationsFailed.incrementAndGet();
            }
        } else {
            responseValidations.incrementAndGet();
            if (result.isValid()) {
                responseValidationsPassed.incrementAndGet();
            } else {
                responseValidationsFailed.incrementAndGet();
            }
        }
    }

    /**
     * Records a request rejection.
     */
    public static void recordRejection() {
        requestsRejected.incrementAndGet();
    }

    /**
     * Gets the current metrics snapshot.
     *
     * @return Metrics snapshot
     */
    public static Metrics getMetrics() {
        return new Metrics(
                requestValidations.get(),
                requestValidationsPassed.get(),
                requestValidationsFailed.get(),
                responseValidations.get(),
                responseValidationsPassed.get(),
                responseValidationsFailed.get(),
                requestsRejected.get());
    }

    /**
     * Resets all metrics.
     */
    public static void resetMetrics() {
        requestValidations.set(0);
        requestValidationsPassed.set(0);
        requestValidationsFailed.set(0);
        responseValidations.set(0);
        responseValidationsPassed.set(0);
        responseValidationsFailed.set(0);
        requestsRejected.set(0);
    }

    /**
     * Metrics snapshot.
     */
    public static class Metrics {
        private final long requestValidations;
        private final long requestValidationsPassed;
        private final long requestValidationsFailed;
        private final long responseValidations;
        private final long responseValidationsPassed;
        private final long responseValidationsFailed;
        private final long requestsRejected;

        public Metrics(long requestValidations, long requestValidationsPassed,
                       long requestValidationsFailed, long responseValidations,
                       long responseValidationsPassed, long responseValidationsFailed,
                       long requestsRejected) {
            this.requestValidations = requestValidations;
            this.requestValidationsPassed = requestValidationsPassed;
            this.requestValidationsFailed = requestValidationsFailed;
            this.responseValidations = responseValidations;
            this.responseValidationsPassed = responseValidationsPassed;
            this.responseValidationsFailed = responseValidationsFailed;
            this.requestsRejected = requestsRejected;
        }

        public long getRequestValidations() { return requestValidations; }
        public long getRequestValidationsPassed() { return requestValidationsPassed; }
        public long getRequestValidationsFailed() { return requestValidationsFailed; }
        public long getResponseValidations() { return responseValidations; }
        public long getResponseValidationsPassed() { return responseValidationsPassed; }
        public long getResponseValidationsFailed() { return responseValidationsFailed; }
        public long getRequestsRejected() { return requestsRejected; }
    }
}
