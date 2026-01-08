package com.cvt.sdk.adapters;

import com.cvt.sdk.ValidationResult;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class CapturedInteractionTest {

    @Test
    @DisplayName("Should store interaction data correctly")
    void testInteractionData() {
        Map<String, String> requestHeaders = new HashMap<>();
        requestHeaders.put("Content-Type", "application/json");

        Map<String, String> responseHeaders = new HashMap<>();
        responseHeaders.put("X-Request-Id", "123");

        ValidationResult validationResult = new ValidationResult(true, Collections.emptyList());

        CapturedInteraction interaction = new CapturedInteraction(
                "POST",
                "/users",
                requestHeaders,
                "{\"name\":\"John\"}",
                201,
                responseHeaders,
                "{\"id\":1}",
                validationResult);

        assertEquals("POST", interaction.getMethod());
        assertEquals("/users", interaction.getPath());
        assertEquals("application/json", interaction.getRequestHeaders().get("Content-Type"));
        assertEquals("{\"name\":\"John\"}", interaction.getRequestBody());
        assertEquals(201, interaction.getStatusCode());
        assertEquals("123", interaction.getResponseHeaders().get("X-Request-Id"));
        assertEquals("{\"id\":1}", interaction.getResponseBody());
        assertTrue(interaction.isValid());
        assertTrue(interaction.getTimestamp() > 0);
    }

    @Test
    @DisplayName("Should handle null values")
    void testNullValues() {
        CapturedInteraction interaction = new CapturedInteraction(
                "GET",
                "/users",
                null,
                null,
                200,
                null,
                null,
                null);

        assertEquals("GET", interaction.getMethod());
        assertEquals("/users", interaction.getPath());
        assertNotNull(interaction.getRequestHeaders());
        assertTrue(interaction.getRequestHeaders().isEmpty());
        assertNull(interaction.getRequestBody());
        assertNotNull(interaction.getResponseHeaders());
        assertTrue(interaction.getResponseHeaders().isEmpty());
        assertNull(interaction.getResponseBody());
        assertNull(interaction.getValidationResult());
        assertFalse(interaction.isValid());
    }

    @Test
    @DisplayName("Should report invalid when validation fails")
    void testInvalidInteraction() {
        ValidationResult validationResult = new ValidationResult(false,
                Collections.singletonList("Missing field"));

        CapturedInteraction interaction = new CapturedInteraction(
                "POST",
                "/users",
                null,
                "{}",
                400,
                null,
                "{\"error\":\"Bad request\"}",
                validationResult);

        assertFalse(interaction.isValid());
        assertFalse(interaction.getValidationResult().isValid());
    }

    @Test
    @DisplayName("toString should include key details")
    void testToString() {
        CapturedInteraction interaction = new CapturedInteraction(
                "GET",
                "/users",
                null,
                null,
                200,
                null,
                null,
                new ValidationResult(true, Collections.emptyList()));

        String str = interaction.toString();
        assertTrue(str.contains("GET"));
        assertTrue(str.contains("/users"));
        assertTrue(str.contains("200"));
    }
}
