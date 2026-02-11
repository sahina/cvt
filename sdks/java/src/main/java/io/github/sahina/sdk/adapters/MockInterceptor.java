package io.github.sahina.sdk.adapters;

import io.github.sahina.sdk.ContractValidator;
import io.github.sahina.sdk.GenerateOptions;
import io.github.sahina.sdk.GeneratedResponse;
import com.google.gson.Gson;

import okhttp3.Headers;
import okhttp3.Interceptor;
import okhttp3.MediaType;
import okhttp3.OkHttpClient;
import okhttp3.Protocol;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import okhttp3.ResponseBody;
import okio.Buffer;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * OkHttp interceptor that generates mock responses from OpenAPI schema.
 *
 * <p>This is useful for testing consumers against OpenAPI schemas without
 * requiring the producer API to be running.
 *
 * <p>Example usage:
 * <pre>{@code
 * ContractValidator validator = ContractValidator.builder()
 *     .address("localhost:50051")
 *     .build();
 * validator.registerSchema("my-api", "./openapi.json");
 *
 * // Simple: one-liner to get a mock client
 * OkHttpClient mockClient = MockInterceptor.createMockClient(validator);
 *
 * // With options
 * MockInterceptor mock = new MockInterceptor(validator)
 *     .withConfig(MockConfig.builder()
 *         .cache()
 *         .excludePath("/health.*")
 *         .build());
 *
 * OkHttpClient client = new OkHttpClient.Builder()
 *     .addInterceptor(mock)
 *     .build();
 *
 * // Inspect captured interactions
 * List<CapturedInteraction> interactions = mock.getInteractions();
 * }</pre>
 */
public class MockInterceptor implements Interceptor {
    private static final Gson GSON = new Gson();
    private static final Map<Integer, String> STATUS_TEXT = createStatusTextMap();

    private final ContractValidator validator;
    private MockConfig config;
    private final List<CapturedInteraction> interactions;
    private final Map<String, GeneratedResponse> responseCache;

    /**
     * Creates a new MockInterceptor with default configuration.
     *
     * @param validator The CVT validator to use for response generation
     */
    public MockInterceptor(ContractValidator validator) {
        this.validator = validator;
        this.config = MockConfig.builder().build();
        this.interactions = Collections.synchronizedList(new ArrayList<>());
        this.responseCache = new ConcurrentHashMap<>();
    }

    /**
     * Sets the mock configuration.
     *
     * @param config The configuration to use
     * @return This interceptor for chaining
     */
    public MockInterceptor withConfig(MockConfig config) {
        this.config = config;
        return this;
    }

    /**
     * Creates a simple mock OkHttpClient.
     *
     * @param validator The CVT validator
     * @return An OkHttpClient configured with this mock interceptor
     */
    public static OkHttpClient createMockClient(ContractValidator validator) {
        return new OkHttpClient.Builder()
                .addInterceptor(new MockInterceptor(validator))
                .build();
    }

    /**
     * Creates a mock OkHttpClient with custom configuration.
     *
     * @param validator The CVT validator
     * @param config The mock configuration
     * @return An OkHttpClient configured with this mock interceptor
     */
    public static OkHttpClient createMockClient(ContractValidator validator, MockConfig config) {
        return new OkHttpClient.Builder()
                .addInterceptor(new MockInterceptor(validator).withConfig(config))
                .build();
    }

    @Override
    public Response intercept(Chain chain) throws IOException {
        Request request = chain.request();
        String path = request.url().encodedPath();
        String query = request.url().encodedQuery();
        if (query != null && !query.isEmpty()) {
            path = path + "?" + query;
        }

        // Check if we should mock this path
        if (!config.shouldMock(path)) {
            throw new IOException("cvt: path \"" + path + "\" is excluded from mocking");
        }

        // Extract request data
        String method = request.method();
        Map<String, String> requestHeaders = headersToMap(request.headers());
        String requestBody = extractRequestBody(request);

        // Generate or retrieve cached response
        GeneratedResponse generated = getOrGenerateResponse(method, path);

        // Build mock OkHttp Response
        Response response = buildResponse(request, generated);

        // Capture interaction
        synchronized (interactions) {
            interactions.add(new CapturedInteraction(
                    method,
                    path,
                    requestHeaders,
                    requestBody,
                    generated.getStatusCode(),
                    generated.getHeaders(),
                    generated.getBody() != null ? GSON.toJson(generated.getBody()) : null,
                    null)); // No validation result for mock responses
        }

        return response;
    }

    /**
     * Get cached response or generate a new one.
     */
    private GeneratedResponse getOrGenerateResponse(String method, String path) {
        String cacheKey = method + ":" + path;

        if (config.isCacheResponses()) {
            GeneratedResponse cached = responseCache.get(cacheKey);
            if (cached != null) {
                return cached;
            }
        }

        // Generate new response
        GenerateOptions options = config.getGenerateOptions();
        GeneratedResponse generated = validator.generateResponse(method, path, options);

        // Cache if enabled
        if (config.isCacheResponses()) {
            responseCache.put(cacheKey, generated);
        }

        return generated;
    }

    /**
     * Build OkHttp Response from GeneratedResponse.
     */
    private Response buildResponse(Request request, GeneratedResponse generated) {
        String bodyJson = generated.getBody() != null ? GSON.toJson(generated.getBody()) : "";
        MediaType mediaType = MediaType.parse("application/json");

        Map<String, String> genHeaders = generated.getHeaders();
        if (genHeaders != null) {
            String contentType = genHeaders.get("content-type");
            if (contentType == null) {
                contentType = genHeaders.get("Content-Type");
            }
            if (contentType != null) {
                mediaType = MediaType.parse(contentType);
            }
        }

        Headers.Builder headersBuilder = new Headers.Builder();
        if (genHeaders != null) {
            for (Map.Entry<String, String> entry : genHeaders.entrySet()) {
                headersBuilder.add(entry.getKey(), entry.getValue());
            }
        }

        // Ensure content-type is set
        if (genHeaders == null || (!genHeaders.containsKey("content-type") && !genHeaders.containsKey("Content-Type"))) {
            headersBuilder.add("Content-Type", "application/json");
        }

        int statusCode = generated.getStatusCode();
        String statusText = STATUS_TEXT.getOrDefault(statusCode, "");

        return new Response.Builder()
                .request(request)
                .protocol(Protocol.HTTP_1_1)
                .code(statusCode)
                .message(statusText)
                .headers(headersBuilder.build())
                .body(ResponseBody.create(bodyJson, mediaType))
                .build();
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
     * Clears the response cache.
     */
    public void clearCache() {
        responseCache.clear();
    }

    private Map<String, String> headersToMap(Headers headers) {
        Map<String, String> map = new HashMap<>();
        for (String name : headers.names()) {
            map.put(name.toLowerCase(), headers.get(name));
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

    private static Map<Integer, String> createStatusTextMap() {
        Map<Integer, String> map = new HashMap<>();
        map.put(200, "OK");
        map.put(201, "Created");
        map.put(204, "No Content");
        map.put(400, "Bad Request");
        map.put(401, "Unauthorized");
        map.put(403, "Forbidden");
        map.put(404, "Not Found");
        map.put(500, "Internal Server Error");
        return Collections.unmodifiableMap(map);
    }
}
