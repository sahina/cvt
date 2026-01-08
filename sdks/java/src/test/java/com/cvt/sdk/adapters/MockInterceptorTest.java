package com.cvt.sdk.adapters;

import com.cvt.proto.ContractValidatorGrpc;
import com.cvt.proto.GenerateFixtureRequest;
import com.cvt.proto.GenerateFixtureResponse;
import com.cvt.proto.RegisterSchemaRequest;
import com.cvt.proto.RegisterSchemaResponse;
import com.cvt.sdk.ContractValidator;
import com.cvt.sdk.GenerateOptions;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.io.IOException;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class MockInterceptorTest {

    @Mock
    private ContractValidatorGrpc.ContractValidatorBlockingStub mockStub;

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
    void setUp() {
        validator = new TestableContractValidator(mockStub);
    }

    private void setupDefaultMocks() throws IOException {
        when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
        when(mockStub.generateFixture(any(GenerateFixtureRequest.class)))
                .thenReturn(GenerateFixtureResponse.newBuilder()
                        .setSuccess(true)
                        .setResponse(com.cvt.proto.GeneratedResponse.newBuilder()
                                .setStatusCode(200)
                                .putHeaders("content-type", "application/json")
                                .setBody("{\"id\":\"123\",\"name\":\"Test User\"}"))
                        .build());

        validator.registerSchema("test", "schema.json");
    }

    @Nested
    @DisplayName("MockInterceptor creation")
    class MockInterceptorCreation {

        @Test
        @DisplayName("Should create interceptor instance")
        void testCreateInterceptor() {
            MockInterceptor interceptor = new MockInterceptor(validator);
            assertNotNull(interceptor);
        }

        @Test
        @DisplayName("Should return empty interactions initially")
        void testEmptyInteractionsInitially() {
            MockInterceptor interceptor = new MockInterceptor(validator);
            assertTrue(interceptor.getInteractions().isEmpty());
        }

        @Test
        @DisplayName("Should create mock client with static method")
        void testCreateMockClient() {
            OkHttpClient mockClient = MockInterceptor.createMockClient(validator);
            assertNotNull(mockClient);
            assertEquals(1, mockClient.interceptors().size());
        }

        @Test
        @DisplayName("Should create mock client with config")
        void testCreateMockClientWithConfig() {
            MockConfig config = MockConfig.builder().cache().build();
            OkHttpClient mockClient = MockInterceptor.createMockClient(validator, config);
            assertNotNull(mockClient);
        }
    }

    @Nested
    @DisplayName("Mock fetch functionality")
    class MockFetchFunctionality {

        private MockInterceptor mockInterceptor;
        private OkHttpClient client;

        @BeforeEach
        void setUpFetch() throws IOException {
            setupDefaultMocks();
            mockInterceptor = new MockInterceptor(validator);
            client = new OkHttpClient.Builder()
                    .addInterceptor(mockInterceptor)
                    .build();
        }

        @Test
        @DisplayName("Should return schema-generated response for GET")
        void testGetRequest() throws IOException {
            Request request = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
                assertEquals("application/json", response.header("content-type"));

                String body = response.body().string();
                assertTrue(body.contains("\"id\":\"123\""));
                assertTrue(body.contains("\"name\":\"Test User\""));
            }
        }

        @Test
        @DisplayName("Should call generateResponse with correct method and path")
        void testGenerateResponseCalled() throws IOException {
            Request request = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            verify(mockStub).generateFixture(argThat(req ->
                    req.getMethod().equals("GET") && req.getPath().equals("/users/123")));
        }

        @Test
        @DisplayName("Should handle POST requests")
        void testPostRequest() throws IOException {
            Request request = new Request.Builder()
                    .url("http://mock.api/users")
                    .post(RequestBody.create("{\"name\":\"New User\"}",
                            okhttp3.MediaType.parse("application/json")))
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            verify(mockStub).generateFixture(argThat(req ->
                    req.getMethod().equals("POST") && req.getPath().equals("/users")));
        }

        @Test
        @DisplayName("Should capture request in interactions")
        void testCaptureRequest() throws IOException {
            Request request = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            List<CapturedInteraction> interactions = mockInterceptor.getInteractions();
            assertEquals(1, interactions.size());
            assertEquals("GET", interactions.get(0).getMethod());
            assertEquals("/users/123", interactions.get(0).getPath());
        }

        @Test
        @DisplayName("Should capture response in interactions")
        void testCaptureResponse() throws IOException {
            Request request = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            List<CapturedInteraction> interactions = mockInterceptor.getInteractions();
            assertEquals(1, interactions.size());
            assertEquals(200, interactions.get(0).getStatusCode());
            assertTrue(interactions.get(0).getResponseBody().contains("Test User"));
        }

        @Test
        @DisplayName("Should include query string in path")
        void testQueryString() throws IOException {
            Request request = new Request.Builder()
                    .url("http://mock.api/users?status=active&limit=10")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            verify(mockStub).generateFixture(argThat(req ->
                    req.getPath().equals("/users?status=active&limit=10")));
        }
    }

    @Nested
    @DisplayName("Caching functionality")
    class CachingFunctionality {

        private MockInterceptor mockInterceptor;
        private OkHttpClient client;

        @BeforeEach
        void setUpCaching() throws IOException {
            setupDefaultMocks();
            mockInterceptor = new MockInterceptor(validator);
            client = new OkHttpClient.Builder()
                    .addInterceptor(mockInterceptor)
                    .build();
        }

        @Test
        @DisplayName("Should not cache by default")
        void testNoCacheByDefault() throws IOException {
            for (int i = 0; i < 2; i++) {
                Request request = new Request.Builder()
                        .url("http://mock.api/users/123")
                        .get()
                        .build();
                try (Response response = client.newCall(request).execute()) {
                    assertEquals(200, response.code());
                }
            }

            verify(mockStub, times(2)).generateFixture(any());
        }

        @Test
        @DisplayName("Should cache responses when enabled")
        void testCacheEnabled() throws IOException {
            mockInterceptor.withConfig(MockConfig.builder().cache().build());

            for (int i = 0; i < 2; i++) {
                Request request = new Request.Builder()
                        .url("http://mock.api/users/123")
                        .get()
                        .build();
                try (Response response = client.newCall(request).execute()) {
                    assertEquals(200, response.code());
                }
            }

            verify(mockStub, times(1)).generateFixture(any());
        }

        @Test
        @DisplayName("Should cache by method+path")
        void testCacheByMethodPath() throws IOException {
            mockInterceptor.withConfig(MockConfig.builder().cache().build());

            Request request1 = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();
            Request request2 = new Request.Builder()
                    .url("http://mock.api/users/456")
                    .get()
                    .build();

            try (Response response = client.newCall(request1).execute()) {
                assertEquals(200, response.code());
            }
            try (Response response = client.newCall(request2).execute()) {
                assertEquals(200, response.code());
            }

            verify(mockStub, times(2)).generateFixture(any());
        }

        @Test
        @DisplayName("Should clear cache")
        void testClearCache() throws IOException {
            mockInterceptor.withConfig(MockConfig.builder().cache().build());

            Request request = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            mockInterceptor.clearCache();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            verify(mockStub, times(2)).generateFixture(any());
        }
    }

    @Nested
    @DisplayName("Interactions management")
    class InteractionsManagement {

        private MockInterceptor mockInterceptor;
        private OkHttpClient client;

        @BeforeEach
        void setUpInteractions() throws IOException {
            setupDefaultMocks();
            mockInterceptor = new MockInterceptor(validator);
            client = new OkHttpClient.Builder()
                    .addInterceptor(mockInterceptor)
                    .build();
        }

        @Test
        @DisplayName("Should return copy of interactions")
        void testReturnCopyOfInteractions() throws IOException {
            Request request = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            List<CapturedInteraction> interactions1 = mockInterceptor.getInteractions();
            List<CapturedInteraction> interactions2 = mockInterceptor.getInteractions();

            assertNotSame(interactions1, interactions2);
            assertEquals(interactions1, interactions2);
        }

        @Test
        @DisplayName("Should clear interactions")
        void testClearInteractions() throws IOException {
            Request request = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            assertEquals(1, mockInterceptor.getInteractions().size());

            mockInterceptor.clearInteractions();

            assertTrue(mockInterceptor.getInteractions().isEmpty());
        }

        @Test
        @DisplayName("Should record timestamp")
        void testRecordTimestamp() throws IOException {
            long before = System.currentTimeMillis();

            Request request = new Request.Builder()
                    .url("http://mock.api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            long after = System.currentTimeMillis();

            List<CapturedInteraction> interactions = mockInterceptor.getInteractions();
            assertTrue(interactions.get(0).getTimestamp() >= before);
            assertTrue(interactions.get(0).getTimestamp() <= after);
        }
    }

    @Nested
    @DisplayName("Path filtering")
    class PathFiltering {

        @Test
        @DisplayName("Should exclude paths matching excludePath")
        void testExcludePath() throws IOException {
            setupDefaultMocks();
            MockInterceptor mockInterceptor = new MockInterceptor(validator)
                    .withConfig(MockConfig.builder()
                            .excludePath("/health.*")
                            .build());

            OkHttpClient client = new OkHttpClient.Builder()
                    .addInterceptor(mockInterceptor)
                    .build();

            Request request = new Request.Builder()
                    .url("http://mock.api/health")
                    .get()
                    .build();

            assertThrows(IOException.class, () -> {
                client.newCall(request).execute().close();
            });
        }

        @Test
        @DisplayName("Should only include paths matching includePath")
        void testIncludePath() throws IOException {
            setupDefaultMocks();
            MockInterceptor mockInterceptor = new MockInterceptor(validator)
                    .withConfig(MockConfig.builder()
                            .includePath("/api/.*")
                            .build());

            OkHttpClient client = new OkHttpClient.Builder()
                    .addInterceptor(mockInterceptor)
                    .build();

            // Included path works
            Request request1 = new Request.Builder()
                    .url("http://mock.api/api/users/123")
                    .get()
                    .build();

            try (Response response = client.newCall(request1).execute()) {
                assertEquals(200, response.code());
            }

            // Non-included path fails
            Request request2 = new Request.Builder()
                    .url("http://mock.api/other/path")
                    .get()
                    .build();

            assertThrows(IOException.class, () -> {
                client.newCall(request2).execute().close();
            });
        }
    }

    @Nested
    @DisplayName("Generate options")
    class GenerateOptionsTests {

        @Test
        @DisplayName("Should pass generateOptions to generateResponse")
        void testGenerateOptions() throws IOException {
            setupDefaultMocks();

            GenerateOptions options = GenerateOptions.builder()
                    .statusCode(201)
                    .useExamples(true)
                    .build();

            MockInterceptor mockInterceptor = new MockInterceptor(validator)
                    .withConfig(MockConfig.builder()
                            .generateOptions(options)
                            .build());

            OkHttpClient client = new OkHttpClient.Builder()
                    .addInterceptor(mockInterceptor)
                    .build();

            Request request = new Request.Builder()
                    .url("http://mock.api/users")
                    .post(RequestBody.create("{}", okhttp3.MediaType.parse("application/json")))
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(200, response.code());
            }

            verify(mockStub).generateFixture(argThat(req ->
                    req.getStatusCode() == 201 && req.getUseExamples()));
        }
    }

    @Nested
    @DisplayName("Custom status codes")
    class CustomStatusCodes {

        @Test
        @DisplayName("Should handle different status codes")
        void testDifferentStatusCode() throws IOException {
            when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                    .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
            when(mockStub.generateFixture(any(GenerateFixtureRequest.class)))
                    .thenReturn(GenerateFixtureResponse.newBuilder()
                            .setSuccess(true)
                            .setResponse(com.cvt.proto.GeneratedResponse.newBuilder()
                                    .setStatusCode(201)
                                    .putHeaders("content-type", "application/json")
                                    .setBody("{\"id\":\"new-123\"}"))
                            .build());

            validator.registerSchema("test", "schema.json");

            MockInterceptor mockInterceptor = new MockInterceptor(validator);
            OkHttpClient client = new OkHttpClient.Builder()
                    .addInterceptor(mockInterceptor)
                    .build();

            Request request = new Request.Builder()
                    .url("http://mock.api/users")
                    .post(RequestBody.create("{}", okhttp3.MediaType.parse("application/json")))
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(201, response.code());
                assertEquals("Created", response.message());
            }
        }

        @Test
        @DisplayName("Should handle 404 status")
        void testNotFoundStatus() throws IOException {
            when(mockStub.registerSchema(any(RegisterSchemaRequest.class)))
                    .thenReturn(RegisterSchemaResponse.newBuilder().setSuccess(true).build());
            when(mockStub.generateFixture(any(GenerateFixtureRequest.class)))
                    .thenReturn(GenerateFixtureResponse.newBuilder()
                            .setSuccess(true)
                            .setResponse(com.cvt.proto.GeneratedResponse.newBuilder()
                                    .setStatusCode(404)
                                    .putHeaders("content-type", "application/json")
                                    .setBody("{\"error\":\"Not Found\"}"))
                            .build());

            validator.registerSchema("test", "schema.json");

            MockInterceptor mockInterceptor = new MockInterceptor(validator);
            OkHttpClient client = new OkHttpClient.Builder()
                    .addInterceptor(mockInterceptor)
                    .build();

            Request request = new Request.Builder()
                    .url("http://mock.api/users/unknown")
                    .get()
                    .build();

            try (Response response = client.newCall(request).execute()) {
                assertEquals(404, response.code());
                assertEquals("Not Found", response.message());
            }
        }
    }
}
