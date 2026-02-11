package io.github.sahina.sdk;

import io.github.sahina.sdk.adapters.CapturedInteraction;
import org.junit.jupiter.api.Test;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for auto-registration functionality.
 */
public class AutoRegisterTest {

    // =========================================================================
    // extractSchemaIdFromUrl tests
    // =========================================================================

    @Test
    void extractSchemaIdFromUrl_mockPrefix() {
        assertEquals("user-api", AutoRegisterUtils.extractSchemaIdFromUrl("http://mock.user-api/users/123"));
    }

    @Test
    void extractSchemaIdFromUrl_mockPrefixWithSubdomain() {
        assertEquals("my-service", AutoRegisterUtils.extractSchemaIdFromUrl("http://mock.my-service/api/v1/items"));
    }

    @Test
    void extractSchemaIdFromUrl_noMockPrefix() {
        assertEquals("api.example.com", AutoRegisterUtils.extractSchemaIdFromUrl("http://api.example.com/users"));
    }

    @Test
    void extractSchemaIdFromUrl_httpsUrl() {
        assertEquals("secure-api", AutoRegisterUtils.extractSchemaIdFromUrl("https://mock.secure-api/data"));
    }

    @Test
    void extractSchemaIdFromUrl_urlWithPort() {
        assertEquals("test-api", AutoRegisterUtils.extractSchemaIdFromUrl("http://mock.test-api:8080/endpoint"));
    }

    @Test
    void extractSchemaIdFromUrl_invalidUrl() {
        assertNull(AutoRegisterUtils.extractSchemaIdFromUrl("not a url"));
    }

    @Test
    void extractSchemaIdFromUrl_nullUrl() {
        assertNull(AutoRegisterUtils.extractSchemaIdFromUrl(null));
    }

    @Test
    void extractSchemaIdFromUrl_emptyUrl() {
        assertNull(AutoRegisterUtils.extractSchemaIdFromUrl(""));
    }

    // =========================================================================
    // extractFieldsFromBody tests
    // =========================================================================

    @Test
    void extractFieldsFromBody_nullBody() {
        assertTrue(AutoRegisterUtils.extractFieldsFromBody(null).isEmpty());
    }

    @Test
    void extractFieldsFromBody_flatObject() {
        // Simulate Gson parsing result
        java.util.Map<String, Object> body = new java.util.LinkedHashMap<>();
        body.put("id", "123");
        body.put("name", "John");
        body.put("email", "john@example.com");

        List<String> fields = AutoRegisterUtils.extractFieldsFromBody(body);
        assertTrue(fields.contains("id"));
        assertTrue(fields.contains("name"));
        assertTrue(fields.contains("email"));
    }

    @Test
    void extractFieldsFromBody_nestedObject() {
        java.util.Map<String, Object> address = new java.util.LinkedHashMap<>();
        address.put("city", "NYC");
        address.put("zip", "10001");

        java.util.Map<String, Object> body = new java.util.LinkedHashMap<>();
        body.put("id", "123");
        body.put("address", address);

        List<String> fields = AutoRegisterUtils.extractFieldsFromBody(body);
        assertTrue(fields.contains("id"));
        assertTrue(fields.contains("address"));
        assertTrue(fields.contains("address.city"));
        assertTrue(fields.contains("address.zip"));
    }

    @Test
    void extractFieldsFromBody_deeplyNested() {
        java.util.Map<String, Object> name = new java.util.LinkedHashMap<>();
        name.put("name", "John");

        java.util.Map<String, Object> profile = new java.util.LinkedHashMap<>();
        profile.put("profile", name);

        java.util.Map<String, Object> body = new java.util.LinkedHashMap<>();
        body.put("user", profile);

        List<String> fields = AutoRegisterUtils.extractFieldsFromBody(body);
        assertTrue(fields.contains("user"));
        assertTrue(fields.contains("user.profile"));
        assertTrue(fields.contains("user.profile.name"));
    }

    @Test
    void extractFieldsFromBody_arrayWithObjects() {
        java.util.Map<String, Object> item = new java.util.LinkedHashMap<>();
        item.put("id", "1");
        item.put("name", "Item 1");

        List<Object> body = Arrays.asList(item);

        List<String> fields = AutoRegisterUtils.extractFieldsFromBody(body);
        assertTrue(fields.contains("id"));
        assertTrue(fields.contains("name"));
    }

    @Test
    void extractFieldsFromBody_emptyArray() {
        List<Object> body = Collections.emptyList();
        assertTrue(AutoRegisterUtils.extractFieldsFromBody(body).isEmpty());
    }

    @Test
    void extractFieldsFromBody_emptyObject() {
        java.util.Map<String, Object> body = new java.util.LinkedHashMap<>();
        assertTrue(AutoRegisterUtils.extractFieldsFromBody(body).isEmpty());
    }

    @Test
    void extractFieldsFromBody_withPrefix() {
        java.util.Map<String, Object> body = new java.util.LinkedHashMap<>();
        body.put("city", "NYC");

        List<String> fields = AutoRegisterUtils.extractFieldsFromBody(body, "address");
        assertTrue(fields.contains("address.city"));
    }

    // =========================================================================
    // normalizePathForEndpoint tests
    // =========================================================================

    @Test
    void normalizePathForEndpoint_simplePath() {
        assertEquals("/users/123", AutoRegisterUtils.normalizePathForEndpoint("/users/123"));
    }

    @Test
    void normalizePathForEndpoint_pathWithQueryString() {
        assertEquals("/users", AutoRegisterUtils.normalizePathForEndpoint("/users?page=1&limit=10"));
    }

    @Test
    void normalizePathForEndpoint_fullUrl() {
        assertEquals("/users/123", AutoRegisterUtils.normalizePathForEndpoint("http://mock.user-api/users/123"));
    }

    @Test
    void normalizePathForEndpoint_fullUrlWithQuery() {
        assertEquals("/users", AutoRegisterUtils.normalizePathForEndpoint("http://mock.user-api/users?active=true"));
    }

    @Test
    void normalizePathForEndpoint_httpsUrl() {
        assertEquals("/data/items", AutoRegisterUtils.normalizePathForEndpoint("https://mock.api/data/items"));
    }

    // =========================================================================
    // extractSchemaIdFromInteractions tests
    // =========================================================================

    @Test
    void extractSchemaIdFromInteractions_singleSchema() {
        CapturedInteraction interaction = new CapturedInteraction(
                "GET",
                "http://mock.user-api/users/123",
                null,
                null,
                200,
                null,
                "{\"id\": \"123\"}",
                null
        );

        AutoRegisterUtils.SchemaIdResult result = AutoRegisterUtils.extractSchemaIdFromInteractions(
                Collections.singletonList(interaction)
        );
        assertFalse(result.hasError());
        assertEquals("user-api", result.getSchemaId());
    }

    @Test
    void extractSchemaIdFromInteractions_multipleInteractionsSameSchema() {
        CapturedInteraction interaction1 = new CapturedInteraction(
                "GET",
                "http://mock.user-api/users/123",
                null,
                null,
                200,
                null,
                "{\"id\": \"123\"}",
                null
        );
        CapturedInteraction interaction2 = new CapturedInteraction(
                "POST",
                "http://mock.user-api/users",
                null,
                null,
                201,
                null,
                "{\"id\": \"789\"}",
                null
        );

        AutoRegisterUtils.SchemaIdResult result = AutoRegisterUtils.extractSchemaIdFromInteractions(
                Arrays.asList(interaction1, interaction2)
        );
        assertFalse(result.hasError());
        assertEquals("user-api", result.getSchemaId());
    }

    @Test
    void extractSchemaIdFromInteractions_multipleDifferentSchemas() {
        CapturedInteraction interaction1 = new CapturedInteraction(
                "GET",
                "http://mock.user-api/users/123",
                null,
                null,
                200,
                null,
                null,
                null
        );
        CapturedInteraction interaction2 = new CapturedInteraction(
                "GET",
                "http://mock.order-api/orders/456",
                null,
                null,
                200,
                null,
                null,
                null
        );

        AutoRegisterUtils.SchemaIdResult result = AutoRegisterUtils.extractSchemaIdFromInteractions(
                Arrays.asList(interaction1, interaction2)
        );
        assertTrue(result.hasError());
        assertTrue(result.getError().contains("multiple schemas detected"));
        assertNull(result.getSchemaId());
    }

    @Test
    void extractSchemaIdFromInteractions_noUrlsInPaths() {
        CapturedInteraction interaction = new CapturedInteraction(
                "GET",
                "/users/123",
                null,
                null,
                200,
                null,
                null,
                null
        );

        AutoRegisterUtils.SchemaIdResult result = AutoRegisterUtils.extractSchemaIdFromInteractions(
                Collections.singletonList(interaction)
        );
        assertTrue(result.hasError());
        assertTrue(result.getError().contains("could not extract schemaId"));
        assertNull(result.getSchemaId());
    }

    // =========================================================================
    // mergeInteractionsToEndpoints tests
    // =========================================================================

    @Test
    void mergeInteractionsToEndpoints_mergeInteractions() {
        CapturedInteraction interaction1 = new CapturedInteraction(
                "GET",
                "http://mock.user-api/users/123",
                null,
                null,
                200,
                null,
                "{\"id\": \"123\", \"name\": \"John\"}",
                null
        );
        CapturedInteraction interaction2 = new CapturedInteraction(
                "POST",
                "http://mock.user-api/users",
                null,
                null,
                201,
                null,
                "{\"id\": \"789\"}",
                null
        );

        List<EndpointUsage> endpoints = AutoRegisterUtils.mergeInteractionsToEndpoints(
                Arrays.asList(interaction1, interaction2)
        );
        assertEquals(2, endpoints.size());

        EndpointUsage getEndpoint = endpoints.stream()
                .filter(ep -> "GET".equals(ep.getMethod()) && "/users/123".equals(ep.getPath()))
                .findFirst()
                .orElse(null);
        assertNotNull(getEndpoint);
        assertTrue(getEndpoint.getUsedFields().contains("id"));
        assertTrue(getEndpoint.getUsedFields().contains("name"));

        EndpointUsage postEndpoint = endpoints.stream()
                .filter(ep -> "POST".equals(ep.getMethod()) && "/users".equals(ep.getPath()))
                .findFirst()
                .orElse(null);
        assertNotNull(postEndpoint);
        assertTrue(postEndpoint.getUsedFields().contains("id"));
    }

    @Test
    void mergeInteractionsToEndpoints_mergeFieldsFromDuplicateEndpoints() {
        CapturedInteraction interaction1 = new CapturedInteraction(
                "GET",
                "http://mock.user-api/users/123",
                null,
                null,
                200,
                null,
                "{\"id\": \"123\", \"name\": \"John\"}",
                null
        );
        CapturedInteraction interaction2 = new CapturedInteraction(
                "GET",
                "http://mock.user-api/users/123",
                null,
                null,
                200,
                null,
                "{\"id\": \"123\", \"email\": \"john@example.com\"}",
                null
        );

        List<EndpointUsage> endpoints = AutoRegisterUtils.mergeInteractionsToEndpoints(
                Arrays.asList(interaction1, interaction2)
        );
        assertEquals(1, endpoints.size());

        EndpointUsage endpoint = endpoints.get(0);
        assertTrue(endpoint.getUsedFields().contains("id"));
        assertTrue(endpoint.getUsedFields().contains("name"));
        assertTrue(endpoint.getUsedFields().contains("email"));
    }

    // =========================================================================
    // buildConsumerFromInteractions tests
    // =========================================================================

    private AutoRegisterConfig validConfig() {
        return AutoRegisterConfig.builder()
                .consumerId("test-service")
                .consumerVersion("1.0.0")
                .environment("dev")
                .schemaVersion("1.0.0")
                .build();
    }

    private List<CapturedInteraction> validInteractions() {
        CapturedInteraction interaction = new CapturedInteraction(
                "GET",
                "http://mock.user-api/users/123",
                null,
                null,
                200,
                null,
                "{\"id\": \"123\", \"name\": \"John\", \"email\": \"john@example.com\"}",
                null
        );
        return Collections.singletonList(interaction);
    }

    @Test
    void buildConsumerFromInteractions_success() {
        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                validInteractions(),
                validConfig()
        );

        assertFalse(result.hasError());
        assertNotNull(result.getOptions());
        assertEquals("test-service", result.getOptions().getConsumerId());
        assertEquals("1.0.0", result.getOptions().getConsumerVersion());
        assertEquals("user-api", result.getOptions().getSchemaId());
        assertEquals("1.0.0", result.getOptions().getSchemaVersion());
        assertEquals("dev", result.getOptions().getEnvironment());
        assertEquals(1, result.getOptions().getUsedEndpoints().size());
    }

    @Test
    void buildConsumerFromInteractions_missingConsumerId() {
        AutoRegisterConfig config = AutoRegisterConfig.builder()
                .consumerId("")
                .consumerVersion("1.0.0")
                .environment("dev")
                .schemaVersion("1.0.0")
                .build();

        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                validInteractions(),
                config
        );

        assertTrue(result.hasError());
        assertEquals("consumerId is required", result.getError());
        assertNull(result.getOptions());
    }

    @Test
    void buildConsumerFromInteractions_missingConsumerVersion() {
        AutoRegisterConfig config = AutoRegisterConfig.builder()
                .consumerId("test-service")
                .consumerVersion("")
                .environment("dev")
                .schemaVersion("1.0.0")
                .build();

        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                validInteractions(),
                config
        );

        assertTrue(result.hasError());
        assertEquals("consumerVersion is required", result.getError());
        assertNull(result.getOptions());
    }

    @Test
    void buildConsumerFromInteractions_missingEnvironment() {
        AutoRegisterConfig config = AutoRegisterConfig.builder()
                .consumerId("test-service")
                .consumerVersion("1.0.0")
                .environment("")
                .schemaVersion("1.0.0")
                .build();

        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                validInteractions(),
                config
        );

        assertTrue(result.hasError());
        assertEquals("environment is required", result.getError());
        assertNull(result.getOptions());
    }

    @Test
    void buildConsumerFromInteractions_missingSchemaVersion() {
        AutoRegisterConfig config = AutoRegisterConfig.builder()
                .consumerId("test-service")
                .consumerVersion("1.0.0")
                .environment("dev")
                .schemaVersion("")
                .build();

        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                validInteractions(),
                config
        );

        assertTrue(result.hasError());
        assertEquals("schemaVersion is required", result.getError());
        assertNull(result.getOptions());
    }

    @Test
    void buildConsumerFromInteractions_emptyInteractions() {
        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                Collections.emptyList(),
                validConfig()
        );

        assertTrue(result.hasError());
        assertEquals("no interactions to register", result.getError());
        assertNull(result.getOptions());
    }

    @Test
    void buildConsumerFromInteractions_nullInteractions() {
        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                null,
                validConfig()
        );

        assertTrue(result.hasError());
        assertEquals("no interactions to register", result.getError());
        assertNull(result.getOptions());
    }

    @Test
    void buildConsumerFromInteractions_explicitSchemaIdOverride() {
        CapturedInteraction interaction = new CapturedInteraction(
                "GET",
                "/users/123",
                null,
                null,
                200,
                null,
                "{\"id\": \"123\"}",
                null
        );

        AutoRegisterConfig config = AutoRegisterConfig.builder()
                .consumerId("test-service")
                .consumerVersion("1.0.0")
                .environment("dev")
                .schemaVersion("1.0.0")
                .schemaId("my-custom-schema")
                .build();

        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                Collections.singletonList(interaction),
                config
        );

        assertFalse(result.hasError());
        assertEquals("my-custom-schema", result.getOptions().getSchemaId());
    }

    @Test
    void buildConsumerFromInteractions_nestedResponseFields() {
        CapturedInteraction interaction = new CapturedInteraction(
                "GET",
                "http://mock.user-api/users/123",
                null,
                null,
                200,
                null,
                "{\"id\": \"123\", \"address\": {\"city\": \"NYC\", \"zip\": \"10001\"}}",
                null
        );

        AutoRegisterUtils.BuildResult result = AutoRegisterUtils.buildConsumerFromInteractions(
                Collections.singletonList(interaction),
                validConfig()
        );

        assertFalse(result.hasError());

        List<String> fields = result.getOptions().getUsedEndpoints().get(0).getUsedFields();
        assertTrue(fields.contains("id"));
        assertTrue(fields.contains("address"));
        assertTrue(fields.contains("address.city"));
        assertTrue(fields.contains("address.zip"));
    }
}
