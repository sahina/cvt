package io.github.sahina.sdk.adapters;

import io.github.sahina.proto.ContractValidatorGrpc;
import io.github.sahina.proto.InteractionRequest;
import io.github.sahina.proto.RegisterSchemaRequest;
import io.github.sahina.proto.RegisterSchemaResponse;
import io.github.sahina.sdk.ContractValidator;
import io.github.sahina.sdk.ValidationResult;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.mockwebserver.MockResponse;
import okhttp3.mockwebserver.MockWebServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.io.IOException;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class OkHttpContractAdapterTest {

    @Mock
    private ContractValidatorGrpc.ContractValidatorBlockingStub mockStub;

    private MockWebServer server;
    private OkHttpClient client;
    private OkHttpContractAdapter adapter;
    private ContractValidator validator;

    // Testable validator that overrides file operations
    private static class TestableContractValidator extends ContractValidator {
        public TestableContractValidator(ContractValidatorGrpc.ContractValidatorBlockingStub stub) {
            super(stub);
        }

        @Override
        protected String readSchemaFromFile(String filePath) {
            return "{\"openapi\":\"3.0.0\"}";
        }
    }

    @BeforeEach
    void setUp() throws IOException {
        server = new MockWebServer();
        server.start();

        validator = new TestableContractValidator(mockStub);
        adapter = new OkHttpContractAdapter(validator);

        client = new OkHttpClient.Builder()
                .addInterceptor(adapter)
                .build();
    }

    @AfterEach
    void tearDown() throws IOException {
        server.shutdown();
    }

    @Test
    @DisplayName("Should capture interaction")
    void testCaptureInteraction() throws Exception {
        // Setup mock responses
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        when(mockStub.validateInteraction(any(InteractionRequest.class)))
                .thenReturn(io.github.sahina.proto.ValidationResult.newBuilder().setValid(true).build());

        validator.registerSchema("test", "schema.json");

        server.enqueue(new MockResponse()
                .setResponseCode(200)
                .setBody("{\"users\":[]}"));

        Request request = new Request.Builder()
                .url(server.url("/users"))
                .get()
                .build();

        try (Response response = client.newCall(request).execute()) {
            assertEquals(200, response.code());
        }

        List<CapturedInteraction> interactions = adapter.getInteractions();
        assertEquals(1, interactions.size());

        CapturedInteraction captured = interactions.get(0);
        assertEquals("GET", captured.getMethod());
        assertEquals("/users", captured.getPath());
        assertEquals(200, captured.getStatusCode());
        assertTrue(captured.isValid());
    }

    @Test
    @DisplayName("Should exclude paths based on config")
    void testExcludePaths() throws Exception {
        adapter.withConfig(AdapterConfig.builder()
                .excludePath("/health.*")
                .build());

        server.enqueue(new MockResponse().setResponseCode(200).setBody("OK"));

        Request request = new Request.Builder()
                .url(server.url("/health"))
                .get()
                .build();

        try (Response response = client.newCall(request).execute()) {
            assertEquals(200, response.code());
        }

        // Should not capture health endpoint
        assertTrue(adapter.getInteractions().isEmpty());
    }

    @Test
    @DisplayName("Should clear interactions")
    void testClearInteractions() throws Exception {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        when(mockStub.validateInteraction(any(InteractionRequest.class)))
                .thenReturn(io.github.sahina.proto.ValidationResult.newBuilder().setValid(true).build());

        validator.registerSchema("test", "schema.json");

        server.enqueue(new MockResponse().setResponseCode(200).setBody("{}"));

        Request request = new Request.Builder()
                .url(server.url("/users"))
                .get()
                .build();

        try (Response response = client.newCall(request).execute()) {
            assertEquals(200, response.code());
        }

        assertEquals(1, adapter.getInteractions().size());

        adapter.clearInteractions();
        assertTrue(adapter.getInteractions().isEmpty());
    }

    @Test
    @DisplayName("Should return invalid interactions")
    void testGetInvalidInteractions() throws Exception {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());

        // First call valid, second invalid
        when(mockStub.validateInteraction(any(InteractionRequest.class)))
                .thenReturn(io.github.sahina.proto.ValidationResult.newBuilder().setValid(true).build())
                .thenReturn(io.github.sahina.proto.ValidationResult.newBuilder()
                        .setValid(false)
                        .addErrors("Missing field")
                        .build());

        validator.registerSchema("test", "schema.json");

        server.enqueue(new MockResponse().setResponseCode(200).setBody("{}"));
        server.enqueue(new MockResponse().setResponseCode(400).setBody("{\"error\":\"bad\"}"));

        // First request (valid)
        try (Response response = client.newCall(new Request.Builder()
                .url(server.url("/users"))
                .get()
                .build()).execute()) {
            assertEquals(200, response.code());
        }

        // Second request (invalid)
        try (Response response = client.newCall(new Request.Builder()
                .url(server.url("/invalid"))
                .get()
                .build()).execute()) {
            assertEquals(400, response.code());
        }

        assertEquals(2, adapter.getInteractions().size());
        assertEquals(1, adapter.getInvalidInteractions().size());
    }

    @Test
    @DisplayName("Should work without auto-validation")
    void testNoAutoValidate() throws Exception {
        adapter.withConfig(AdapterConfig.builder()
                .autoValidate(false)
                .build());

        server.enqueue(new MockResponse().setResponseCode(200).setBody("{}"));

        Request request = new Request.Builder()
                .url(server.url("/users"))
                .get()
                .build();

        try (Response response = client.newCall(request).execute()) {
            assertEquals(200, response.code());
        }

        List<CapturedInteraction> interactions = adapter.getInteractions();
        assertEquals(1, interactions.size());
        assertNull(interactions.get(0).getValidationResult());
    }

    @Test
    @DisplayName("Should attach to client builder")
    void testAttach() {
        OkHttpClient.Builder builder = new OkHttpClient.Builder();
        adapter.attach(builder);

        OkHttpClient builtClient = builder.build();
        assertEquals(1, builtClient.interceptors().size());
    }
}
