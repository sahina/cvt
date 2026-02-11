package io.github.sahina.sdk.adapters;

import java.util.ArrayList;
import java.util.List;
import java.util.regex.Pattern;

/**
 * Configuration options for HTTP adapters.
 * Use the builder pattern to create instances.
 */
public class AdapterConfig {
    private final boolean autoValidate;
    private final List<Pattern> includePatterns;
    private final List<Pattern> excludePatterns;
    private final boolean captureInteractions;

    private AdapterConfig(Builder builder) {
        this.autoValidate = builder.autoValidate;
        this.includePatterns = new ArrayList<>(builder.includePatterns);
        this.excludePatterns = new ArrayList<>(builder.excludePatterns);
        this.captureInteractions = builder.captureInteractions;
    }

    /**
     * @return Whether to automatically validate interactions
     */
    public boolean isAutoValidate() {
        return autoValidate;
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
     * @return Whether to capture interactions for later retrieval
     */
    public boolean isCaptureInteractions() {
        return captureInteractions;
    }

    /**
     * Checks if a path should be processed based on include/exclude patterns.
     *
     * @param path The path to check
     * @return true if the path should be processed
     */
    public boolean shouldProcess(String path) {
        // Check excludes first
        for (Pattern pattern : excludePatterns) {
            if (pattern.matcher(path).matches()) {
                return false;
            }
        }

        // If no includes specified, include all
        if (includePatterns.isEmpty()) {
            return true;
        }

        // Check includes
        for (Pattern pattern : includePatterns) {
            if (pattern.matcher(path).matches()) {
                return true;
            }
        }

        return false;
    }

    /**
     * Creates a new builder for AdapterConfig.
     *
     * @return A new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for AdapterConfig.
     */
    public static class Builder {
        private boolean autoValidate = true;
        private final List<Pattern> includePatterns = new ArrayList<>();
        private final List<Pattern> excludePatterns = new ArrayList<>();
        private boolean captureInteractions = true;

        /**
         * Sets whether to automatically validate interactions.
         *
         * @param autoValidate True to enable automatic validation
         * @return This builder
         */
        public Builder autoValidate(boolean autoValidate) {
            this.autoValidate = autoValidate;
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
         * Sets whether to capture interactions for later retrieval.
         *
         * @param captureInteractions True to capture interactions
         * @return This builder
         */
        public Builder captureInteractions(boolean captureInteractions) {
            this.captureInteractions = captureInteractions;
            return this;
        }

        /**
         * Builds a new AdapterConfig instance.
         *
         * @return A new AdapterConfig
         */
        public AdapterConfig build() {
            return new AdapterConfig(this);
        }
    }
}
