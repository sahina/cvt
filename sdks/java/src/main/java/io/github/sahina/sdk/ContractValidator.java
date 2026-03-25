package io.github.sahina.sdk;

import java.io.BufferedReader;
import java.io.FileInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.net.HttpURLConnection;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

import io.github.sahina.sdk.adapters.CapturedInteraction;

import io.github.sahina.proto.CanIDeployRequest;
import io.github.sahina.proto.CanIDeployResponse;
import io.github.sahina.proto.CompareSchemasRequest;
import io.github.sahina.proto.CompareSchemasResponse;
import io.github.sahina.proto.ContractValidatorGrpc;
import io.github.sahina.proto.DeregisterConsumerRequest;
import io.github.sahina.proto.DeregisterConsumerResponse;
import io.github.sahina.proto.GenerateFixtureRequest;
import io.github.sahina.proto.GenerateFixtureResponse;
import io.github.sahina.proto.InteractionRequest;
import io.github.sahina.proto.ListConsumersRequest;
import io.github.sahina.proto.ListConsumersResponse;
import io.github.sahina.proto.ListEndpointsRequest;
import io.github.sahina.proto.ListEndpointsResponse;
import io.github.sahina.proto.OutputType;
import io.github.sahina.proto.RegisterConsumerRequest;
import io.github.sahina.proto.RegisterConsumerResponse;
import io.github.sahina.proto.RegisterSchemaRequest;
import io.github.sahina.proto.RegisterSchemaResponse;
import io.github.sahina.proto.RequestData;
import io.github.sahina.proto.ResponseData;

import com.google.gson.Gson;
import com.google.gson.JsonSyntaxException;

import io.grpc.CallOptions;
import io.grpc.Channel;
import io.grpc.ClientCall;
import io.grpc.ClientInterceptor;
import io.grpc.ClientInterceptors;
import io.grpc.ForwardingClientCall;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import io.grpc.Metadata;
import io.grpc.MethodDescriptor;
import io.grpc.StatusRuntimeException;
import io.grpc.netty.shaded.io.grpc.netty.GrpcSslContexts;
import io.grpc.netty.shaded.io.grpc.netty.NettyChannelBuilder;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContext;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContextBuilder;

/**
 * Client for the Contract Validator Toolkit (CVT).
 * Allows validating HTTP interactions against OpenAPI schemas via a gRPC
 * service.
 *
 * <p>
 * Example usage with TLS and API key:
 * 
 * <pre>{@code
 * ContractValidator validator = ContractValidator.builder()
 *         .address("localhost:9550")
 *         .tlsEnabled(true)
 *         .rootCertPath("./certs/ca.crt")
 *         .apiKey("cvt-dev-key-12345")
 *         .build();
 * }</pre>
 */
public class ContractValidator implements AutoCloseable {
    private static final String DEFAULT_ADDRESS = "localhost:9550";
    private static final Metadata.Key<String> API_KEY_HEADER = Metadata.Key.of("x-api-key",
            Metadata.ASCII_STRING_MARSHALLER);
    private static final Gson GSON = new Gson();

    private final ManagedChannel channel;
    private final ContractValidatorGrpc.ContractValidatorBlockingStub client;
    private String schemaId;

    /**
     * Creates a new ContractValidator instance connecting to the default address
     * (localhost:9550).
     */
    public ContractValidator() {
        this(DEFAULT_ADDRESS);
    }

    /**
     * Creates a new ContractValidator instance.
     *
     * @param address The address of the CVT gRPC server (e.g., "localhost:9550")
     */
    public ContractValidator(String address) {
        this(builder().address(address));
    }

    /**
     * Creates a new ContractValidator instance from a builder.
     *
     * @param builder The builder with configuration options
     */
    private ContractValidator(Builder builder) {
        String address = builder.address;
        if (address == null || address.isEmpty()) {
            address = DEFAULT_ADDRESS;
        }

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

            // Create stub with optional API key
            Channel channelToUse = this.channel;
            if (builder.apiKey != null && !builder.apiKey.isEmpty()) {
                final String apiKey = builder.apiKey;
                ClientInterceptor apiKeyInterceptor = new ClientInterceptor() {
                    @Override
                    public <ReqT, RespT> ClientCall<ReqT, RespT> interceptCall(
                            MethodDescriptor<ReqT, RespT> method,
                            CallOptions callOptions,
                            Channel next) {
                        return new ForwardingClientCall.SimpleForwardingClientCall<ReqT, RespT>(
                                next.newCall(method, callOptions)) {
                            @Override
                            public void start(Listener<RespT> responseListener, Metadata headers) {
                                headers.put(API_KEY_HEADER, apiKey);
                                super.start(responseListener, headers);
                            }
                        };
                    }
                };
                channelToUse = ClientInterceptors.intercept(this.channel, apiKeyInterceptor);
            }

            this.client = ContractValidatorGrpc.newBlockingStub(channelToUse);
            this.schemaId = null;
        } catch (IOException e) {
            throw new RuntimeException("Failed to initialize TLS", e);
        }
    }

    /**
     * Creates a new ContractValidator instance with a custom stub (for testing).
     *
     * @param client The gRPC blocking stub to use
     */
    protected ContractValidator(ContractValidatorGrpc.ContractValidatorBlockingStub client) {
        this.channel = null;
        this.client = client;
        this.schemaId = null;
    }

    /**
     * Creates a new builder for ContractValidator.
     *
     * @return A new Builder instance
     */
    public static Builder builder() {
        return new Builder();
    }

    /**
     * Builder for ContractValidator with TLS and API key configuration.
     */
    public static class Builder {
        private String address = DEFAULT_ADDRESS;
        private boolean tlsEnabled = false;
        private String rootCertPath = null;
        private String apiKey = null;

        /**
         * Sets the server address.
         *
         * @param address The gRPC server address (e.g., "localhost:9550")
         * @return This builder
         */
        public Builder address(String address) {
            this.address = address;
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
         * Sets the API key for authentication.
         *
         * @param apiKey The API key
         * @return This builder
         */
        public Builder apiKey(String apiKey) {
            this.apiKey = apiKey;
            return this;
        }

        /**
         * Builds a new ContractValidator instance.
         *
         * @return A new ContractValidator
         */
        public ContractValidator build() {
            return new ContractValidator(this);
        }
    }

    /**
     * Registers a schema for validation.
     *
     * @param schemaId   A unique identifier for the schema
     * @param schemaPath The path to the schema file (local file path or HTTP/HTTPS
     *                   URL)
     * @throws IOException              if the schema cannot be read
     * @throws IllegalArgumentException if the schema registration fails
     */
    public void registerSchema(String schemaId, String schemaPath) throws IOException {
        String schemaContent;

        // Read schema content first (this will throw IOException if file doesn't exist)
        if (schemaPath.startsWith("http://") || schemaPath.startsWith("https://")) {
            schemaContent = fetchSchemaFromUrl(schemaPath);
        } else {
            schemaContent = readSchemaFromFile(schemaPath);
        }

        // Only try to register if we successfully read the schema
        RegisterSchemaRequest request = RegisterSchemaRequest.newBuilder()
                .setSchemaId(schemaId)
                .setSchemaContent(schemaContent)
                .build();

        try {
            RegisterSchemaResponse response = client.registerSchema(request);
            if (!response.getSuccess()) {
                throw new IllegalArgumentException("Schema registration failed: " + response.getMessage());
            }
            this.schemaId = schemaId;
        } catch (StatusRuntimeException e) {
            throw new IOException("Failed to register schema: " + e.getStatus(), e);
        }
    }

    /**
     * Validates an HTTP interaction (request and response) against the registered
     * schema.
     *
     * @param request  The HTTP request to validate
     * @param response The HTTP response to validate
     * @return A ValidationResult indicating whether the interaction is valid
     * @throws IllegalStateException if no schema has been registered
     */
    public ValidationResult validate(ValidationRequest request, ValidationResponse response) {
        if (schemaId == null) {
            throw new IllegalStateException("Schema not registered. Call registerSchema first.");
        }

        // Build the gRPC request
        RequestData.Builder requestBuilder = RequestData.newBuilder()
                .setMethod(request.getMethod())
                .setPath(request.getPath());

        if (request.getHeaders() != null) {
            requestBuilder.putAllHeaders(convertHeaders(request.getHeaders()));
        }

        if (request.getBody() != null && !request.getBody().isEmpty()) {
            requestBuilder.setBody(request.getBody());
        }

        ResponseData.Builder responseBuilder = ResponseData.newBuilder()
                .setStatusCode(response.getStatusCode());

        if (response.getHeaders() != null) {
            responseBuilder.putAllHeaders(convertHeaders(response.getHeaders()));
        }

        if (response.getBody() != null && !response.getBody().isEmpty()) {
            responseBuilder.setBody(response.getBody());
        }

        InteractionRequest interactionRequest = InteractionRequest.newBuilder()
                .setSchemaId(schemaId)
                .setRequest(requestBuilder)
                .setResponse(responseBuilder)
                .build();

        try {
            io.github.sahina.proto.ValidationResult result = client.validateInteraction(interactionRequest);
            return new ValidationResult(result.getValid(), new ArrayList<>(result.getErrorsList()));
        } catch (StatusRuntimeException e) {
            List<String> errors = new ArrayList<>();
            errors.add("gRPC error: " + e.getStatus().getDescription());
            return new ValidationResult(false, errors);
        }
    }

    /**
     * Registers a schema with version information for comparison.
     *
     * @param schemaId   A unique identifier for the schema
     * @param schemaPath The path to the schema file (local file path or HTTP/HTTPS
     *                   URL)
     * @param version    The semantic version of the schema (e.g., "1.0.0")
     * @throws IOException              if the schema cannot be read
     * @throws IllegalArgumentException if the schema registration fails
     */
    public void registerSchemaWithVersion(String schemaId, String schemaPath, String version) throws IOException {
        String schemaContent;

        if (schemaPath.startsWith("http://") || schemaPath.startsWith("https://")) {
            schemaContent = fetchSchemaFromUrl(schemaPath);
        } else {
            schemaContent = readSchemaFromFile(schemaPath);
        }

        RegisterSchemaRequest request = RegisterSchemaRequest.newBuilder()
                .setSchemaId(schemaId)
                .setSchemaContent(schemaContent)
                .setSchemaVersion(version)
                .build();

        try {
            RegisterSchemaResponse response = client.registerSchema(request);
            if (!response.getSuccess()) {
                throw new IllegalArgumentException("Schema registration failed: " + response.getMessage());
            }
            this.schemaId = schemaId;
        } catch (StatusRuntimeException e) {
            throw new IOException("Failed to register schema: " + e.getStatus(), e);
        }
    }

    /**
     * Compares two schema versions to detect breaking changes.
     *
     * @param schemaId   The schema identifier to compare versions for
     * @param oldVersion The old version to compare from (empty string for previous)
     * @param newVersion The new version to compare to (empty string for latest)
     * @return A CompareResult containing compatibility status and breaking changes
     */
    public CompareResult compareSchemas(String schemaId, String oldVersion, String newVersion) {
        CompareSchemasRequest request = CompareSchemasRequest.newBuilder()
                .setSchemaId(schemaId)
                .setOldVersion(oldVersion != null ? oldVersion : "")
                .setNewVersion(newVersion != null ? newVersion : "")
                .build();

        try {
            CompareSchemasResponse response = client.compareSchemas(request);

            List<BreakingChange> breakingChanges = new ArrayList<>();
            for (io.github.sahina.proto.BreakingChange bc : response.getBreakingChangesList()) {
                breakingChanges.add(new BreakingChange(
                        bc.getType().name(),
                        bc.getPath(),
                        bc.getMethod(),
                        bc.getDescription(),
                        bc.getOldValue(),
                        bc.getNewValue()));
            }

            return new CompareResult(response.getCompatible(), breakingChanges);
        } catch (StatusRuntimeException e) {
            List<BreakingChange> errors = new ArrayList<>();
            return new CompareResult(false, errors);
        }
    }

    /**
     * Generates a complete fixture (request and response) for an endpoint.
     *
     * @param method  The HTTP method (GET, POST, etc.)
     * @param path    The API path (e.g., /users/{id})
     * @param options Optional generation options (can be null for defaults)
     * @return A GeneratedFixture containing request and response
     * @throws IllegalStateException if no schema has been registered
     * @throws RuntimeException      if fixture generation fails
     */
    public GeneratedFixture generateFixture(String method, String path, GenerateOptions options) {
        if (schemaId == null) {
            throw new IllegalStateException("Schema not registered. Call registerSchema first.");
        }

        GenerateFixtureRequest.Builder requestBuilder = GenerateFixtureRequest.newBuilder()
                .setSchemaId(schemaId)
                .setMethod(method.toUpperCase())
                .setPath(path)
                .setOutputType(OutputType.OUTPUT_FIXTURE);

        if (options != null) {
            requestBuilder.setStatusCode(options.getStatusCode());
            requestBuilder.setUseExamples(options.isUseExamples());
            requestBuilder.setContentType(options.getContentType());
        } else {
            requestBuilder.setUseExamples(true);
            requestBuilder.setContentType("application/json");
        }

        try {
            GenerateFixtureResponse response = client.generateFixture(requestBuilder.build());
            if (!response.getSuccess()) {
                throw new RuntimeException("Fixture generation failed: " + response.getMessage());
            }

            io.github.sahina.proto.GeneratedFixture pbFixture = response.getFixture();
            return parseGeneratedFixture(pbFixture);
        } catch (StatusRuntimeException e) {
            throw new RuntimeException("gRPC error: " + e.getStatus().getDescription(), e);
        }
    }

    /**
     * Generates a response for an endpoint.
     *
     * @param method  The HTTP method (GET, POST, etc.)
     * @param path    The API path (e.g., /users/{id})
     * @param options Optional generation options (can be null for defaults)
     * @return A GeneratedResponse
     * @throws IllegalStateException if no schema has been registered
     * @throws RuntimeException      if response generation fails
     */
    public GeneratedResponse generateResponse(String method, String path, GenerateOptions options) {
        if (schemaId == null) {
            throw new IllegalStateException("Schema not registered. Call registerSchema first.");
        }

        GenerateFixtureRequest.Builder requestBuilder = GenerateFixtureRequest.newBuilder()
                .setSchemaId(schemaId)
                .setMethod(method.toUpperCase())
                .setPath(path)
                .setOutputType(OutputType.OUTPUT_RESPONSE);

        if (options != null) {
            requestBuilder.setStatusCode(options.getStatusCode());
            requestBuilder.setUseExamples(options.isUseExamples());
            requestBuilder.setContentType(options.getContentType());
        } else {
            requestBuilder.setUseExamples(true);
            requestBuilder.setContentType("application/json");
        }

        try {
            GenerateFixtureResponse response = client.generateFixture(requestBuilder.build());
            if (!response.getSuccess()) {
                throw new RuntimeException("Response generation failed: " + response.getMessage());
            }

            io.github.sahina.proto.GeneratedResponse pbResp = response.getResponse();
            return parseGeneratedResponse(pbResp);
        } catch (StatusRuntimeException e) {
            throw new RuntimeException("gRPC error: " + e.getStatus().getDescription(), e);
        }
    }

    /**
     * Generates a request body for an endpoint.
     *
     * @param method  The HTTP method (GET, POST, etc.)
     * @param path    The API path (e.g., /users/{id})
     * @param options Optional generation options (can be null for defaults)
     * @return The generated request body (parsed from JSON), or null if no body
     * @throws IllegalStateException if no schema has been registered
     * @throws RuntimeException      if request body generation fails
     */
    public Object generateRequestBody(String method, String path, GenerateOptions options) {
        if (schemaId == null) {
            throw new IllegalStateException("Schema not registered. Call registerSchema first.");
        }

        GenerateFixtureRequest.Builder requestBuilder = GenerateFixtureRequest.newBuilder()
                .setSchemaId(schemaId)
                .setMethod(method.toUpperCase())
                .setPath(path)
                .setOutputType(OutputType.OUTPUT_REQUEST);

        if (options != null) {
            requestBuilder.setStatusCode(options.getStatusCode());
            requestBuilder.setUseExamples(options.isUseExamples());
            requestBuilder.setContentType(options.getContentType());
        } else {
            requestBuilder.setUseExamples(true);
            requestBuilder.setContentType("application/json");
        }

        try {
            GenerateFixtureResponse response = client.generateFixture(requestBuilder.build());
            if (!response.getSuccess()) {
                throw new RuntimeException("Request body generation failed: " + response.getMessage());
            }

            String bodyJson = response.getRequestBody();
            if (bodyJson == null || bodyJson.isEmpty()) {
                return null;
            }
            return parseJsonBody(bodyJson);
        } catch (StatusRuntimeException e) {
            throw new RuntimeException("gRPC error: " + e.getStatus().getDescription(), e);
        }
    }

    /**
     * Lists all endpoints in the registered schema.
     *
     * @return A list of EndpointInfo objects describing each endpoint
     * @throws IllegalStateException if no schema has been registered
     * @throws RuntimeException      if listing endpoints fails
     */
    public List<EndpointInfo> listEndpoints() {
        if (schemaId == null) {
            throw new IllegalStateException("Schema not registered. Call registerSchema first.");
        }

        ListEndpointsRequest request = ListEndpointsRequest.newBuilder()
                .setSchemaId(schemaId)
                .build();

        try {
            ListEndpointsResponse response = client.listEndpoints(request);

            List<EndpointInfo> endpoints = new ArrayList<>();
            for (io.github.sahina.proto.EndpointInfo pbEndpoint : response.getEndpointsList()) {
                endpoints.add(new EndpointInfo(
                        pbEndpoint.getMethod(),
                        pbEndpoint.getPath(),
                        pbEndpoint.getOperationId(),
                        pbEndpoint.getSummary()));
            }
            return endpoints;
        } catch (StatusRuntimeException e) {
            throw new RuntimeException("gRPC error: " + e.getStatus().getDescription(), e);
        }
    }

    // =========================================================================
    // Consumer Registry Methods
    // =========================================================================

    /**
     * Registers a consumer's dependency on a schema.
     * This tracks which consumers use which schemas for deployment safety analysis.
     *
     * @param options Consumer registration options
     * @return ConsumerInfo with the registered consumer details
     * @throws RuntimeException if registration fails
     */
    public ConsumerInfo registerConsumer(RegisterConsumerOptions options) {
        RegisterConsumerRequest.Builder requestBuilder = RegisterConsumerRequest.newBuilder()
                .setConsumerId(options.getConsumerId())
                .setConsumerVersion(options.getConsumerVersion())
                .setSchemaId(options.getSchemaId())
                .setSchemaVersion(options.getSchemaVersion())
                .setEnvironment(options.getEnvironment());

        // Convert used endpoints to proto format
        for (EndpointUsage ep : options.getUsedEndpoints()) {
            io.github.sahina.proto.EndpointUsage.Builder epBuilder = io.github.sahina.proto.EndpointUsage.newBuilder()
                    .setMethod(ep.getMethod())
                    .setPath(ep.getPath());
            if (ep.getUsedFields() != null) {
                epBuilder.addAllUsedFields(ep.getUsedFields());
            }
            requestBuilder.addUsedEndpoints(epBuilder.build());
        }

        try {
            RegisterConsumerResponse response = client.registerConsumer(requestBuilder.build());
            if (!response.getSuccess()) {
                throw new RuntimeException("Consumer registration failed: " + response.getMessage());
            }
            return mapConsumerInfo(response.getConsumer());
        } catch (StatusRuntimeException e) {
            throw new RuntimeException("gRPC error: " + e.getStatus().getDescription(), e);
        }
    }

    /**
     * Lists all consumers that depend on a schema.
     *
     * @param schemaId    The schema identifier
     * @param environment Optional environment filter (can be null)
     * @return A list of ConsumerInfo objects
     */
    public List<ConsumerInfo> listConsumers(String schemaId, String environment) {
        ListConsumersRequest request = ListConsumersRequest.newBuilder()
                .setSchemaId(schemaId)
                .setEnvironment(environment != null ? environment : "")
                .build();

        try {
            ListConsumersResponse response = client.listConsumers(request);

            List<ConsumerInfo> consumers = new ArrayList<>();
            for (io.github.sahina.proto.ConsumerInfo pbConsumer : response.getConsumersList()) {
                consumers.add(mapConsumerInfo(pbConsumer));
            }
            return consumers;
        } catch (StatusRuntimeException e) {
            throw new RuntimeException("gRPC error: " + e.getStatus().getDescription(), e);
        }
    }

    /**
     * Removes a consumer registration for a specific schema.
     *
     * @param consumerId  The consumer identifier
     * @param schemaId    The schema identifier
     * @param environment The environment (dev, staging, prod)
     * @throws RuntimeException if deregistration fails
     */
    public void deregisterConsumer(String consumerId, String schemaId, String environment) {
        DeregisterConsumerRequest request = DeregisterConsumerRequest.newBuilder()
                .setConsumerId(consumerId)
                .setSchemaId(schemaId)
                .setEnvironment(environment)
                .build();

        try {
            DeregisterConsumerResponse response = client.deregisterConsumer(request);
            if (!response.getSuccess()) {
                throw new RuntimeException("Consumer deregistration failed: " + response.getMessage());
            }
        } catch (StatusRuntimeException e) {
            throw new RuntimeException("gRPC error: " + e.getStatus().getDescription(), e);
        }
    }

    /**
     * Checks if a schema version can be safely deployed without breaking consumers.
     *
     * @param schemaId    The schema identifier
     * @param newVersion  The new version to deploy
     * @param environment Target environment (dev, staging, prod)
     * @return CanIDeployResult with deployment safety analysis
     */
    public CanIDeployResult canIDeploy(String schemaId, String newVersion, String environment) {
        CanIDeployRequest request = CanIDeployRequest.newBuilder()
                .setSchemaId(schemaId)
                .setNewVersion(newVersion)
                .setEnvironment(environment)
                .build();

        try {
            CanIDeployResponse response = client.canIDeploy(request);

            // Convert breaking changes
            List<BreakingChange> breakingChanges = new ArrayList<>();
            for (io.github.sahina.proto.BreakingChange bc : response.getBreakingChangesList()) {
                breakingChanges.add(new BreakingChange(
                        bc.getType().name(),
                        bc.getPath(),
                        bc.getMethod(),
                        bc.getDescription(),
                        bc.getOldValue(),
                        bc.getNewValue()));
            }

            // Convert affected consumers
            List<ConsumerImpact> affectedConsumers = new ArrayList<>();
            for (io.github.sahina.proto.ConsumerImpact pbImpact : response.getAffectedConsumersList()) {
                List<BreakingChange> relevantChanges = new ArrayList<>();
                for (io.github.sahina.proto.BreakingChange bc : pbImpact.getRelevantChangesList()) {
                    relevantChanges.add(new BreakingChange(
                            bc.getType().name(),
                            bc.getPath(),
                            bc.getMethod(),
                            bc.getDescription(),
                            bc.getOldValue(),
                            bc.getNewValue()));
                }

                affectedConsumers.add(new ConsumerImpact(
                        pbImpact.getConsumerId(),
                        pbImpact.getConsumerVersion(),
                        pbImpact.getCurrentSchemaVersion(),
                        pbImpact.getEnvironment(),
                        pbImpact.getWillBreak(),
                        relevantChanges));
            }

            return new CanIDeployResult(
                    response.getSafeToDeploy(),
                    response.getSummary(),
                    breakingChanges,
                    affectedConsumers);
        } catch (StatusRuntimeException e) {
            throw new RuntimeException("gRPC error: " + e.getStatus().getDescription(), e);
        }
    }

    // =========================================================================
    // Auto-Registration from Captured Interactions
    // =========================================================================

    /**
     * Builds consumer registration options from captured test interactions.
     * This is a "dry run" method that allows previewing what would be registered
     * without actually making a registration call.
     *
     * <p>The schemaId is auto-extracted from the mock URL hostnames in the interactions.
     * For example, "http://mock.user-api/users/123" extracts "user-api" as the schemaId.
     * You can override this by providing an explicit schemaId in the config.
     *
     * <p>Example usage:
     * <pre>{@code
     * MockHttpClient mock = MockHttpClient.builder()
     *         .validator(validator)
     *         .schemaId("user-api")
     *         .build();
     *
     * // Run tests using the mock client...
     * HttpResponse response = mock.execute(request);
     *
     * // Build registration options from captured interactions
     * AutoRegisterUtils.BuildResult result = validator.buildConsumerFromInteractions(
     *         mock.getInteractions(),
     *         AutoRegisterConfig.builder()
     *                 .consumerId("order-service")
     *                 .consumerVersion("2.1.0")
     *                 .environment("dev")
     *                 .schemaVersion("1.0.0")
     *                 .build()
     * );
     *
     * if (result.hasError()) {
     *     System.err.println("Error: " + result.getError());
     * } else {
     *     System.out.println("Would register: " + result.getOptions());
     * }
     * }</pre>
     *
     * @param interactions The captured interactions from a mock client adapter
     * @param config       Configuration specifying consumer and schema details
     * @return A BuildResult containing either the registration options or an error
     */
    public AutoRegisterUtils.BuildResult buildConsumerFromInteractions(
            List<CapturedInteraction> interactions,
            AutoRegisterConfig config) {
        return AutoRegisterUtils.buildConsumerFromInteractions(interactions, config);
    }

    /**
     * Registers a consumer from captured test interactions.
     * This automatically extracts endpoints and used fields from the interactions
     * and registers the consumer with the CVT server.
     *
     * <p>The schemaId is auto-extracted from the mock URL hostnames in the interactions.
     * For example, "http://mock.user-api/users/123" extracts "user-api" as the schemaId.
     * You can override this by providing an explicit schemaId in the config.
     *
     * <p>Example usage:
     * <pre>{@code
     * MockHttpClient mock = MockHttpClient.builder()
     *         .validator(validator)
     *         .schemaId("user-api")
     *         .build();
     *
     * // Run tests using the mock client...
     * HttpResponse response = mock.execute(request);
     *
     * // Register consumer from captured interactions
     * ConsumerInfo info = validator.registerConsumerFromInteractions(
     *         mock.getInteractions(),
     *         AutoRegisterConfig.builder()
     *                 .consumerId("order-service")
     *                 .consumerVersion("2.1.0")
     *                 .environment("dev")
     *                 .schemaVersion("1.0.0")
     *                 .build()
     * );
     *
     * System.out.println("Registered: " + info.getConsumerId());
     * }</pre>
     *
     * @param interactions The captured interactions from a mock client adapter
     * @param config       Configuration specifying consumer and schema details
     * @return ConsumerInfo with the registered consumer details
     * @throws RuntimeException if building options fails or registration fails
     */
    public ConsumerInfo registerConsumerFromInteractions(
            List<CapturedInteraction> interactions,
            AutoRegisterConfig config) {
        AutoRegisterUtils.BuildResult result = buildConsumerFromInteractions(interactions, config);
        if (result.hasError()) {
            throw new RuntimeException("Failed to build consumer options: " + result.getError());
        }
        return registerConsumer(result.getOptions());
    }

    private ConsumerInfo mapConsumerInfo(io.github.sahina.proto.ConsumerInfo pbConsumer) {
        List<EndpointUsage> usedEndpoints = new ArrayList<>();
        for (io.github.sahina.proto.EndpointUsage pbEp : pbConsumer.getUsedEndpointsList()) {
            usedEndpoints.add(new EndpointUsage(
                    pbEp.getMethod(),
                    pbEp.getPath(),
                    new ArrayList<>(pbEp.getUsedFieldsList())));
        }

        return new ConsumerInfo(
                pbConsumer.getConsumerId(),
                pbConsumer.getConsumerVersion(),
                pbConsumer.getSchemaId(),
                pbConsumer.getSchemaVersion(),
                pbConsumer.getEnvironment(),
                pbConsumer.getRegisteredAt(),
                pbConsumer.getLastValidatedAt(),
                usedEndpoints);
    }

    private GeneratedFixture parseGeneratedFixture(io.github.sahina.proto.GeneratedFixture pbFixture) {
        GeneratedRequest request = parseGeneratedRequest(pbFixture.getRequest());
        GeneratedResponse response = parseGeneratedResponse(pbFixture.getResponse());
        return new GeneratedFixture(request, response);
    }

    private GeneratedRequest parseGeneratedRequest(io.github.sahina.proto.GeneratedRequest pbReq) {
        Object body = null;
        String bodyJson = pbReq.getBody();
        if (bodyJson != null && !bodyJson.isEmpty()) {
            body = parseJsonBody(bodyJson);
        }
        return new GeneratedRequest(
                pbReq.getMethod(),
                pbReq.getPath(),
                new HashMap<>(pbReq.getHeadersMap()),
                body);
    }

    private GeneratedResponse parseGeneratedResponse(io.github.sahina.proto.GeneratedResponse pbResp) {
        Object body = null;
        String bodyJson = pbResp.getBody();
        if (bodyJson != null && !bodyJson.isEmpty()) {
            body = parseJsonBody(bodyJson);
        }
        return new GeneratedResponse(
                pbResp.getStatusCode(),
                new HashMap<>(pbResp.getHeadersMap()),
                body);
    }

    private Object parseJsonBody(String json) {
        try {
            return GSON.fromJson(json, Object.class);
        } catch (JsonSyntaxException e) {
            // If it's not valid JSON, return as string
            return json;
        }
    }

    /**
     * Closes the gRPC client connection.
     * Should be called when the validator is no longer needed to free resources.
     */
    @Override
    public void close() {
        if (channel != null && !channel.isShutdown()) {
            try {
                channel.shutdown().awaitTermination(5, TimeUnit.SECONDS);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                channel.shutdownNow();
            }
        }
    }

    protected String readSchemaFromFile(String filePath) throws IOException {
        return new String(Files.readAllBytes(Paths.get(filePath)));
    }

    protected String fetchSchemaFromUrl(String urlString) throws IOException {
        HttpURLConnection conn = (HttpURLConnection) URI.create(urlString).toURL().openConnection();
        conn.setRequestMethod("GET");
        conn.setConnectTimeout(10000);
        conn.setReadTimeout(10000);

        int responseCode = conn.getResponseCode();
        if (responseCode < 200 || responseCode >= 300) {
            throw new IOException("Failed to fetch schema: HTTP " + responseCode);
        }

        StringBuilder content = new StringBuilder();
        try (BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream()))) {
            String line;
            while ((line = reader.readLine()) != null) {
                content.append(line).append("\n");
            }
        }

        return content.toString();
    }

    private Map<String, String> convertHeaders(Map<String, String> headers) {
        Map<String, String> result = new HashMap<>();
        for (Map.Entry<String, String> entry : headers.entrySet()) {
            String key = entry.getKey();
            String value = entry.getValue();

            if (value != null) {
                result.put(key, value);
            }
        }
        return result;
    }
}
