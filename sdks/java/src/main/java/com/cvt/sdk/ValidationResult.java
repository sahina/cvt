package com.cvt.sdk;

import java.util.Collections;
import java.util.List;

/**
 * Represents the result of validating an HTTP interaction against an OpenAPI schema.
 */
public class ValidationResult {
    private final boolean valid;
    private final List<String> errors;

    public ValidationResult(boolean valid, List<String> errors) {
        this.valid = valid;
        this.errors = errors != null ? List.copyOf(errors) : Collections.emptyList();
    }

    /**
     * @return true if the interaction is valid according to the schema
     */
    public boolean isValid() {
        return valid;
    }

    /**
     * @return list of validation error messages, empty if valid
     */
    public List<String> getErrors() {
        return errors;
    }

    @Override
    public String toString() {
        if (valid) {
            return "ValidationResult{valid=true}";
        }
        return "ValidationResult{valid=false, errors=" + errors + "}";
    }
}
