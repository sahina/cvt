package io.github.sahina.sdk.producer;

import java.util.Collections;
import java.util.List;

/**
 * Validation result from producer testing.
 */
public class TestValidationResult {
    private final boolean valid;
    private final List<String> errors;
    private final String validatedAgainstVersion;
    private final String validatedAgainstHash;

    /**
     * Creates a new TestValidationResult.
     *
     * @param valid                   Whether validation passed
     * @param errors                  Validation error messages
     * @param validatedAgainstVersion Schema version used for validation
     * @param validatedAgainstHash    Schema hash used for validation
     */
    public TestValidationResult(
            boolean valid,
            List<String> errors,
            String validatedAgainstVersion,
            String validatedAgainstHash) {
        this.valid = valid;
        this.errors = errors != null ? errors : Collections.emptyList();
        this.validatedAgainstVersion = validatedAgainstVersion;
        this.validatedAgainstHash = validatedAgainstHash;
    }

    /**
     * Returns whether validation passed.
     *
     * @return true if valid
     */
    public boolean isValid() {
        return valid;
    }

    /**
     * Returns the validation error messages.
     *
     * @return List of error messages (empty if valid)
     */
    public List<String> getErrors() {
        return errors;
    }

    /**
     * Returns the schema version used for validation.
     *
     * @return Schema version
     */
    public String getValidatedAgainstVersion() {
        return validatedAgainstVersion;
    }

    /**
     * Returns the schema hash used for validation.
     *
     * @return Schema hash
     */
    public String getValidatedAgainstHash() {
        return validatedAgainstHash;
    }

    @Override
    public String toString() {
        return "TestValidationResult{" +
                "valid=" + valid +
                ", errors=" + errors +
                ", validatedAgainstVersion='" + validatedAgainstVersion + '\'' +
                ", validatedAgainstHash='" + validatedAgainstHash + '\'' +
                '}';
    }
}
