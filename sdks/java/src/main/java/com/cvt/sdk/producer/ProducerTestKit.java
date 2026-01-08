package com.cvt.sdk.producer;

import com.cvt.proto.ContractValidatorGrpc;
import com.cvt.proto.RequestData;
import com.cvt.proto.ResponseData;
import com.cvt.proto.ValidateProducerRequest;
import com.cvt.proto.ValidationResult;

import com.google.gson.Gson;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Metadata;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContext;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContextBuilder;
import io.grpc.stub.MetadataUtils;

import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.TimeUnit;

/**
 * Enables schema compliance testing for producers.
 *
 * <p>Allows producers to validate their API responses against their OpenAPI
 * schema without needing real consumers. Use it in your test suite to ensure
 * your handler output matches your contract.
 *
 * <p>Example with TLS and API key:
 * <pre>{@code
 * ProducerTestKit testKit = ProducerTestKit.builder()
 *         .schemaId("user-api")
 *         .serverAddress("localhost:50051")
 *         .tlsEnabled(true)
 *         .rootCertPath("./certs/ca.crt")
 *         .apiKey("cvt-dev-key-12345")
 *         .build();
 *
 * try {
 *     // Call your handler
 *     UserResponse response = myHandler.getUser("123");
 *
 *     // Validate against schema
 *     TestValidationResult result = testKit.validateResponse(
 *             "GET",
 *             "/users/123",
 *             TestResponseData.builder()
 *                     .statusCode(200)
 *                     .body(response)
 *                     .build()
 *     );
 *
 *     assertTrue(result.isValid(), "Errors: " + result.getErrors());
 * } finally {
 *     testKit.close();
 * }
 * }</pre>
 */
public class ProducerTestKit implements AutoCloseable {
    private static final String DEFAULT_ADDRESS = "localhost:50051";
    private static final Gson GSON = new Gson();
    private static final Metadata.Key<String> API_KEY_HEADER =
            Metadata.Key.of("x-api-key", Metadata.ASCII_STRING_MARSHALLER);

    private final ManagedChannel channel;
    private final ContractValidatorGrpc.ContractValidatorBlockingStub client;
    private final String schemaId;
    private final String schemaVersion;

    private ProducerTestKit(Builder builder) {
        Objects.requireNonNull(builder.schemaId, "schemaId is required");
        if (builder.schemaId.isEmpty()) {
            throw new IllegalArgumentException("schemaId is required");
        }

        this.schemaId = builder.schemaId;
        this.schemaVersion = builder.schemaVersion;

        String address = builder.serverAddress != null ? builder.serverAddress : DEFAULT_ADDRESS;

        try {
            if (builder.tlsEnabled) {
                // Build TLS channel
                SslContextBuilder sslBuilder = GrpcSslContexts.forClient();
                if (builder.rootCertPath != null) {
                    try (InputStream certStream = new FileInputStream(builder.rootCertPath)) {
                        sslBuilder.trustManager(certStream);
                    }
                }
                SslContext sslContext = sslBuilder.build();
                this.channel = NettyChannelBuilder.forTarget(address)
                        .sslContext(sslContext)
                        .build();
            } else {
                this.channel = ManagedChannelBuilder.forTarget(address)
                        .usePlaintext()
                        .build();
            }
        } catch (IOException e) {
            throw new RuntimeException("Failed to initialize TLS", e);
        }

        ContractValidatorGrpc.ContractValidatorBlockingStub stub =
                ContractValidatorGrpc.newBlockingStub(channel);

        // Add API key metadata if configured
        if (builder.apiKey != null && !builder.apiKey.isEmpty()) {
            Metadata metadata = new Metadata();
            metadata.put(API_KEY_HEADER, builder.apiKey);
            stub = stub.withInterceptors(MetadataUtils.newAttachHeadersInterceptor(metadata));
        }

        this.client = stub;
    }

    /**
     * Validates a producer's response against the registered schema.
     *
     * @param method     HTTP method (GET, POST, etc.)
     * @param path       API path with actual values (e.g., /users/123)
     * @param response   Response data to validate
     * @return Validation result
     */
    public TestValidationResult validateResponse(String method, String path, TestResponseData response) {
        return validateResponse(method, path, response, null);
    }

    /**
     * Validates a producer's response against the registered schema with request context.
     *
     * @param method     HTTP method (GET, POST, etc.)
     * @param path       API path with actual values (e.g., /users/123)
     * @param response   Response data to validate
     * @param request    Optional request context for path parameter extraction
     * @return Validation result
     */
    public TestValidationResult validateResponse(
            String method,
            String path,
            TestResponseData response,
            TestRequestContext request) {

        // Build response data
        ResponseData.Builder responseBuilder = ResponseData.newBuilder()
                .setStatusCode(response.getStatusCode())
                .setBody(serializeBody(response.getBody()));

        if (response.getHeaders() != null) {
            responseBuilder.putAllHeaders(response.getHeaders());
        }

        // Build gRPC request
        ValidateProducerRequest.Builder reqBuilder = ValidateProducerRequest.newBuilder()
                .setSchemaId(schemaId)
                .setSchemaVersion(schemaVersion != null ? schemaVersion : "")
                .setMethod(method.toUpperCase())
                .setPath(path)
                .setResponse(responseBuilder);

        // Add optional request context if provided
        if (request != null) {
            RequestData.Builder requestBuilder = RequestData.newBuilder()
                    .setMethod(request.getMethod() != null ? request.getMethod() : method)
                    .setPath(request.getPath() != null ? request.getPath() : path)
                    .setBody(serializeBody(request.getBody()));

            if (request.getHeaders() != null) {
                requestBuilder.putAllHeaders(request.getHeaders());
            }

            reqBuilder.setRequest(requestBuilder);
        }

        // Call server
        ValidationResult result = client.validateProducerResponse(reqBuilder.build());

        return new TestValidationResult(
                result.getValid(),
                new ArrayList<>(result.getErrorsList()),
                result.getValidatedAgainstVersion(),
                result.getValidatedAgainstHash()
        );
    }

    /**
     * Validates a full interaction (request + response) against the schema.
     *
     * @param request  Request context
     * @param response Response data to validate
     * @return Validation result
     */
    public TestValidationResult validateInteraction(
            TestRequestContext request,
            TestResponseData response) {
        return validateResponse(request.getMethod(), request.getPath(), response, request);
    }

    /**
     * Creates a helper for testing a specific endpoint.
     *
     * @param method      HTTP method (GET, POST, etc.)
     * @param pathPattern API path pattern (e.g., /users/{id})
     * @return Endpoint tester
     */
    public EndpointTester forEndpoint(String method, String pathPattern) {
        return new EndpointTester(this, method, pathPattern);
    }

    private String serializeBody(Object body) {
        if (body == null) {
            return "";
        }
        if (body instanceof String) {
            return (String) body;
        }
        return GSON.toJson(body);
    }

    @Override
    public void close() {
        if (channel != null) {
            try {
                channel.shutdown().awaitTermination(5, TimeUnit.SECONDS);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                channel.shutdownNow();
            }
        }
    }

    /**
     * Creates a new builder for ProducerTestKit.
     *
     * @return A new builder
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for ProducerTestKit.
     */
    public static class Builder {
        private String schemaId;
        private String schemaVersion;
        private String serverAddress;
        private String apiKey;
        private boolean tlsEnabled = false;
        private String rootCertPath = null;

        /**
         * Sets the schema ID to validate against (required).
         *
         * @param schemaId The schema ID
         * @return This builder
         */
        public Builder schemaId(String schemaId) {
            this.schemaId = schemaId;
            return this;
        }

        /**
         * Sets the optional schema version to validate against.
         *
         * @param schemaVersion The schema version
         * @return This builder
         */
        public Builder schemaVersion(String schemaVersion) {
            this.schemaVersion = schemaVersion;
            return this;
        }

        /**
         * Sets the server address (default: "localhost:50051").
         *
         * @param serverAddress The server address
         * @return This builder
         */
        public Builder serverAddress(String serverAddress) {
            this.serverAddress = serverAddress;
            return this;
        }

        /**
         * Sets the optional API key for authentication.
         *
         * @param apiKey The API key
         * @return This builder
         */
        public Builder apiKey(String apiKey) {
            this.apiKey = apiKey;
            return this;
        }

        /**
         * Enables or disables TLS.
         *
         * @param enabled True to enable TLS
         * @return This builder
         */
        public Builder tlsEnabled(boolean enabled) {
            this.tlsEnabled = enabled;
            return this;
        }

        /**
         * Sets the path to the root CA certificate for TLS verification.
         *
         * @param path Path to the CA certificate file
         * @return This builder
         */
        public Builder rootCertPath(String path) {
            this.rootCertPath = path;
            return this;
        }

        /**
         * Builds the ProducerTestKit.
         *
         * @return A new ProducerTestKit instance
         */
        public ProducerTestKit build() {
            return new ProducerTestKit(this);
        }
    }

    /**
     * Helper for testing a specific endpoint.
     */
    public static class EndpointTester {
        private final ProducerTestKit testKit;
        private final String method;
        private final String pathPattern;

        EndpointTester(ProducerTestKit testKit, String method, String pathPattern) {
            this.testKit = testKit;
            this.method = method;
            this.pathPattern = pathPattern;
        }

        /**
         * Validates a response for this endpoint.
         *
         * @param response   Response data to validate
         * @param pathValues Values to substitute in path parameters
         * @return Validation result
         */
        public TestValidationResult validateResponse(
                TestResponseData response,
                Map<String, String> pathValues) {

            String actualPath = pathPattern;
            if (pathValues != null) {
                for (Map.Entry<String, String> entry : pathValues.entrySet()) {
                    actualPath = actualPath.replace("{" + entry.getKey() + "}", entry.getValue());
                }
            }

            return testKit.validateResponse(method, actualPath, response);
        }

        /**
         * Validates a response for this endpoint without path substitution.
         *
         * @param response Response data to validate
         * @return Validation result
         */
        public TestValidationResult validateResponse(TestResponseData response) {
            return validateResponse(response, Collections.emptyMap());
        }
    }
}
