package io.github.sahina.sdk.adapters;

import io.github.sahina.sdk.ContractValidator;
import io.github.sahina.sdk.ValidationRequest;
import io.github.sahina.sdk.ValidationResponse;
import io.github.sahina.sdk.ValidationResult;

import okhttp3.Headers;
import okhttp3.Interceptor;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import okhttp3.ResponseBody;
import okio.Buffer;

import java.io.IOException;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * OkHttp interceptor that captures HTTP interactions and validates them against
 * an OpenAPI schema using the CVT validator.
 *
 * <p>Example usage:
 * <pre>{@code
 * ContractValidator validator = ContractValidator.builder()
 *     .address("localhost:50051")
 *     .build();
 * validator.registerSchema("my-api", "./openapi.json");
 *
 * OkHttpContractAdapter adapter = new OkHttpContractAdapter(validator)
 *     .withConfig(AdapterConfig.builder()
 *         .autoValidate(true)
 *         .excludePath("/health.*")
 *         .build());
 *
 * OkHttpClient client = new OkHttpClient.Builder()
 *     .addInterceptor(adapter)
 *     .build();
 * }</pre>
 */
public class OkHttpContractAdapter implements Interceptor {
    private final ContractValidator validator;
    private AdapterConfig config;
    private final List<CapturedInteraction> interactions;

    /**
     * Creates a new OkHttpContractAdapter with default configuration.
     *
     * @param validator The CVT validator to use for validation
     */
    public OkHttpContractAdapter(ContractValidator validator) {
        this.validator = validator;
        this.config = AdapterConfig.builder().build();
        this.interactions = new ArrayList<>();
    }

    /**
     * Sets the adapter configuration.
     *
     * @param config The configuration to use
     * @return This adapter for chaining
     */
    public OkHttpContractAdapter withConfig(AdapterConfig config) {
        this.config = config;
        return this;
    }

    /**
     * Attaches this adapter to an OkHttpClient builder.
     *
     * @param builder The client builder to attach to
     * @return The builder for chaining
     */
    public OkHttpClient.Builder attach(OkHttpClient.Builder builder) {
        return builder.addInterceptor(this);
    }

    @Override
    public Response intercept(Chain chain) throws IOException {
        Request request = chain.request();
        String path = request.url().encodedPath();

        // Check if we should process this path
        if (!config.shouldProcess(path)) {
            return chain.proceed(request);
        }

        // Extract request data
        String method = request.method();
        Map<String, String> requestHeaders = headersToMap(request.headers());
        String requestBody = extractRequestBody(request);

        // Execute the request
        Response response = chain.proceed(request);

        // Extract response data
        int statusCode = response.code();
        Map<String, String> responseHeaders = headersToMap(response.headers());
        String responseBody = extractResponseBody(response);

        // Rebuild response with the body we consumed
        response = rebuildResponse(response, responseBody);

        // Validate if enabled
        ValidationResult validationResult = null;
        if (config.isAutoValidate()) {
            try {
                ValidationRequest.Builder valRequestBuilder = ValidationRequest.builder()
                        .method(method)
                        .path(path);
                if (requestHeaders != null) {
                    valRequestBuilder.headers(requestHeaders);
                }
                if (requestBody != null) {
                    valRequestBuilder.body(requestBody);
                }

                ValidationResponse.Builder valResponseBuilder = ValidationResponse.builder()
                        .statusCode(statusCode);
                if (responseHeaders != null) {
                    valResponseBuilder.headers(responseHeaders);
                }
                if (responseBody != null) {
                    valResponseBuilder.body(responseBody);
                }

                validationResult = validator.validate(valRequestBuilder.build(), valResponseBuilder.build());
            } catch (Exception e) {
                // Validation failed, create error result
                List<String> errors = new ArrayList<>();
                errors.add("Validation error: " + e.getMessage());
                validationResult = new ValidationResult(false, errors);
            }
        }

        // Capture interaction if enabled
        if (config.isCaptureInteractions()) {
            synchronized (interactions) {
                interactions.add(new CapturedInteraction(
                        method,
                        path,
                        requestHeaders,
                        requestBody,
                        statusCode,
                        responseHeaders,
                        responseBody,
                        validationResult));
            }
        }

        return response;
    }

    /**
     * Returns all captured interactions.
     *
     * @return A copy of the captured interactions list
     */
    public List<CapturedInteraction> getInteractions() {
        synchronized (interactions) {
            return new ArrayList<>(interactions);
        }
    }

    /**
     * Clears all captured interactions.
     */
    public void clearInteractions() {
        synchronized (interactions) {
            interactions.clear();
        }
    }

    /**
     * Returns only invalid interactions (those that failed validation).
     *
     * @return A list of invalid interactions
     */
    public List<CapturedInteraction> getInvalidInteractions() {
        synchronized (interactions) {
            List<CapturedInteraction> invalid = new ArrayList<>();
            for (CapturedInteraction interaction : interactions) {
                if (!interaction.isValid()) {
                    invalid.add(interaction);
                }
            }
            return invalid;
        }
    }

    private Map<String, String> headersToMap(Headers headers) {
        Map<String, String> map = new HashMap<>();
        for (String name : headers.names()) {
            map.put(name, headers.get(name));
        }
        return map;
    }

    private String extractRequestBody(Request request) throws IOException {
        RequestBody body = request.body();
        if (body == null) {
            return null;
        }

        Buffer buffer = new Buffer();
        body.writeTo(buffer);
        return buffer.readUtf8();
    }

    private String extractResponseBody(Response response) throws IOException {
        ResponseBody body = response.body();
        if (body == null) {
            return null;
        }

        return body.string();
    }

    private Response rebuildResponse(Response response, String body) {
        if (body == null) {
            return response;
        }

        MediaType contentType = response.body() != null ? response.body().contentType() : null;
        ResponseBody newBody = ResponseBody.create(body, contentType);

        return response.newBuilder()
                .body(newBody)
                .build();
    }
}
