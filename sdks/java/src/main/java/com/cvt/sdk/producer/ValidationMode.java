package com.cvt.sdk.producer;

/**
 * Validation mode determines how validation failures are handled.
 */
public enum ValidationMode {
    /**
     * Reject invalid requests with 400/422 errors.
     */
    STRICT,

    /**
     * Log violations but allow requests to proceed.
     */
    WARN,

    /**
     * Validate asynchronously, record metrics only.
     */
    SHADOW
}
