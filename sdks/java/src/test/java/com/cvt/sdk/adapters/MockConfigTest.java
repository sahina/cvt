package com.cvt.sdk.adapters;

import com.cvt.sdk.GenerateOptions;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class MockConfigTest {

    @Nested
    @DisplayName("Builder defaults")
    class BuilderDefaults {

        @Test
        @DisplayName("Should have cache disabled by default")
        void testCacheDisabledByDefault() {
            MockConfig config = MockConfig.builder().build();
            assertFalse(config.isCacheResponses());
        }

        @Test
        @DisplayName("Should have null generateOptions by default")
        void testNullGenerateOptionsByDefault() {
            MockConfig config = MockConfig.builder().build();
            assertNull(config.getGenerateOptions());
        }

        @Test
        @DisplayName("Should have empty path patterns by default")
        void testEmptyPathPatternsByDefault() {
            MockConfig config = MockConfig.builder().build();
            assertTrue(config.getIncludePatterns().isEmpty());
            assertTrue(config.getExcludePatterns().isEmpty());
        }
    }

    @Nested
    @DisplayName("Builder configuration")
    class BuilderConfiguration {

        @Test
        @DisplayName("Should enable cache")
        void testEnableCache() {
            MockConfig config = MockConfig.builder().cache().build();
            assertTrue(config.isCacheResponses());
        }

        @Test
        @DisplayName("Should set cache with boolean")
        void testSetCacheBoolean() {
            MockConfig configTrue = MockConfig.builder().cache(true).build();
            MockConfig configFalse = MockConfig.builder().cache(false).build();

            assertTrue(configTrue.isCacheResponses());
            assertFalse(configFalse.isCacheResponses());
        }

        @Test
        @DisplayName("Should set generateOptions")
        void testSetGenerateOptions() {
            GenerateOptions options = GenerateOptions.builder()
                    .statusCode(201)
                    .useExamples(true)
                    .build();

            MockConfig config = MockConfig.builder()
                    .generateOptions(options)
                    .build();

            assertNotNull(config.getGenerateOptions());
            assertEquals(201, config.getGenerateOptions().getStatusCode());
        }

        @Test
        @DisplayName("Should add include path")
        void testAddIncludePath() {
            MockConfig config = MockConfig.builder()
                    .includePath("/api/.*")
                    .build();

            assertEquals(1, config.getIncludePatterns().size());
        }

        @Test
        @DisplayName("Should add exclude path")
        void testAddExcludePath() {
            MockConfig config = MockConfig.builder()
                    .excludePath("/health.*")
                    .build();

            assertEquals(1, config.getExcludePatterns().size());
        }

        @Test
        @DisplayName("Should add multiple patterns")
        void testAddMultiplePatterns() {
            MockConfig config = MockConfig.builder()
                    .includePath("/api/.*")
                    .includePath("/v1/.*")
                    .excludePath("/health.*")
                    .excludePath("/metrics.*")
                    .build();

            assertEquals(2, config.getIncludePatterns().size());
            assertEquals(2, config.getExcludePatterns().size());
        }
    }

    @Nested
    @DisplayName("Path filtering")
    class PathFiltering {

        @Test
        @DisplayName("Should mock all paths when no filters")
        void testMockAllPathsWithNoFilters() {
            MockConfig config = MockConfig.builder().build();

            assertTrue(config.shouldMock("/users"));
            assertTrue(config.shouldMock("/api/users/123"));
            assertTrue(config.shouldMock("/health"));
        }

        @Test
        @DisplayName("Should exclude paths matching exclude pattern")
        void testExcludePath() {
            MockConfig config = MockConfig.builder()
                    .excludePath("/health.*")
                    .build();

            assertTrue(config.shouldMock("/users"));
            assertFalse(config.shouldMock("/health"));
            assertFalse(config.shouldMock("/health/check"));
        }

        @Test
        @DisplayName("Should only include paths matching include pattern")
        void testIncludePath() {
            MockConfig config = MockConfig.builder()
                    .includePath("/api/.*")
                    .build();

            assertTrue(config.shouldMock("/api/users"));
            assertTrue(config.shouldMock("/api/users/123"));
            assertFalse(config.shouldMock("/users"));
            assertFalse(config.shouldMock("/health"));
        }

        @Test
        @DisplayName("Should apply exclude before include")
        void testExcludeBeforeInclude() {
            MockConfig config = MockConfig.builder()
                    .includePath("/api/.*")
                    .excludePath("/api/health.*")
                    .build();

            assertTrue(config.shouldMock("/api/users"));
            assertFalse(config.shouldMock("/api/health"));
            assertFalse(config.shouldMock("/users"));
        }
    }
}
