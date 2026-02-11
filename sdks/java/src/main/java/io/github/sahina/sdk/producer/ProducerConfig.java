package io.github.sahina.sdk.producer;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.function.BiFunction;
import java.util.function.Consumer;
import java.util.regex.Pattern;

/**
 * Configuration for producer-side validation middleware.
 * Use the builder pattern to create instances.
 *
 * <p>Example:
 * <pre>{@code
 * ProducerConfig config = ProducerConfig.builder()
 *         .schemaId("my-api")
 *         .validator(myValidator)
 *         .mode(ValidationMode.STRICT)
 *         .validateRequest(true)
 *         .validateResponse(true)
 *         .excludePath("/health")
 *         .build();
 * }</pre>
 */
public class ProducerConfig {
    private final String schemaId;
    private final Validator validator;
    private final ValidationMode mode;
    private final boolean validateRequest;
    private final boolean validateResponse;
    private final List<Pattern> includePatterns;
    private final List<Pattern> excludePatterns;
    private final BiFunction<ProducerValidationResult, Object, Object> onRequestFailure;
    private final Consumer<ProducerValidationResult> onResponseFailure;
    private final String logPrefix;

    private ProducerConfig(Builder builder) {
        if (builder.schemaId == null || builder.schemaId.isEmpty()) {
            throw new IllegalArgumentException("schemaId is required");
        }
        if (builder.validator == null) {
            throw new IllegalArgumentException("validator is required");
        }

        this.schemaId = builder.schemaId;
        this.validator = builder.validator;
        this.mode = builder.mode;
        this.validateRequest = builder.validateRequest;
        this.validateResponse = builder.validateResponse;
        this.includePatterns = Collections.unmodifiableList(new ArrayList<>(builder.includePatterns));
        this.excludePatterns = Collections.unmodifiableList(new ArrayList<>(builder.excludePatterns));
        this.onRequestFailure = builder.onRequestFailure;
        this.onResponseFailure = builder.onResponseFailure;
        this.logPrefix = builder.logPrefix;
    }

    /**
     * @return The schema ID to validate against
     */
    public String getSchemaId() {
        return schemaId;
    }

    /**
     * @return The validator to use for validation
     */
    public Validator getValidator() {
        return validator;
    }

    /**
     * @return The validation mode
     */
    public ValidationMode getMode() {
        return mode;
    }

    /**
     * @return Whether to validate incoming requests
     */
    public boolean isValidateRequest() {
        return validateRequest;
    }

    /**
     * @return Whether to validate outgoing responses
     */
    public boolean isValidateResponse() {
        return validateResponse;
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
     * @return Callback for request validation failures
     */
    public BiFunction<ProducerValidationResult, Object, Object> getOnRequestFailure() {
        return onRequestFailure;
    }

    /**
     * @return Callback for response validation failures
     */
    public Consumer<ProducerValidationResult> getOnResponseFailure() {
        return onResponseFailure;
    }

    /**
     * @return Prefix for log messages
     */
    public String getLogPrefix() {
        return logPrefix;
    }

    /**
     * Checks if a path should be validated based on include/exclude patterns.
     *
     * @param path The path to check
     * @return true if the path should be validated
     */
    public boolean shouldValidatePath(String path) {
        // Check excludes first
        for (Pattern pattern : excludePatterns) {
            if (pattern.matcher(path).find()) {
                return false;
            }
        }

        // If no includes specified, include all
        if (includePatterns.isEmpty()) {
            return true;
        }

        // Check includes
        for (Pattern pattern : includePatterns) {
            if (pattern.matcher(path).find()) {
                return true;
            }
        }

        return false;
    }

    /**
     * Creates a new builder for ProducerConfig.
     *
     * @return A new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for ProducerConfig.
     */
    public static class Builder {
        private String schemaId;
        private Validator validator;
        private ValidationMode mode = ValidationMode.STRICT;
        private boolean validateRequest = true;
        private boolean validateResponse = true;
        private final List<Pattern> includePatterns = new ArrayList<>();
        private final List<Pattern> excludePatterns = new ArrayList<>();
        private BiFunction<ProducerValidationResult, Object, Object> onRequestFailure;
        private Consumer<ProducerValidationResult> onResponseFailure;
        private String logPrefix = "CVT";

        /**
         * Sets the schema ID to validate against.
         *
         * @param schemaId The schema identifier
         * @return This builder
         */
        public Builder schemaId(String schemaId) {
            this.schemaId = schemaId;
            return this;
        }

        /**
         * Sets the validator to use.
         *
         * @param validator The validator instance
         * @return This builder
         */
        public Builder validator(Validator validator) {
            this.validator = validator;
            return this;
        }

        /**
         * Sets the validation mode.
         *
         * @param mode The validation mode
         * @return This builder
         */
        public Builder mode(ValidationMode mode) {
            this.mode = mode;
            return this;
        }

        /**
         * Sets whether to validate incoming requests.
         *
         * @param validateRequest True to validate requests
         * @return This builder
         */
        public Builder validateRequest(boolean validateRequest) {
            this.validateRequest = validateRequest;
            return this;
        }

        /**
         * Sets whether to validate outgoing responses.
         *
         * @param validateResponse True to validate responses
         * @return This builder
         */
        public Builder validateResponse(boolean validateResponse) {
            this.validateResponse = validateResponse;
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
         * Sets the callback for request validation failures.
         *
         * @param handler Function that receives result and request, returns custom response or null
         * @return This builder
         */
        public Builder onRequestFailure(BiFunction<ProducerValidationResult, Object, Object> handler) {
            this.onRequestFailure = handler;
            return this;
        }

        /**
         * Sets the callback for response validation failures.
         *
         * @param handler Consumer that receives the validation result
         * @return This builder
         */
        public Builder onResponseFailure(Consumer<ProducerValidationResult> handler) {
            this.onResponseFailure = handler;
            return this;
        }

        /**
         * Sets the log prefix.
         *
         * @param logPrefix Prefix for log messages
         * @return This builder
         */
        public Builder logPrefix(String logPrefix) {
            this.logPrefix = logPrefix;
            return this;
        }

        /**
         * Builds a new ProducerConfig instance.
         *
         * @return A new ProducerConfig
         * @throws IllegalArgumentException if schemaId or validator is not set
         */
        public ProducerConfig build() {
            return new ProducerConfig(this);
        }
    }
}
