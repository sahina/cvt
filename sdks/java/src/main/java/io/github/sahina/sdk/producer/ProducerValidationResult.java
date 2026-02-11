package io.github.sahina.sdk.producer;

import java.util.Collections;
import java.util.List;

/**
 * Result of producer validation.
 */
public class ProducerValidationResult {
    private final boolean valid;
    private final List<String> errors;
    private final String type; // "request" or "response"

    /**
     * Creates a new validation result.
     *
     * @param valid  Whether the validation passed
     * @param errors List of validation errors (empty if valid)
     * @param type   Type of validation ("request" or "response")
     */
    public ProducerValidationResult(boolean valid, List<String> errors, String type) {
        this.valid = valid;
        this.errors = errors != null ? Collections.unmodifiableList(errors) : Collections.emptyList();
        this.type = type;
    }

    /**
     * Creates a successful validation result.
     *
     * @param type Type of validation ("request" or "response")
     * @return A successful validation result
     */
    public static ProducerValidationResult success(String type) {
        return new ProducerValidationResult(true, Collections.emptyList(), type);
    }

    /**
     * Creates a failed validation result.
     *
     * @param errors List of validation errors
     * @param type   Type of validation ("request" or "response")
     * @return A failed validation result
     */
    public static ProducerValidationResult failure(List<String> errors, String type) {
        return new ProducerValidationResult(false, errors, type);
    }

    /**
     * @return Whether the validation passed
     */
    public boolean isValid() {
        return valid;
    }

    /**
     * @return List of validation errors (empty if valid)
     */
    public List<String> getErrors() {
        return errors;
    }

    /**
     * @return Type of validation ("request" or "response")
     */
    public String getType() {
        return type;
    }
}
