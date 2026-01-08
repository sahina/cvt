package com.cvt.sdk.producer;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class ProducerTest {

    /**
     * Mock validator that returns configurable results.
     */
    static class MockValidator implements Validator {
        private final boolean valid;
        private final List<String> errors;
        private int callCount = 0;
        private String lastSchemaId;
        private String lastMethod;
        private String lastPath;

        MockValidator(boolean valid, List<String> errors) {
            this.valid = valid;
            this.errors = errors;
        }

        MockValidator(boolean valid) {
            this(valid, Collections.emptyList());
        }

        @Override
        public ProducerValidationResult validate(
                String schemaId,
                String method,
                String path,
                Map<String, String> headers,
                String body,
                int statusCode,
                Map<String, String> responseHeaders,
                String responseBody) {
            callCount++;
            lastSchemaId = schemaId;
            lastMethod = method;
            lastPath = path;
            return new ProducerValidationResult(valid, errors, "");
        }

        int getCallCount() {
            return callCount;
        }

        String getLastSchemaId() {
            return lastSchemaId;
        }

        String getLastMethod() {
            return lastMethod;
        }

        String getLastPath() {
            return lastPath;
        }
    }

    @Nested
    @DisplayName("ProducerConfig Builder Tests")
    class ProducerConfigTest {

        @Test
        @DisplayName("should create config with valid settings")
        void createValidConfig() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .mode(ValidationMode.STRICT)
                    .build();

            assertNotNull(config);
            assertEquals("test-schema", config.getSchemaId());
            assertEquals(ValidationMode.STRICT, config.getMode());
            assertTrue(config.isValidateRequest());
            assertTrue(config.isValidateResponse());
        }

        @Test
        @DisplayName("should throw exception when schemaId is missing")
        void throwWhenSchemaIdMissing() {
            assertThrows(IllegalArgumentException.class, () -> {
                ProducerConfig.builder()
                        .validator(new MockValidator(true))
                        .build();
            });
        }

        @Test
        @DisplayName("should throw exception when schemaId is empty")
        void throwWhenSchemaIdEmpty() {
            assertThrows(IllegalArgumentException.class, () -> {
                ProducerConfig.builder()
                        .schemaId("")
                        .validator(new MockValidator(true))
                        .build();
            });
        }

        @Test
        @DisplayName("should throw exception when validator is missing")
        void throwWhenValidatorMissing() {
            assertThrows(IllegalArgumentException.class, () -> {
                ProducerConfig.builder()
                        .schemaId("test-schema")
                        .build();
            });
        }

        @Test
        @DisplayName("should apply default values")
        void applyDefaultValues() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .build();

            assertEquals(ValidationMode.STRICT, config.getMode());
            assertTrue(config.isValidateRequest());
            assertTrue(config.isValidateResponse());
            assertEquals("CVT", config.getLogPrefix());
        }

        @Test
        @DisplayName("should use custom mode")
        void useCustomMode() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .mode(ValidationMode.WARN)
                    .build();

            assertEquals(ValidationMode.WARN, config.getMode());
        }
    }

    @Nested
    @DisplayName("Path Filtering Tests")
    class PathFilteringTest {

        @Test
        @DisplayName("should validate all paths by default")
        void validateAllPathsByDefault() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .build();

            assertTrue(config.shouldValidatePath("/api/users"));
            assertTrue(config.shouldValidatePath("/health"));
            assertTrue(config.shouldValidatePath("/any/path"));
        }

        @Test
        @DisplayName("should exclude paths matching excludePath patterns")
        void excludeMatchingPaths() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .excludePath("/health")
                    .excludePath("/metrics")
                    .build();

            assertTrue(config.shouldValidatePath("/api/users"));
            assertFalse(config.shouldValidatePath("/health"));
            assertFalse(config.shouldValidatePath("/metrics"));
        }

        @Test
        @DisplayName("should only validate paths matching includePath patterns")
        void includeMatchingPathsOnly() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .includePath("/api/.*")
                    .build();

            assertTrue(config.shouldValidatePath("/api/users"));
            assertFalse(config.shouldValidatePath("/health"));
        }

        @Test
        @DisplayName("should give precedence to exclude over include")
        void excludeTakesPrecedence() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .includePath("/api/.*")
                    .excludePath("/api/internal.*")
                    .build();

            assertTrue(config.shouldValidatePath("/api/users"));
            assertFalse(config.shouldValidatePath("/api/internal/debug"));
        }
    }

    @Nested
    @DisplayName("Request Validation Tests")
    class RequestValidationTest {

        @Test
        @DisplayName("should validate a valid request")
        void validateValidRequest() {
            MockValidator validator = new MockValidator(true);
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(validator)
                    .build();

            Producer producer = new Producer(config);
            Map<String, String> headers = new HashMap<>();
            headers.put("content-type", "application/json");

            ProducerValidationResult result = producer.validateRequest(
                    "POST", "/users", headers, "{\"name\":\"test\"}");

            assertTrue(result.isValid());
            assertEquals("request", result.getType());
            assertEquals(1, validator.getCallCount());
        }

        @Test
        @DisplayName("should return invalid for invalid request")
        void returnInvalidForInvalidRequest() {
            MockValidator validator = new MockValidator(false,
                    Arrays.asList("missing required field: email"));
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(validator)
                    .build();

            Producer producer = new Producer(config);

            ProducerValidationResult result = producer.validateRequest(
                    "POST", "/users", null, "{\"name\":\"test\"}");

            assertFalse(result.isValid());
            assertTrue(result.getErrors().contains("missing required field: email"));
        }

        @Test
        @DisplayName("should skip validation when validateRequest is false")
        void skipValidationWhenDisabled() {
            MockValidator validator = new MockValidator(false);
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(validator)
                    .validateRequest(false)
                    .build();

            Producer producer = new Producer(config);

            ProducerValidationResult result = producer.validateRequest(
                    "POST", "/users", null, "{}");

            assertTrue(result.isValid());
            assertEquals(0, validator.getCallCount());
        }

        @Test
        @DisplayName("should pass correct data to validator")
        void passCorrectDataToValidator() {
            MockValidator validator = new MockValidator(true);
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("my-api")
                    .validator(validator)
                    .build();

            Producer producer = new Producer(config);

            producer.validateRequest("POST", "/users?page=1", null, null);

            assertEquals("my-api", validator.getLastSchemaId());
            assertEquals("POST", validator.getLastMethod());
            assertEquals("/users?page=1", validator.getLastPath());
        }
    }

    @Nested
    @DisplayName("Response Validation Tests")
    class ResponseValidationTest {

        @Test
        @DisplayName("should validate a valid response")
        void validateValidResponse() {
            MockValidator validator = new MockValidator(true);
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(validator)
                    .build();

            Producer producer = new Producer(config);

            ProducerValidationResult result = producer.validateResponse(
                    "GET", "/users/123", null, null,
                    200, null, "{\"id\":123}");

            assertTrue(result.isValid());
            assertEquals("response", result.getType());
        }

        @Test
        @DisplayName("should return invalid for invalid response")
        void returnInvalidForInvalidResponse() {
            MockValidator validator = new MockValidator(false,
                    Arrays.asList("response body type mismatch"));
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(validator)
                    .build();

            Producer producer = new Producer(config);

            ProducerValidationResult result = producer.validateResponse(
                    "GET", "/users/123", null, null,
                    200, null, "{\"id\":\"not-a-number\"}");

            assertFalse(result.isValid());
            assertTrue(result.getErrors().contains("response body type mismatch"));
        }

        @Test
        @DisplayName("should skip validation when validateResponse is false")
        void skipValidationWhenDisabled() {
            MockValidator validator = new MockValidator(false);
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(validator)
                    .validateResponse(false)
                    .build();

            Producer producer = new Producer(config);

            ProducerValidationResult result = producer.validateResponse(
                    "GET", "/users/123", null, null,
                    200, null, "{}");

            assertTrue(result.isValid());
            assertEquals(0, validator.getCallCount());
        }
    }

    @Nested
    @DisplayName("Request Failure Handling Tests")
    class RequestFailureHandlingTest {

        private Object mockRequest = new Object();

        @Test
        @DisplayName("should reject request in strict mode")
        void rejectInStrictMode() {
            Producer.resetMetrics();
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .mode(ValidationMode.STRICT)
                    .build();

            Producer producer = new Producer(config);
            ProducerValidationResult result = new ProducerValidationResult(
                    false, Arrays.asList("test error"), "request");

            Object[] handleResult = producer.handleRequestFailure(result, mockRequest);

            assertFalse((Boolean) handleResult[0]); // Should not continue
            assertNull(handleResult[1]); // No custom response

            Producer.Metrics metrics = Producer.getMetrics();
            assertEquals(1, metrics.getRequestsRejected());
        }

        @Test
        @DisplayName("should continue in warn mode")
        void continueInWarnMode() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .mode(ValidationMode.WARN)
                    .build();

            Producer producer = new Producer(config);
            ProducerValidationResult result = new ProducerValidationResult(
                    false, Arrays.asList("test error"), "request");

            Object[] handleResult = producer.handleRequestFailure(result, mockRequest);

            assertTrue((Boolean) handleResult[0]); // Should continue
        }

        @Test
        @DisplayName("should continue in shadow mode")
        void continueInShadowMode() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .mode(ValidationMode.SHADOW)
                    .build();

            Producer producer = new Producer(config);
            ProducerValidationResult result = new ProducerValidationResult(
                    false, Arrays.asList("test error"), "request");

            Object[] handleResult = producer.handleRequestFailure(result, mockRequest);

            assertTrue((Boolean) handleResult[0]); // Should continue
        }

        @Test
        @DisplayName("should call custom onRequestFailure handler")
        void callCustomHandler() {
            AtomicBoolean handlerCalled = new AtomicBoolean(false);
            AtomicReference<ProducerValidationResult> capturedResult = new AtomicReference<>();

            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .mode(ValidationMode.STRICT)
                    .onRequestFailure((res, req) -> {
                        handlerCalled.set(true);
                        capturedResult.set(res);
                        return Map.of("custom", "response");
                    })
                    .build();

            Producer producer = new Producer(config);
            ProducerValidationResult result = new ProducerValidationResult(
                    false, Arrays.asList("test error"), "request");

            Object[] handleResult = producer.handleRequestFailure(result, mockRequest);

            assertTrue(handlerCalled.get());
            assertNotNull(capturedResult.get());
            assertFalse((Boolean) handleResult[0]); // Should not continue
            assertNotNull(handleResult[1]); // Custom response
        }
    }

    @Nested
    @DisplayName("Response Failure Handling Tests")
    class ResponseFailureHandlingTest {

        private Object mockRequest = new Object();
        private Object mockResponse = new Object();

        @Test
        @DisplayName("should handle response failure in strict mode")
        void handleInStrictMode() {
            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .mode(ValidationMode.STRICT)
                    .build();

            Producer producer = new Producer(config);
            ProducerValidationResult result = new ProducerValidationResult(
                    false, Arrays.asList("response error"), "response");

            // Should not throw
            assertDoesNotThrow(() ->
                producer.handleResponseFailure(result, mockRequest, mockResponse));
        }

        @Test
        @DisplayName("should call custom onResponseFailure handler")
        void callCustomHandler() {
            AtomicBoolean handlerCalled = new AtomicBoolean(false);

            ProducerConfig config = ProducerConfig.builder()
                    .schemaId("test-schema")
                    .validator(new MockValidator(true))
                    .mode(ValidationMode.STRICT)
                    .onResponseFailure(res -> handlerCalled.set(true))
                    .build();

            Producer producer = new Producer(config);
            ProducerValidationResult result = new ProducerValidationResult(
                    false, Arrays.asList("response error"), "response");

            producer.handleResponseFailure(result, mockRequest, mockResponse);

            assertTrue(handlerCalled.get());
        }
    }

    @Nested
    @DisplayName("Metrics Tests")
    class MetricsTest {

        @BeforeEach
        void resetMetrics() {
            Producer.resetMetrics();
        }

        @Test
        @DisplayName("should record request validations")
        void recordRequestValidations() {
            ProducerValidationResult passResult = ProducerValidationResult.success("request");
            ProducerValidationResult failResult = new ProducerValidationResult(
                    false, Arrays.asList("error"), "request");

            Producer.recordValidationMetrics("request", passResult);
            Producer.recordValidationMetrics("request", failResult);

            Producer.Metrics metrics = Producer.getMetrics();
            assertEquals(2, metrics.getRequestValidations());
            assertEquals(1, metrics.getRequestValidationsPassed());
            assertEquals(1, metrics.getRequestValidationsFailed());
        }

        @Test
        @DisplayName("should record response validations")
        void recordResponseValidations() {
            ProducerValidationResult passResult = ProducerValidationResult.success("response");
            ProducerValidationResult failResult = new ProducerValidationResult(
                    false, Arrays.asList("error"), "response");

            Producer.recordValidationMetrics("response", passResult);
            Producer.recordValidationMetrics("response", passResult);
            Producer.recordValidationMetrics("response", failResult);

            Producer.Metrics metrics = Producer.getMetrics();
            assertEquals(3, metrics.getResponseValidations());
            assertEquals(2, metrics.getResponseValidationsPassed());
            assertEquals(1, metrics.getResponseValidationsFailed());
        }

        @Test
        @DisplayName("should record rejections")
        void recordRejections() {
            Producer.recordRejection();
            Producer.recordRejection();

            Producer.Metrics metrics = Producer.getMetrics();
            assertEquals(2, metrics.getRequestsRejected());
        }

        @Test
        @DisplayName("should reset all metrics")
        void resetAllMetrics() {
            Producer.recordValidationMetrics("request",
                    ProducerValidationResult.success("request"));
            Producer.recordValidationMetrics("response",
                    ProducerValidationResult.success("response"));
            Producer.recordRejection();

            Producer.resetMetrics();

            Producer.Metrics metrics = Producer.getMetrics();
            assertEquals(0, metrics.getRequestValidations());
            assertEquals(0, metrics.getResponseValidations());
            assertEquals(0, metrics.getRequestsRejected());
        }
    }

    @Nested
    @DisplayName("ProducerValidationResult Tests")
    class ProducerValidationResultTest {

        @Test
        @DisplayName("should create success result")
        void createSuccessResult() {
            ProducerValidationResult result = ProducerValidationResult.success("request");

            assertTrue(result.isValid());
            assertTrue(result.getErrors().isEmpty());
            assertEquals("request", result.getType());
        }

        @Test
        @DisplayName("should create failure result")
        void createFailureResult() {
            List<String> errors = Arrays.asList("error1", "error2");
            ProducerValidationResult result = ProducerValidationResult.failure(errors, "response");

            assertFalse(result.isValid());
            assertEquals(2, result.getErrors().size());
            assertEquals("response", result.getType());
        }
    }
}
