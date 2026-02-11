package io.github.sahina.sdk.adapters;

import io.github.sahina.sdk.GenerateOptions;

import java.util.ArrayList;
import java.util.List;
import java.util.regex.Pattern;

/**
 * Configuration options for the mock interceptor.
 * Use the builder pattern to create instances.
 */
public class MockConfig {
    private final boolean cacheResponses;
    private final GenerateOptions generateOptions;
    private final List<Pattern> includePatterns;
    private final List<Pattern> excludePatterns;

    private MockConfig(Builder builder) {
        this.cacheResponses = builder.cacheResponses;
        this.generateOptions = builder.generateOptions;
        this.includePatterns = new ArrayList<>(builder.includePatterns);
        this.excludePatterns = new ArrayList<>(builder.excludePatterns);
    }

    /**
     * @return Whether to cache generated responses by method+path
     */
    public boolean isCacheResponses() {
        return cacheResponses;
    }

    /**
     * @return Options for response generation
     */
    public GenerateOptions getGenerateOptions() {
        return generateOptions;
    }

    /**
     * @return Patterns for paths to include (empty = include all)
     */
    public List<Pattern> getIncludePatterns() {
        return includePatterns;
    }

    /**
     * @return Patterns for paths to exclude
     */
    public List<Pattern> getExcludePatterns() {
        return excludePatterns;
    }

    /**
     * Checks if a path should be mocked based on include/exclude patterns.
     *
     * @param path The path to check
     * @return true if the path should be mocked
     */
    public boolean shouldMock(String path) {
        // Check excludes first
        for (Pattern pattern : excludePatterns) {
            if (pattern.matcher(path).matches() || pattern.matcher(path).find()) {
                return false;
            }
        }

        // If no includes specified, include all
        if (includePatterns.isEmpty()) {
            return true;
        }

        // Check includes
        for (Pattern pattern : includePatterns) {
            if (pattern.matcher(path).matches() || pattern.matcher(path).find()) {
                return true;
            }
        }

        return false;
    }

    /**
     * Creates a new builder for MockConfig.
     *
     * @return A new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for MockConfig.
     */
    public static class Builder {
        private boolean cacheResponses = false;
        private GenerateOptions generateOptions = null;
        private final List<Pattern> includePatterns = new ArrayList<>();
        private final List<Pattern> excludePatterns = new ArrayList<>();

        /**
         * Enables response caching.
         *
         * @return This builder
         */
        public Builder cache() {
            this.cacheResponses = true;
            return this;
        }

        /**
         * Sets whether to cache generated responses.
         *
         * @param cache True to enable caching
         * @return This builder
         */
        public Builder cache(boolean cache) {
            this.cacheResponses = cache;
            return this;
        }

        /**
         * Sets options for response generation.
         *
         * @param options The generation options
         * @return This builder
         */
        public Builder generateOptions(GenerateOptions options) {
            this.generateOptions = options;
            return this;
        }

        /**
         * Adds a path pattern to include.
         *
         * @param pattern Regex pattern for paths to include
         * @return This builder
         */
        public Builder includePath(String pattern) {
            this.includePatterns.add(Pattern.compile(pattern));
            return this;
        }

        /**
         * Adds a path pattern to exclude.
         *
         * @param pattern Regex pattern for paths to exclude
         * @return This builder
         */
        public Builder excludePath(String pattern) {
            this.excludePatterns.add(Pattern.compile(pattern));
            return this;
        }

        /**
         * Builds a new MockConfig instance.
         *
         * @return A new MockConfig
         */
        public MockConfig build() {
            return new MockConfig(this);
        }
    }
}
