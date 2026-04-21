package io.github.sahina.sdk;

import io.github.sahina.proto.CanIDeployRequest;
import io.github.sahina.proto.CanIDeployResponse;
import io.github.sahina.proto.CompareSchemasRequest;
import io.github.sahina.proto.CompareSchemasResponse;
import io.github.sahina.proto.ContractValidatorGrpc;
import io.github.sahina.proto.DeregisterConsumerRequest;
import io.github.sahina.proto.DeregisterConsumerResponse;
import io.github.sahina.proto.GenerateFixtureRequest;
import io.github.sahina.proto.GenerateFixtureResponse;
import io.github.sahina.proto.GetSchemaRequest;
import io.github.sahina.proto.GetSchemaResponse;
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
import io.github.sahina.proto.SchemaMetadata;
import io.github.sahina.proto.SchemaOwnership;
import io.github.sahina.proto.ValidationResult;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.io.IOException;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class ContractValidatorTest {

    @Mock
    private ContractValidatorGrpc.ContractValidatorBlockingStub mockStub;

    private ContractValidator validator;

    // Subclass to override file/network operations
    private class TestableContractValidator extends ContractValidator {
        public TestableContractValidator(ContractValidatorGrpc.ContractValidatorBlockingStub client) {
            super(client);
        }

        @Override
        protected String readSchemaFromFile(String filePath) throws IOException {
            if (filePath.contains("nonexistent")) {
                throw new IOException("File not found");
            }
            return "{\"openapi\":\"3.0.0\"}";
        }

        @Override
        protected String fetchSchemaFromUrl(String urlString) throws IOException {
            if (urlString.contains("error")) {
                throw new IOException("Network error");
            }
            return "{\"openapi\":\"3.0.0\"}";
        }
    }

    @BeforeEach
    void setUp() {
        validator = new TestableContractValidator(mockStub);
    }

    @Test
    @DisplayName("Should register schema from local file")
    void testRegisterSchemaFromFile() throws IOException {
        RegisterSchemaResponse response = RegisterSchemaResponse.newBuilder().setSuccess(true).build();
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))).thenReturn(response);

        validator.registerSchema("test-schema", "path/to/schema.json");

        ArgumentCaptor<RegisterSchemaRequest> captor = ArgumentCaptor.forClass(RegisterSchemaRequest.class);
        verify(mockStub).registerSchema(captor.capture());
        assertEquals("test-schema", captor.getValue().getSchemaId());
        assertEquals("{\"openapi\":\"3.0.0\"}", captor.getValue().getSchemaContent());
    }

    @Test
    @DisplayName("Should register schema from URL")
    void testRegisterSchemaFromUrl() throws IOException {
        RegisterSchemaResponse response = RegisterSchemaResponse.newBuilder().setSuccess(true).build();
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))).thenReturn(response);

        validator.registerSchema("url-schema", "http://example.com/schema.json");

        verify(mockStub).registerSchema(any(RegisterSchemaRequest.class));
    }

    @Test
    @DisplayName("Should throw exception when schema file not found")
    void testRegisterSchemaFileNotFound() {
        assertThrows(IOException.class, () -> {
            validator.registerSchema("test-schema", "/path/to/nonexistent/file.json");
        });
    }

    @Test
    @DisplayName("Should throw exception when registration fails")
    void testRegisterSchemaFailure() {
        RegisterSchemaResponse response = RegisterSchemaResponse.newBuilder() 
                .setSuccess(false)
                .setMessage("Invalid schema")
                .build();
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))).thenReturn(response);

        IllegalArgumentException exception = assertThrows(IllegalArgumentException.class, () -> {
            validator.registerSchema("test-schema", "path/to/schema.json");
        });
        assertEquals("Schema registration failed: Invalid schema", exception.getMessage());
    }

    @Test
    @DisplayName("Should throw IOException on gRPC error during registration")
    void testRegisterSchemaGrpcError() {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))) 
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        assertThrows(IOException.class, () -> {
            validator.registerSchema("test-schema", "path/to/schema.json");
        });
    }

    @Test
    @DisplayName("Should validate valid interaction")
    void testValidateValid() throws IOException {
        // Register schema first
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))) 
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        // Mock validation response
        ValidationResult protoResult = ValidationResult.newBuilder() 
                .setValid(true)
                .build();
        when(mockStub.validateInteraction(any(InteractionRequest.class))).thenReturn(protoResult);

        ValidationRequest request = ValidationRequest.builder()
                .method("POST")
                .path("/users")
                .header("Content-Type", "application/json")
                .body("{}")
                .build();

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(201)
                .build();

        io.github.sahina.sdk.ValidationResult result = validator.validate(request, response);

        assertTrue(result.isValid());
        assertTrue(result.getErrors().isEmpty());
    }

    @Test
    @DisplayName("Should validate invalid interaction")
    void testValidateInvalid() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))) 
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        ValidationResult protoResult = ValidationResult.newBuilder() 
                .setValid(false)
                .addErrors("Missing field")
                .build();
        when(mockStub.validateInteraction(any(InteractionRequest.class))).thenReturn(protoResult);

        ValidationRequest request = ValidationRequest.builder().method("GET").path("/").build();
        ValidationResponse response = ValidationResponse.builder().statusCode(200).build();

        io.github.sahina.sdk.ValidationResult result = validator.validate(request, response);

        assertFalse(result.isValid());
        assertEquals(1, result.getErrors().size());
        assertEquals("Missing field", result.getErrors().get(0));
    }

    @Test
    @DisplayName("Should handle gRPC error during validation")
    void testValidateGrpcError() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))) 
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        when(mockStub.validateInteraction(any(InteractionRequest.class))) 
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        ValidationRequest request = ValidationRequest.builder().method("GET").path("/").build();
        ValidationResponse response = ValidationResponse.builder().statusCode(200).build();

        io.github.sahina.sdk.ValidationResult result = validator.validate(request, response);

        assertFalse(result.isValid());
        assertTrue(result.getErrors().get(0).contains("gRPC error"));
    }

    @Test
    @DisplayName("Should throw exception when validating without schema registration")
    void testValidateWithoutSchema() {
        ValidationRequest request = ValidationRequest.builder().method("GET").path("/").build();
        ValidationResponse response = ValidationResponse.builder().statusCode(200).build();

        assertThrows(IllegalStateException.class, () -> {
            validator.validate(request, response);
        });
    }

    @Test
    @DisplayName("Should require method in ValidationRequest")
    void testValidationRequestRequiresMethod() {
        IllegalArgumentException exception = assertThrows(IllegalArgumentException.class, () -> {
            ValidationRequest.builder() 
                    .path("/pet")
                    .build();
        });
        assertTrue(exception.getMessage().contains("method"));
    }

    @Test
    @DisplayName("Should require path in ValidationRequest")
    void testValidationRequestRequiresPath() {
        IllegalArgumentException exception = assertThrows(IllegalArgumentException.class, () -> {
            ValidationRequest.builder() 
                    .method("GET")
                    .build();
        });
        assertTrue(exception.getMessage().contains("Path"));
    }

    @Test
    @DisplayName("Should build ValidationRequest with all fields")
    void testBuildValidationRequest() {
        Map<String, String> headers = new HashMap<>();
        headers.put("content-type", "application/json");

        ValidationRequest request = ValidationRequest.builder()
                .method("POST")
                .path("/pet")
                .headers(headers)
                .body("{\"name\": \"Fluffy\"}")
                .build();

        assertEquals("POST", request.getMethod());
        assertEquals("/pet", request.getPath());
        assertEquals("application/json", request.getHeaders().get("content-type"));
        assertEquals("{\"name\": \"Fluffy\"}", request.getBody());
    }

    @Test
    @DisplayName("Should build ValidationResponse with all fields")
    void testBuildValidationResponse() {
        Map<String, String> headers = new HashMap<>();
        headers.put("content-type", "application/json");

        ValidationResponse response = ValidationResponse.builder()
                .statusCode(201)
                .headers(headers)
                .body("{\"id\": 1}")
                .build();

        assertEquals(201, response.getStatusCode());
        assertEquals("application/json", response.getHeaders().get("content-type"));
        assertEquals("{\"id\": 1}", response.getBody());
    }

    @Test
    @DisplayName("ValidationResult should have correct properties")
    void testValidationResult() {
        java.util.List<String> errors = java.util.Arrays.asList("error1", "error2");
        io.github.sahina.sdk.ValidationResult result = new io.github.sahina.sdk.ValidationResult(false, errors);

        assertFalse(result.isValid());
        assertEquals(2, result.getErrors().size());
        assertTrue(result.getErrors().contains("error1"));
        assertTrue(result.getErrors().contains("error2"));
    }

    // ============ Fixture Generation Tests ============

    @Test
    @DisplayName("Should throw exception when generating fixture without schema")
    void testGenerateFixtureWithoutSchema() {
        assertThrows(IllegalStateException.class, () -> {
            validator.generateFixture("GET", "/users", null);
        });
    }

    @Test
    @DisplayName("Should generate fixture successfully")
    void testGenerateFixture() throws IOException {
        // Register schema first
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        // Mock generate fixture response
        io.github.sahina.proto.GeneratedRequest protoRequest = io.github.sahina.proto.GeneratedRequest.newBuilder()
                .setMethod("GET")
                .setPath("/users/123")
                .putHeaders("Content-Type", "application/json")
                .setBody("{\"id\":123}")
                .build();

        io.github.sahina.proto.GeneratedResponse protoResponse = io.github.sahina.proto.GeneratedResponse.newBuilder()
                .setStatusCode(200)
                .putHeaders("Content-Type", "application/json")
                .setBody("{\"name\":\"John\"}")
                .build();

        io.github.sahina.proto.GeneratedFixture protoFixture = io.github.sahina.proto.GeneratedFixture.newBuilder()
                .setRequest(protoRequest)
                .setResponse(protoResponse)
                .build();

        GenerateFixtureResponse response = GenerateFixtureResponse.newBuilder()
                .setSuccess(true)
                .setFixture(protoFixture)
                .build();

        when(mockStub.generateFixture(any(GenerateFixtureRequest.class))).thenReturn(response);

        GeneratedFixture fixture = validator.generateFixture("GET", "/users/{id}", null);

        assertNotNull(fixture);
        assertNotNull(fixture.getRequest());
        assertNotNull(fixture.getResponse());
        assertEquals("GET", fixture.getRequest().getMethod());
        assertEquals("/users/123", fixture.getRequest().getPath());
        assertEquals(200, fixture.getResponse().getStatusCode());
    }

    @Test
    @DisplayName("Should generate fixture with options")
    void testGenerateFixtureWithOptions() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        GenerateFixtureResponse response = GenerateFixtureResponse.newBuilder()
                .setSuccess(true)
                .setFixture(io.github.sahina.proto.GeneratedFixture.newBuilder()
                        .setRequest(io.github.sahina.proto.GeneratedRequest.newBuilder().build())
                        .setResponse(io.github.sahina.proto.GeneratedResponse.newBuilder().setStatusCode(201).build())
                        .build())
                .build();

        when(mockStub.generateFixture(any(GenerateFixtureRequest.class))).thenReturn(response);

        GenerateOptions options = GenerateOptions.builder()
                .statusCode(201)
                .useExamples(true)
                .contentType("application/json")
                .build();

        GeneratedFixture fixture = validator.generateFixture("POST", "/users", options);

        assertNotNull(fixture);

        ArgumentCaptor<GenerateFixtureRequest> captor = ArgumentCaptor.forClass(GenerateFixtureRequest.class);
        verify(mockStub).generateFixture(captor.capture());
        assertEquals(201, captor.getValue().getStatusCode());
        assertEquals(OutputType.OUTPUT_FIXTURE, captor.getValue().getOutputType());
    }

    @Test
    @DisplayName("Should throw exception when fixture generation fails")
    void testGenerateFixtureFailure() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        GenerateFixtureResponse response = GenerateFixtureResponse.newBuilder()
                .setSuccess(false)
                .setMessage("Endpoint not found")
                .build();

        when(mockStub.generateFixture(any(GenerateFixtureRequest.class))).thenReturn(response);

        RuntimeException exception = assertThrows(RuntimeException.class, () -> {
            validator.generateFixture("GET", "/unknown", null);
        });
        assertTrue(exception.getMessage().contains("Endpoint not found"));
    }

    @Test
    @DisplayName("Should throw exception when generating response without schema")
    void testGenerateResponseWithoutSchema() {
        assertThrows(IllegalStateException.class, () -> {
            validator.generateResponse("GET", "/users", null);
        });
    }

    @Test
    @DisplayName("Should generate response successfully")
    void testGenerateResponse() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        io.github.sahina.proto.GeneratedResponse protoResponse = io.github.sahina.proto.GeneratedResponse.newBuilder()
                .setStatusCode(200)
                .putHeaders("Content-Type", "application/json")
                .setBody("{\"users\":[]}")
                .build();

        GenerateFixtureResponse response = GenerateFixtureResponse.newBuilder()
                .setSuccess(true)
                .setResponse(protoResponse)
                .build();

        when(mockStub.generateFixture(any(GenerateFixtureRequest.class))).thenReturn(response);

        GeneratedResponse generatedResponse = validator.generateResponse("GET", "/users", null);

        assertNotNull(generatedResponse);
        assertEquals(200, generatedResponse.getStatusCode());

        ArgumentCaptor<GenerateFixtureRequest> captor = ArgumentCaptor.forClass(GenerateFixtureRequest.class);
        verify(mockStub).generateFixture(captor.capture());
        assertEquals(OutputType.OUTPUT_RESPONSE, captor.getValue().getOutputType());
    }

    @Test
    @DisplayName("Should throw exception when generating request body without schema")
    void testGenerateRequestBodyWithoutSchema() {
        assertThrows(IllegalStateException.class, () -> {
            validator.generateRequestBody("POST", "/users", null);
        });
    }

    @Test
    @DisplayName("Should generate request body successfully")
    void testGenerateRequestBody() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        GenerateFixtureResponse response = GenerateFixtureResponse.newBuilder()
                .setSuccess(true)
                .setRequestBody("{\"name\":\"test\"}")
                .build();

        when(mockStub.generateFixture(any(GenerateFixtureRequest.class))).thenReturn(response);

        Object body = validator.generateRequestBody("POST", "/users", null);

        assertNotNull(body);

        ArgumentCaptor<GenerateFixtureRequest> captor = ArgumentCaptor.forClass(GenerateFixtureRequest.class);
        verify(mockStub).generateFixture(captor.capture());
        assertEquals(OutputType.OUTPUT_REQUEST, captor.getValue().getOutputType());
    }

    @Test
    @DisplayName("Should return null for empty request body")
    void testGenerateRequestBodyEmpty() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        GenerateFixtureResponse response = GenerateFixtureResponse.newBuilder()
                .setSuccess(true)
                .setRequestBody("")
                .build();

        when(mockStub.generateFixture(any(GenerateFixtureRequest.class))).thenReturn(response);

        Object body = validator.generateRequestBody("GET", "/users", null);

        assertNull(body);
    }

    @Test
    @DisplayName("Should throw exception when listing endpoints without schema")
    void testListEndpointsWithoutSchema() {
        assertThrows(IllegalStateException.class, () -> {
            validator.listEndpoints();
        });
    }

    @Test
    @DisplayName("Should list endpoints successfully")
    void testListEndpoints() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        ListEndpointsResponse response = ListEndpointsResponse.newBuilder()
                .addEndpoints(io.github.sahina.proto.EndpointInfo.newBuilder()
                        .setMethod("GET")
                        .setPath("/users")
                        .setOperationId("getUsers")
                        .setSummary("Get all users")
                        .build())
                .addEndpoints(io.github.sahina.proto.EndpointInfo.newBuilder()
                        .setMethod("POST")
                        .setPath("/users")
                        .setOperationId("createUser")
                        .setSummary("Create a user")
                        .build())
                .build();

        when(mockStub.listEndpoints(any(ListEndpointsRequest.class))).thenReturn(response);

        List<EndpointInfo> endpoints = validator.listEndpoints();

        assertNotNull(endpoints);
        assertEquals(2, endpoints.size());
        assertEquals("GET", endpoints.get(0).getMethod());
        assertEquals("/users", endpoints.get(0).getPath());
        assertEquals("getUsers", endpoints.get(0).getOperationId());
        assertEquals("Get all users", endpoints.get(0).getSummary());
    }

    @Test
    @DisplayName("Should handle gRPC error when listing endpoints")
    void testListEndpointsGrpcError() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        validator.registerSchema("test-schema", "path/to/schema.json");

        when(mockStub.listEndpoints(any(ListEndpointsRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        RuntimeException exception = assertThrows(RuntimeException.class, () -> {
            validator.listEndpoints();
        });
        assertTrue(exception.getMessage().contains("gRPC error"));
    }

    // ============ GenerateOptions Tests ============

    @Test
    @DisplayName("GenerateOptions should have correct default values")
    void testGenerateOptionsDefaults() {
        GenerateOptions options = GenerateOptions.builder().build();

        assertEquals(0, options.getStatusCode());
        assertTrue(options.isUseExamples());
        assertEquals("application/json", options.getContentType());
    }

    @Test
    @DisplayName("GenerateOptions should accept custom values")
    void testGenerateOptionsCustom() {
        GenerateOptions options = GenerateOptions.builder()
                .statusCode(404)
                .useExamples(false)
                .contentType("text/plain")
                .build();

        assertEquals(404, options.getStatusCode());
        assertFalse(options.isUseExamples());
        assertEquals("text/plain", options.getContentType());
    }

    // ============ Compare Schemas Tests ============

    @Test
    @DisplayName("Should compare compatible schemas")
    void testCompareSchemasCompatible() {
        CompareSchemasResponse response = CompareSchemasResponse.newBuilder()
                .setCompatible(true)
                .build();
        when(mockStub.compareSchemas(any(CompareSchemasRequest.class))).thenReturn(response);

        CompareResult result = validator.compareSchemas("test-schema", "1.0.0", "2.0.0");

        assertTrue(result.isCompatible());
        assertTrue(result.getBreakingChanges().isEmpty());
    }

    @Test
    @DisplayName("Should detect breaking changes between schemas")
    void testCompareSchemasWithBreakingChanges() {
        io.github.sahina.proto.BreakingChange protoChange = io.github.sahina.proto.BreakingChange.newBuilder()
                .setType(io.github.sahina.proto.BreakingChangeType.ENDPOINT_REMOVED)
                .setPath("/users/{id}")
                .setMethod("DELETE")
                .setDescription("Endpoint was removed")
                .setOldValue("existed")
                .setNewValue("")
                .build();

        CompareSchemasResponse response = CompareSchemasResponse.newBuilder()
                .setCompatible(false)
                .addBreakingChanges(protoChange)
                .build();
        when(mockStub.compareSchemas(any(CompareSchemasRequest.class))).thenReturn(response);

        CompareResult result = validator.compareSchemas("test-schema", "1.0.0", "2.0.0");

        assertFalse(result.isCompatible());
        assertEquals(1, result.getBreakingChanges().size());
        assertEquals("/users/{id}", result.getBreakingChanges().get(0).getPath());
        assertEquals("DELETE", result.getBreakingChanges().get(0).getMethod());
    }

    @Test
    @DisplayName("Should handle null versions in compareSchemas")
    void testCompareSchemasNullVersions() {
        CompareSchemasResponse response = CompareSchemasResponse.newBuilder()
                .setCompatible(true)
                .build();
        when(mockStub.compareSchemas(any(CompareSchemasRequest.class))).thenReturn(response);

        CompareResult result = validator.compareSchemas("test-schema", null, null);

        assertTrue(result.isCompatible());

        ArgumentCaptor<CompareSchemasRequest> captor = ArgumentCaptor.forClass(CompareSchemasRequest.class);
        verify(mockStub).compareSchemas(captor.capture());
        assertEquals("", captor.getValue().getOldVersion());
        assertEquals("", captor.getValue().getNewVersion());
    }

    @Test
    @DisplayName("Should handle gRPC error during schema comparison")
    void testCompareSchemasGrpcError() {
        when(mockStub.compareSchemas(any(CompareSchemasRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        CompareResult result = validator.compareSchemas("test-schema", "1.0.0", "2.0.0");

        assertFalse(result.isCompatible());
        assertTrue(result.getBreakingChanges().isEmpty());
    }

    // ============ Register Schema With Version Tests ============

    @Test
    @DisplayName("Should register schema with version")
    void testRegisterSchemaWithVersion() throws IOException {
        RegisterSchemaResponse response = RegisterSchemaResponse.newBuilder().setSuccess(true).build();
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))).thenReturn(response);

        validator.registerSchemaWithVersion("test-schema", "path/to/schema.json", "1.0.0");

        ArgumentCaptor<RegisterSchemaRequest> captor = ArgumentCaptor.forClass(RegisterSchemaRequest.class);
        verify(mockStub).registerSchema(captor.capture());
        assertEquals("test-schema", captor.getValue().getSchemaId());
        assertEquals("1.0.0", captor.getValue().getSchemaVersion());
    }

    @Test
    @DisplayName("Should throw exception when versioned registration fails")
    void testRegisterSchemaWithVersionFailure() {
        RegisterSchemaResponse response = RegisterSchemaResponse.newBuilder()
                .setSuccess(false)
                .setMessage("Version conflict")
                .build();
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class))).thenReturn(response);

        assertThrows(IllegalArgumentException.class, () -> {
            validator.registerSchemaWithVersion("test-schema", "path/to/schema.json", "1.0.0");
        });
    }

    // ============ Consumer Registry Tests ============

    @Test
    @DisplayName("Should register consumer successfully")
    void testRegisterConsumer() {
        io.github.sahina.proto.ConsumerInfo protoConsumer = io.github.sahina.proto.ConsumerInfo.newBuilder()
                .setConsumerId("order-service")
                .setConsumerVersion("1.0.0")
                .setSchemaId("user-api")
                .setSchemaVersion("1.0.0")
                .setEnvironment("prod")
                .setRegisteredAt(1000L)
                .setLastValidatedAt(2000L)
                .build();

        RegisterConsumerResponse response = RegisterConsumerResponse.newBuilder()
                .setSuccess(true)
                .setConsumer(protoConsumer)
                .build();
        when(mockStub.registerConsumer(any(RegisterConsumerRequest.class))).thenReturn(response);

        RegisterConsumerOptions options = RegisterConsumerOptions.builder()
                .consumerId("order-service")
                .consumerVersion("1.0.0")
                .schemaId("user-api")
                .schemaVersion("1.0.0")
                .environment("prod")
                .build();

        ConsumerInfo result = validator.registerConsumer(options);

        assertEquals("order-service", result.getConsumerId());
        assertEquals("prod", result.getEnvironment());
    }

    @Test
    @DisplayName("Should register consumer with used endpoints")
    void testRegisterConsumerWithEndpoints() {
        io.github.sahina.proto.ConsumerInfo protoConsumer = io.github.sahina.proto.ConsumerInfo.newBuilder()
                .setConsumerId("order-service")
                .setConsumerVersion("1.0.0")
                .setSchemaId("user-api")
                .setSchemaVersion("1.0.0")
                .setEnvironment("prod")
                .build();

        RegisterConsumerResponse response = RegisterConsumerResponse.newBuilder()
                .setSuccess(true)
                .setConsumer(protoConsumer)
                .build();
        when(mockStub.registerConsumer(any(RegisterConsumerRequest.class))).thenReturn(response);

        List<EndpointUsage> endpoints = new ArrayList<>();
        endpoints.add(new EndpointUsage("GET", "/users/{id}", java.util.Arrays.asList("id", "name")));

        RegisterConsumerOptions options = RegisterConsumerOptions.builder()
                .consumerId("order-service")
                .consumerVersion("1.0.0")
                .schemaId("user-api")
                .schemaVersion("1.0.0")
                .environment("prod")
                .usedEndpoints(endpoints)
                .build();

        validator.registerConsumer(options);

        ArgumentCaptor<RegisterConsumerRequest> captor = ArgumentCaptor.forClass(RegisterConsumerRequest.class);
        verify(mockStub).registerConsumer(captor.capture());
        assertEquals(1, captor.getValue().getUsedEndpointsCount());
        assertEquals("GET", captor.getValue().getUsedEndpoints(0).getMethod());
    }

    @Test
    @DisplayName("Should throw exception when consumer registration fails")
    void testRegisterConsumerFailure() {
        RegisterConsumerResponse response = RegisterConsumerResponse.newBuilder()
                .setSuccess(false)
                .setMessage("Registration failed")
                .build();
        when(mockStub.registerConsumer(any(RegisterConsumerRequest.class))).thenReturn(response);

        RegisterConsumerOptions options = RegisterConsumerOptions.builder()
                .consumerId("order-service")
                .consumerVersion("1.0.0")
                .schemaId("user-api")
                .schemaVersion("1.0.0")
                .environment("prod")
                .build();

        RuntimeException exception = assertThrows(RuntimeException.class, () -> {
            validator.registerConsumer(options);
        });
        assertTrue(exception.getMessage().contains("Registration failed"));
    }

    @Test
    @DisplayName("Should list consumers successfully")
    void testListConsumers() {
        io.github.sahina.proto.ConsumerInfo protoConsumer = io.github.sahina.proto.ConsumerInfo.newBuilder()
                .setConsumerId("order-service")
                .setConsumerVersion("1.0.0")
                .setSchemaId("user-api")
                .setSchemaVersion("1.0.0")
                .setEnvironment("prod")
                .build();

        ListConsumersResponse response = ListConsumersResponse.newBuilder()
                .addConsumers(protoConsumer)
                .build();
        when(mockStub.listConsumers(any(ListConsumersRequest.class))).thenReturn(response);

        List<ConsumerInfo> consumers = validator.listConsumers("user-api", "prod");

        assertEquals(1, consumers.size());
        assertEquals("order-service", consumers.get(0).getConsumerId());
    }

    @Test
    @DisplayName("Should list consumers with null environment")
    void testListConsumersNullEnvironment() {
        ListConsumersResponse response = ListConsumersResponse.newBuilder().build();
        when(mockStub.listConsumers(any(ListConsumersRequest.class))).thenReturn(response);

        validator.listConsumers("user-api", null);

        ArgumentCaptor<ListConsumersRequest> captor = ArgumentCaptor.forClass(ListConsumersRequest.class);
        verify(mockStub).listConsumers(captor.capture());
        assertEquals("", captor.getValue().getEnvironment());
    }

    @Test
    @DisplayName("Should handle gRPC error when listing consumers")
    void testListConsumersGrpcError() {
        when(mockStub.listConsumers(any(ListConsumersRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        assertThrows(RuntimeException.class, () -> {
            validator.listConsumers("user-api", "prod");
        });
    }

    @Test
    @DisplayName("Should deregister consumer successfully")
    void testDeregisterConsumer() {
        DeregisterConsumerResponse response = DeregisterConsumerResponse.newBuilder()
                .setSuccess(true)
                .build();
        when(mockStub.deregisterConsumer(any(DeregisterConsumerRequest.class))).thenReturn(response);

        validator.deregisterConsumer("order-service", "user-api", "prod");

        verify(mockStub).deregisterConsumer(any(DeregisterConsumerRequest.class));
    }

    @Test
    @DisplayName("Should throw exception when consumer deregistration fails")
    void testDeregisterConsumerFailure() {
        DeregisterConsumerResponse response = DeregisterConsumerResponse.newBuilder()
                .setSuccess(false)
                .setMessage("Consumer not found")
                .build();
        when(mockStub.deregisterConsumer(any(DeregisterConsumerRequest.class))).thenReturn(response);

        RuntimeException exception = assertThrows(RuntimeException.class, () -> {
            validator.deregisterConsumer("unknown", "user-api", "prod");
        });
        assertTrue(exception.getMessage().contains("Consumer not found"));
    }

    @Test
    @DisplayName("Should handle gRPC error when deregistering consumer")
    void testDeregisterConsumerGrpcError() {
        when(mockStub.deregisterConsumer(any(DeregisterConsumerRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        assertThrows(RuntimeException.class, () -> {
            validator.deregisterConsumer("order-service", "user-api", "prod");
        });
    }

    // ============ Can I Deploy Tests ============

    @Test
    @DisplayName("Should return safe to deploy")
    void testCanIDeploySafe() {
        CanIDeployResponse response = CanIDeployResponse.newBuilder()
                .setSafeToDeploy(true)
                .setSummary("No breaking changes")
                .build();
        when(mockStub.canIDeploy(any(CanIDeployRequest.class))).thenReturn(response);

        CanIDeployResult result = validator.canIDeploy("user-api", "2.0.0", "prod");

        assertTrue(result.isSafeToDeploy());
        assertEquals("No breaking changes", result.getSummary());
        assertTrue(result.getBreakingChanges().isEmpty());
        assertTrue(result.getAffectedConsumers().isEmpty());
    }

    @Test
    @DisplayName("Should return affected consumers when unsafe")
    void testCanIDeployUnsafe() {
        io.github.sahina.proto.BreakingChange protoChange = io.github.sahina.proto.BreakingChange.newBuilder()
                .setType(io.github.sahina.proto.BreakingChangeType.ENDPOINT_REMOVED)
                .setPath("/users")
                .setMethod("DELETE")
                .setDescription("Endpoint removed")
                .build();

        io.github.sahina.proto.ConsumerImpact protoImpact = io.github.sahina.proto.ConsumerImpact.newBuilder()
                .setConsumerId("order-service")
                .setConsumerVersion("1.0.0")
                .setCurrentSchemaVersion("1.0.0")
                .setEnvironment("prod")
                .setWillBreak(true)
                .addRelevantChanges(protoChange)
                .build();

        CanIDeployResponse response = CanIDeployResponse.newBuilder()
                .setSafeToDeploy(false)
                .setSummary("Breaking changes affect consumers")
                .addBreakingChanges(protoChange)
                .addAffectedConsumers(protoImpact)
                .build();
        when(mockStub.canIDeploy(any(CanIDeployRequest.class))).thenReturn(response);

        CanIDeployResult result = validator.canIDeploy("user-api", "2.0.0", "prod");

        assertFalse(result.isSafeToDeploy());
        assertEquals(1, result.getBreakingChanges().size());
        assertEquals(1, result.getAffectedConsumers().size());
        assertTrue(result.getAffectedConsumers().get(0).isWillBreak());
        assertEquals("order-service", result.getAffectedConsumers().get(0).getConsumerId());
    }

    @Test
    @DisplayName("Should handle gRPC error in canIDeploy")
    void testCanIDeployGrpcError() {
        when(mockStub.canIDeploy(any(CanIDeployRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        assertThrows(RuntimeException.class, () -> {
            validator.canIDeploy("user-api", "2.0.0", "prod");
        });
    }

    // ============ Data Class Tests ============

    @Test
    @DisplayName("BreakingChange should have correct properties")
    void testBreakingChange() {
        BreakingChange change = new BreakingChange(
                "ENDPOINT_REMOVED",
                "/users/{id}",
                "DELETE",
                "Endpoint was removed",
                "existed",
                "");

        assertEquals("ENDPOINT_REMOVED", change.getType());
        assertEquals("/users/{id}", change.getPath());
        assertEquals("DELETE", change.getMethod());
        assertEquals("Endpoint was removed", change.getDescription());
        assertEquals("existed", change.getOldValue());
        assertEquals("", change.getNewValue());
    }

    @Test
    @DisplayName("CompareResult should have correct properties")
    void testCompareResult() {
        List<BreakingChange> changes = new ArrayList<>();
        changes.add(new BreakingChange("TYPE", "/path", "GET", "desc", "old", "new"));

        CompareResult result = new CompareResult(false, changes);

        assertFalse(result.isCompatible());
        assertEquals(1, result.getBreakingChanges().size());
    }

    @Test
    @DisplayName("EndpointInfo should have correct properties")
    void testEndpointInfo() {
        EndpointInfo info = new EndpointInfo("GET", "/users", "getUsers", "Get all users");

        assertEquals("GET", info.getMethod());
        assertEquals("/users", info.getPath());
        assertEquals("getUsers", info.getOperationId());
        assertEquals("Get all users", info.getSummary());
    }

    @Test
    @DisplayName("EndpointUsage should have correct properties")
    void testEndpointUsage() {
        List<String> fields = java.util.Arrays.asList("id", "name");
        EndpointUsage usage = new EndpointUsage("GET", "/users/{id}", fields);

        assertEquals("GET", usage.getMethod());
        assertEquals("/users/{id}", usage.getPath());
        assertEquals(2, usage.getUsedFields().size());
    }

    @Test
    @DisplayName("ConsumerInfo should have correct properties")
    void testConsumerInfo() {
        List<EndpointUsage> endpoints = new ArrayList<>();
        endpoints.add(new EndpointUsage("GET", "/users", null));

        ConsumerInfo info = new ConsumerInfo(
                "order-service",
                "1.0.0",
                "user-api",
                "1.0.0",
                "prod",
                1000L,
                2000L,
                endpoints);

        assertEquals("order-service", info.getConsumerId());
        assertEquals("1.0.0", info.getConsumerVersion());
        assertEquals("user-api", info.getSchemaId());
        assertEquals("prod", info.getEnvironment());
        assertEquals(1000L, info.getRegisteredAt());
        assertEquals(1, info.getUsedEndpoints().size());
    }

    @Test
    @DisplayName("ConsumerImpact should have correct properties")
    void testConsumerImpact() {
        List<BreakingChange> changes = new ArrayList<>();
        changes.add(new BreakingChange("TYPE", "/path", "GET", "desc", "old", "new"));

        ConsumerImpact impact = new ConsumerImpact(
                "order-service",
                "1.0.0",
                "1.0.0",
                "prod",
                true,
                changes);

        assertEquals("order-service", impact.getConsumerId());
        assertTrue(impact.isWillBreak());
        assertEquals(1, impact.getRelevantChanges().size());
    }

    @Test
    @DisplayName("CanIDeployResult should have correct properties")
    void testCanIDeployResult() {
        List<BreakingChange> changes = new ArrayList<>();
        List<ConsumerImpact> consumers = new ArrayList<>();

        CanIDeployResult result = new CanIDeployResult(true, "Safe", changes, consumers);

        assertTrue(result.isSafeToDeploy());
        assertEquals("Safe", result.getSummary());
        assertTrue(result.getBreakingChanges().isEmpty());
        assertTrue(result.getAffectedConsumers().isEmpty());
    }

    @Test
    @DisplayName("RegisterConsumerOptions builder should work correctly")
    void testRegisterConsumerOptionsBuilder() {
        List<EndpointUsage> endpoints = new ArrayList<>();
        endpoints.add(new EndpointUsage("GET", "/users", null));

        RegisterConsumerOptions options = RegisterConsumerOptions.builder()
                .consumerId("order-service")
                .consumerVersion("1.0.0")
                .schemaId("user-api")
                .schemaVersion("1.0.0")
                .environment("prod")
                .usedEndpoints(endpoints)
                .build();

        assertEquals("order-service", options.getConsumerId());
        assertEquals("1.0.0", options.getConsumerVersion());
        assertEquals("user-api", options.getSchemaId());
        assertEquals("prod", options.getEnvironment());
        assertEquals(1, options.getUsedEndpoints().size());
    }

    @Test
    @DisplayName("GeneratedFixture should have correct properties")
    void testGeneratedFixture() {
        GeneratedRequest request = new GeneratedRequest("GET", "/users", new HashMap<>(), null);
        GeneratedResponse response = new GeneratedResponse(200, new HashMap<>(), null);
        GeneratedFixture fixture = new GeneratedFixture(request, response);

        assertEquals("GET", fixture.getRequest().getMethod());
        assertEquals(200, fixture.getResponse().getStatusCode());
    }

    @Test
    @DisplayName("GeneratedRequest should have correct properties")
    void testGeneratedRequest() {
        Map<String, String> headers = new HashMap<>();
        headers.put("Content-Type", "application/json");
        Object body = new HashMap<String, Object>();

        GeneratedRequest request = new GeneratedRequest("POST", "/users", headers, body);

        assertEquals("POST", request.getMethod());
        assertEquals("/users", request.getPath());
        assertEquals("application/json", request.getHeaders().get("Content-Type"));
        assertNotNull(request.getBody());
    }

    @Test
    @DisplayName("GeneratedResponse should have correct properties")
    void testGeneratedResponse() {
        Map<String, String> headers = new HashMap<>();
        headers.put("Content-Type", "application/json");
        Object body = new HashMap<String, Object>();

        GeneratedResponse response = new GeneratedResponse(201, headers, body);

        assertEquals(201, response.getStatusCode());
        assertEquals("application/json", response.getHeaders().get("Content-Type"));
        assertNotNull(response.getBody());
    }

    // ============================================================================
    // useSchema tests
    // ============================================================================

    private SchemaMetadata buildSchemaMetadata() {
        return buildSchemaMetadata("1.2.0", true);
    }

    private SchemaMetadata buildSchemaMetadata(String version, boolean withOwnership) {
        SchemaMetadata.Builder b = SchemaMetadata.newBuilder()
                .setSchemaId("petstore")
                .setSchemaVersion(version)
                .setSchemaHash("abc123")
                .setRegisteredAt(1714000000L)
                .setUpdatedAt(1714000500L)
                .setOpenapiVersion("3.0.0")
                .setEndpointCount(7);
        if (withOwnership) {
            b.setOwnership(SchemaOwnership.newBuilder()
                    .setOwner("jane")
                    .setTeam("platform")
                    .setContactEmail("platform@example.com")
                    .setReadOnly(false)
                    .build());
        }
        return b.build();
    }

    @Test
    @DisplayName("useSchema resolves latest and returns SchemaInfo")
    void testUseSchemaResolvesLatest() {
        GetSchemaResponse response = GetSchemaResponse.newBuilder()
                .setFound(true)
                .setMetadata(buildSchemaMetadata())
                .build();
        when(mockStub.getSchema(any(GetSchemaRequest.class))).thenReturn(response);

        SchemaInfo info = validator.useSchema("petstore");

        ArgumentCaptor<GetSchemaRequest> captor = ArgumentCaptor.forClass(GetSchemaRequest.class);
        verify(mockStub).getSchema(captor.capture());
        assertEquals("petstore", captor.getValue().getSchemaId());
        assertEquals("", captor.getValue().getSchemaVersion());

        assertEquals("petstore", info.getSchemaId());
        assertEquals("1.2.0", info.getSchemaVersion());
        assertEquals(7, info.getEndpointCount());
        assertNotNull(info.getOwnership());
        assertEquals("jane", info.getOwnership().getOwner());
        assertEquals("platform", info.getOwnership().getTeam());
    }

    @Test
    @DisplayName("useSchema pins to an explicit version")
    void testUseSchemaPinsVersion() {
        GetSchemaResponse response = GetSchemaResponse.newBuilder()
                .setFound(true)
                .setMetadata(buildSchemaMetadata("1.0.0", false))
                .build();
        when(mockStub.getSchema(any(GetSchemaRequest.class))).thenReturn(response);

        SchemaInfo info = validator.useSchema("petstore", "1.0.0");

        ArgumentCaptor<GetSchemaRequest> captor = ArgumentCaptor.forClass(GetSchemaRequest.class);
        verify(mockStub).getSchema(captor.capture());
        assertEquals("1.0.0", captor.getValue().getSchemaVersion());
        assertEquals("1.0.0", info.getSchemaVersion());
    }

    @Test
    @DisplayName("useSchema throws when schema is not registered")
    void testUseSchemaNotFound() {
        GetSchemaResponse response = GetSchemaResponse.newBuilder().setFound(false).build();
        when(mockStub.getSchema(any(GetSchemaRequest.class))).thenReturn(response);

        IllegalArgumentException e = assertThrows(IllegalArgumentException.class,
                () -> validator.useSchema("nope"));
        assertTrue(e.getMessage().contains("'nope'"));
        assertTrue(e.getMessage().contains("not registered"));
    }

    @Test
    @DisplayName("useSchema error message includes version when pinning")
    void testUseSchemaNotFoundWithVersion() {
        GetSchemaResponse response = GetSchemaResponse.newBuilder().setFound(false).build();
        when(mockStub.getSchema(any(GetSchemaRequest.class))).thenReturn(response);

        IllegalArgumentException e = assertThrows(IllegalArgumentException.class,
                () -> validator.useSchema("petstore", "9.9.9"));
        assertTrue(e.getMessage().contains("'petstore@9.9.9'"));
    }

    @Test
    @DisplayName("useSchema propagates gRPC errors")
    void testUseSchemaPropagatesRpcError() {
        when(mockStub.getSchema(any(GetSchemaRequest.class)))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        assertThrows(IllegalStateException.class, () -> validator.useSchema("petstore"));
    }

    @Test
    @DisplayName("validate() sends schema_version after useSchema")
    void testValidateSendsVersionAfterUseSchema() {
        GetSchemaResponse getResponse = GetSchemaResponse.newBuilder()
                .setFound(true)
                .setMetadata(buildSchemaMetadata())
                .build();
        when(mockStub.getSchema(any(GetSchemaRequest.class))).thenReturn(getResponse);
        when(mockStub.validateInteraction(any(InteractionRequest.class)))
                .thenReturn(ValidationResult.newBuilder().setValid(true).build());

        validator.useSchema("petstore");
        validator.validate(
                ValidationRequest.builder().method("GET").path("/pet/1").build(),
                ValidationResponse.builder().statusCode(200).build());

        ArgumentCaptor<InteractionRequest> captor = ArgumentCaptor.forClass(InteractionRequest.class);
        verify(mockStub).validateInteraction(captor.capture());
        assertEquals("petstore", captor.getValue().getSchemaId());
        assertEquals("1.2.0", captor.getValue().getSchemaVersion());
    }

    @Test
    @DisplayName("SchemaOwnership getters and toString")
    void testSchemaOwnershipGetters() {
        io.github.sahina.sdk.SchemaOwnership o = new io.github.sahina.sdk.SchemaOwnership(
                "jane", "platform", "platform@example.com", true);
        assertEquals("jane", o.getOwner());
        assertEquals("platform", o.getTeam());
        assertEquals("platform@example.com", o.getContactEmail());
        assertTrue(o.isReadOnly());
        assertTrue(o.toString().contains("jane"));

        io.github.sahina.sdk.SchemaOwnership nullCoalesced =
                new io.github.sahina.sdk.SchemaOwnership(null, null, null, false);
        assertEquals("", nullCoalesced.getOwner());
        assertEquals("", nullCoalesced.getTeam());
        assertEquals("", nullCoalesced.getContactEmail());
        assertFalse(nullCoalesced.isReadOnly());
    }

    @Test
    @DisplayName("SchemaInfo getters and toString")
    void testSchemaInfoGetters() {
        io.github.sahina.sdk.SchemaOwnership ownership =
                new io.github.sahina.sdk.SchemaOwnership("jane", "platform", "", false);
        SchemaInfo info = new SchemaInfo("petstore", "1.2.0", "abc123", 100L, 200L, "3.0.0", 7, ownership);
        assertEquals("petstore", info.getSchemaId());
        assertEquals("1.2.0", info.getSchemaVersion());
        assertEquals("abc123", info.getSchemaHash());
        assertEquals(100L, info.getRegisteredAt());
        assertEquals(200L, info.getUpdatedAt());
        assertEquals("3.0.0", info.getOpenapiVersion());
        assertEquals(7, info.getEndpointCount());
        assertSame(ownership, info.getOwnership());
        assertTrue(info.toString().contains("petstore"));

        SchemaInfo nullCoalesced = new SchemaInfo("x", "1.0", null, 0L, 0L, null, 0, null);
        assertEquals("", nullCoalesced.getSchemaHash());
        assertEquals("", nullCoalesced.getOpenapiVersion());
        assertNull(nullCoalesced.getOwnership());
    }

    @Test
    @DisplayName("validate() does NOT send schema_version after registerSchema only")
    void testValidateDoesNotSendVersionAfterRegisterSchema() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        when(mockStub.validateInteraction(any(InteractionRequest.class)))
                .thenReturn(ValidationResult.newBuilder().setValid(true).build());

        validator.registerSchema("test-schema", "path/to/schema.json");
        validator.validate(
                ValidationRequest.builder().method("GET").path("/pet/1").build(),
                ValidationResponse.builder().statusCode(200).build());

        ArgumentCaptor<InteractionRequest> captor = ArgumentCaptor.forClass(InteractionRequest.class);
        verify(mockStub).validateInteraction(captor.capture());
        assertEquals("test-schema", captor.getValue().getSchemaId());
        assertEquals("", captor.getValue().getSchemaVersion());
    }
}