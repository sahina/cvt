package com.cvt.sdk.adapters;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class AdapterConfigTest {

    @Test
    @DisplayName("Should have correct default values")
    void testDefaults() {
        AdapterConfig config = AdapterConfig.builder().build();

        assertTrue(config.isAutoValidate());
        assertTrue(config.isCaptureInteractions());
        assertTrue(config.getIncludePatterns().isEmpty());
        assertTrue(config.getExcludePatterns().isEmpty());
    }

    @Test
    @DisplayName("Should accept custom values")
    void testCustomValues() {
        AdapterConfig config = AdapterConfig.builder()
                .autoValidate(false)
                .captureInteractions(false)
                .includePath("/api/.*")
                .excludePath("/health.*")
                .build();

        assertFalse(config.isAutoValidate());
        assertFalse(config.isCaptureInteractions());
        assertEquals(1, config.getIncludePatterns().size());
        assertEquals(1, config.getExcludePatterns().size());
    }

    @Test
    @DisplayName("Should process all paths when no patterns specified")
    void testShouldProcessNoPatterns() {
        AdapterConfig config = AdapterConfig.builder().build();

        assertTrue(config.shouldProcess("/users"));
        assertTrue(config.shouldProcess("/api/v1/users"));
        assertTrue(config.shouldProcess("/health"));
    }

    @Test
    @DisplayName("Should exclude paths matching exclude patterns")
    void testShouldProcessExclude() {
        AdapterConfig config = AdapterConfig.builder()
                .excludePath("/health.*")
                .excludePath("/metrics")
                .build();

        assertTrue(config.shouldProcess("/users"));
        assertFalse(config.shouldProcess("/health"));
        assertFalse(config.shouldProcess("/health/live"));
        assertFalse(config.shouldProcess("/metrics"));
    }

    @Test
    @DisplayName("Should only include paths matching include patterns")
    void testShouldProcessInclude() {
        AdapterConfig config = AdapterConfig.builder()
                .includePath("/api/.*")
                .build();

        assertTrue(config.shouldProcess("/api/users"));
        assertTrue(config.shouldProcess("/api/v1/products"));
        assertFalse(config.shouldProcess("/users"));
        assertFalse(config.shouldProcess("/health"));
    }

    @Test
    @DisplayName("Exclude patterns should take precedence over include patterns")
    void testExcludeTakesPrecedence() {
        AdapterConfig config = AdapterConfig.builder()
                .includePath("/api/.*")
                .excludePath("/api/internal.*")
                .build();

        assertTrue(config.shouldProcess("/api/users"));
        assertFalse(config.shouldProcess("/api/internal/admin"));
        assertFalse(config.shouldProcess("/users"));
    }
}
