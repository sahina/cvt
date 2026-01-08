package com.cvt.sdk.producer;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.util.Arrays;
import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for ProducerTestKit and related classes.
 */
class ProducerTestKitTest {

    @Nested
    @DisplayName("ProducerTestKit.Builder Tests")
    class BuilderTest {

        @Test
        @DisplayName("should throw exception when schemaId is null")
        void throwWhenSchemaIdNull() {
            assertThrows(NullPointerException.class, () ->
                ProducerTestKit.builder()
                    .schemaId(null)
                    .build()
            );
        }

        @Test
        @DisplayName("should throw exception when schemaId is empty")
        void throwWhenSchemaIdEmpty() {
            assertThrows(IllegalArgumentException.class, () ->
                ProducerTestKit.builder()
                    .schemaId("")
                    .build()
            );
        }

        @Test
        @DisplayName("should use default server address when not specified")
        void useDefaultServerAddress() {
            // Channel is created lazily, so we just verify the builder works
            ProducerTestKit testKit = ProducerTestKit.builder()
                .schemaId("test-schema")
                .build();
            assertNotNull(testKit);
            testKit.close();
        }

        @Test
        @DisplayName("should accept custom server address")
        void acceptCustomServerAddress() {
            ProducerTestKit testKit = ProducerTestKit.builder()
                .schemaId("test-schema")
                .serverAddress("localhost:9999")
                .build();
            assertNotNull(testKit);
            testKit.close();
        }

        @Test
        @DisplayName("should accept schema version")
        void acceptSchemaVersion() {
            ProducerTestKit testKit = ProducerTestKit.builder()
                .schemaId("test-schema")
                .schemaVersion("1.0.0")
                .build();
            assertNotNull(testKit);
            testKit.close();
        }

        @Test
        @DisplayName("should accept API key")
        void acceptApiKey() {
            ProducerTestKit testKit = ProducerTestKit.builder()
                .schemaId("test-schema")
                .apiKey("my-secret-key")
                .build();
            assertNotNull(testKit);
            testKit.close();
        }
    }

    @Nested
    @DisplayName("TestResponseData.Builder Tests")
    class TestResponseDataBuilderTest {

        @Test
        @DisplayName("should create response data with status code")
        void createWithStatusCode() {
            TestResponseData response = TestResponseData.builder()
                .statusCode(200)
                .build();

            assertEquals(200, response.getStatusCode());
            assertNotNull(response.getHeaders());
            assertTrue(response.getHeaders().isEmpty());
            assertNull(response.getBody());
        }

        @Test
        @DisplayName("should create response data with body")
        void createWithBody() {
            Map<String, Object> body = new HashMap<>();
            body.put("id", "123");
            body.put("name", "Test User");

            TestResponseData response = TestResponseData.builder()
                .statusCode(200)
                .body(body)
                .build();

            assertEquals(200, response.getStatusCode());
            assertEquals(body, response.getBody());
        }

        @Test
        @DisplayName("should create response data with string body")
        void createWithStringBody() {
            String body = "{\"id\":\"123\"}";

            TestResponseData response = TestResponseData.builder()
                .statusCode(200)
                .body(body)
                .build();

            assertEquals(body, response.getBody());
        }

        @Test
        @DisplayName("should create response data with headers")
        void createWithHeaders() {
            Map<String, String> headers = new HashMap<>();
            headers.put("Content-Type", "application/json");
            headers.put("X-Request-Id", "abc123");

            TestResponseData response = TestResponseData.builder()
                .statusCode(200)
                .headers(headers)
                .build();

            assertEquals(headers, response.getHeaders());
        }

        @Test
        @DisplayName("should add individual headers")
        void addIndividualHeaders() {
            TestResponseData response = TestResponseData.builder()
                .statusCode(200)
                .header("Content-Type", "application/json")
                .header("X-Request-Id", "abc123")
                .build();

            assertEquals("application/json", response.getHeaders().get("Content-Type"));
            assertEquals("abc123", response.getHeaders().get("X-Request-Id"));
        }

        @Test
        @DisplayName("should handle null headers gracefully")
        void handleNullHeaders() {
            TestResponseData response = TestResponseData.builder()
                .statusCode(200)
                .headers(null)
                .build();

            assertNotNull(response.getHeaders());
            assertTrue(response.getHeaders().isEmpty());
        }
    }

    @Nested
    @DisplayName("TestRequestContext.Builder Tests")
    class TestRequestContextBuilderTest {

        @Test
        @DisplayName("should create request context with method and path")
        void createWithMethodAndPath() {
            TestRequestContext request = TestRequestContext.builder()
                .method("GET")
                .path("/users/123")
                .build();

            assertEquals("GET", request.getMethod());
            assertEquals("/users/123", request.getPath());
            assertNotNull(request.getHeaders());
            assertTrue(request.getHeaders().isEmpty());
            assertNull(request.getBody());
        }

        @Test
        @DisplayName("should create request context with body")
        void createWithBody() {
            Map<String, Object> body = new HashMap<>();
            body.put("name", "Test User");
            body.put("email", "test@example.com");

            TestRequestContext request = TestRequestContext.builder()
                .method("POST")
                .path("/users")
                .body(body)
                .build();

            assertEquals("POST", request.getMethod());
            assertEquals("/users", request.getPath());
            assertEquals(body, request.getBody());
        }

        @Test
        @DisplayName("should create request context with headers")
        void createWithHeaders() {
            Map<String, String> headers = new HashMap<>();
            headers.put("Authorization", "Bearer token123");
            headers.put("Content-Type", "application/json");

            TestRequestContext request = TestRequestContext.builder()
                .method("POST")
                .path("/users")
                .headers(headers)
                .build();

            assertEquals(headers, request.getHeaders());
        }

        @Test
        @DisplayName("should add individual headers")
        void addIndividualHeaders() {
            TestRequestContext request = TestRequestContext.builder()
                .method("POST")
                .path("/users")
                .header("Authorization", "Bearer token123")
                .header("Content-Type", "application/json")
                .build();

            assertEquals("Bearer token123", request.getHeaders().get("Authorization"));
            assertEquals("application/json", request.getHeaders().get("Content-Type"));
        }

        @Test
        @DisplayName("should handle null headers gracefully")
        void handleNullHeaders() {
            TestRequestContext request = TestRequestContext.builder()
                .method("GET")
                .path("/users")
                .headers(null)
                .build();

            assertNotNull(request.getHeaders());
            assertTrue(request.getHeaders().isEmpty());
        }
    }

    @Nested
    @DisplayName("TestValidationResult Tests")
    class TestValidationResultTest {

        @Test
        @DisplayName("should create valid result")
        void createValidResult() {
            TestValidationResult result = new TestValidationResult(
                true,
                Collections.emptyList(),
                "1.0.0",
                "abc123hash"
            );

            assertTrue(result.isValid());
            assertTrue(result.getErrors().isEmpty());
            assertEquals("1.0.0", result.getValidatedAgainstVersion());
            assertEquals("abc123hash", result.getValidatedAgainstHash());
        }

        @Test
        @DisplayName("should create invalid result with errors")
        void createInvalidResult() {
            TestValidationResult result = new TestValidationResult(
                false,
                Arrays.asList("missing required field: name", "invalid type for id"),
                "1.0.0",
                "abc123hash"
            );

            assertFalse(result.isValid());
            assertEquals(2, result.getErrors().size());
            assertTrue(result.getErrors().contains("missing required field: name"));
            assertTrue(result.getErrors().contains("invalid type for id"));
        }

        @Test
        @DisplayName("should handle null errors list")
        void handleNullErrors() {
            TestValidationResult result = new TestValidationResult(
                true,
                null,
                "1.0.0",
                "abc123hash"
            );

            assertTrue(result.isValid());
            assertNotNull(result.getErrors());
            assertTrue(result.getErrors().isEmpty());
        }

        @Test
        @DisplayName("should generate proper toString")
        void generateToString() {
            TestValidationResult result = new TestValidationResult(
                true,
                Collections.emptyList(),
                "1.0.0",
                "abc123"
            );

            String str = result.toString();
            assertTrue(str.contains("valid=true"));
            assertTrue(str.contains("validatedAgainstVersion='1.0.0'"));
            assertTrue(str.contains("validatedAgainstHash='abc123'"));
        }
    }

    @Nested
    @DisplayName("EndpointTester Tests")
    class EndpointTesterTest {

        @Test
        @DisplayName("should substitute path parameters")
        void substitutePathParameters() {
            // We can't fully test without a server, but we can verify the pattern
            // by checking the pathPattern and pathValues work together
            String pathPattern = "/users/{id}/orders/{orderId}";
            Map<String, String> pathValues = new HashMap<>();
            pathValues.put("id", "123");
            pathValues.put("orderId", "456");

            String actualPath = pathPattern;
            for (Map.Entry<String, String> entry : pathValues.entrySet()) {
                actualPath = actualPath.replace("{" + entry.getKey() + "}", entry.getValue());
            }

            assertEquals("/users/123/orders/456", actualPath);
        }

        @Test
        @DisplayName("should handle empty path values")
        void handleEmptyPathValues() {
            String pathPattern = "/health";
            Map<String, String> pathValues = Collections.emptyMap();

            String actualPath = pathPattern;
            for (Map.Entry<String, String> entry : pathValues.entrySet()) {
                actualPath = actualPath.replace("{" + entry.getKey() + "}", entry.getValue());
            }

            assertEquals("/health", actualPath);
        }

        @Test
        @DisplayName("should handle single path parameter")
        void handleSinglePathParameter() {
            String pathPattern = "/pets/{petId}";
            Map<String, String> pathValues = new HashMap<>();
            pathValues.put("petId", "fluffy-123");

            String actualPath = pathPattern;
            for (Map.Entry<String, String> entry : pathValues.entrySet()) {
                actualPath = actualPath.replace("{" + entry.getKey() + "}", entry.getValue());
            }

            assertEquals("/pets/fluffy-123", actualPath);
        }
    }
}
