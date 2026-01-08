package com.cvt.sdk;

import com.cvt.sdk.adapters.CapturedInteraction;
import com.google.gson.Gson;
import com.google.gson.JsonArray;
import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonSyntaxException;

import java.net.URI;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Utility functions for auto-registration of consumers from captured interactions.
 */
public final class AutoRegisterUtils {
    private static final Gson GSON = new Gson();

    private AutoRegisterUtils() {
        // Utility class, prevent instantiation
    }

    /**
     * Extracts the schemaId from a mock URL.
     * For example: "http://mock.user-api/users/123" returns "user-api"
     *
     * @param urlStr The URL string to extract from
     * @return The schemaId, or null if extraction fails
     */
    public static String extractSchemaIdFromUrl(String urlStr) {
        if (urlStr == null || urlStr.isEmpty()) {
            return null;
        }

        try {
            URI uri = URI.create(urlStr);
            String host = uri.getHost();
            if (host == null) {
                return null;
            }

            // Strip port if present (URI.getHost() already excludes port)
            // Strip "mock." prefix if present
            if (host.startsWith("mock.")) {
                return host.substring(5);
            }
            return host;
        } catch (IllegalArgumentException e) {
            return null;
        }
    }

    /**
     * Result of extracting schemaId from interactions.
     */
    public static class SchemaIdResult {
        private final String schemaId;
        private final String error;

        public SchemaIdResult(String schemaId, String error) {
            this.schemaId = schemaId;
            this.error = error;
        }

        public String getSchemaId() {
            return schemaId;
        }

        public String getError() {
            return error;
        }

        public boolean hasError() {
            return error != null;
        }
    }

    /**
     * Extracts the schemaId from captured interactions.
     * Returns an error if multiple different schemas are detected or if no schema can be extracted.
     *
     * @param interactions The captured interactions
     * @return A SchemaIdResult with either the schemaId or an error
     */
    public static SchemaIdResult extractSchemaIdFromInteractions(List<CapturedInteraction> interactions) {
        Set<String> schemaIds = new HashSet<>();

        for (CapturedInteraction interaction : interactions) {
            String path = interaction.getPath();
            if (path != null && (path.startsWith("http://") || path.startsWith("https://"))) {
                String schemaId = extractSchemaIdFromUrl(path);
                if (schemaId != null) {
                    schemaIds.add(schemaId);
                }
            }
        }

        if (schemaIds.isEmpty()) {
            return new SchemaIdResult(null,
                    "could not extract schemaId from interactions; provide schemaId in config");
        }

        if (schemaIds.size() > 1) {
            List<String> sortedIds = new ArrayList<>(schemaIds);
            Collections.sort(sortedIds);
            return new SchemaIdResult(null,
                    "multiple schemas detected (" + String.join(", ", sortedIds) + "); provide explicit schemaId in config");
        }

        return new SchemaIdResult(schemaIds.iterator().next(), null);
    }

    /**
     * Normalizes a path by extracting just the path portion from a URL.
     *
     * @param pathOrUrl The path or full URL
     * @return The normalized path
     */
    public static String normalizePathForEndpoint(String pathOrUrl) {
        if (pathOrUrl == null) {
            return "";
        }

        String result = pathOrUrl;

        // If it's a full URL, parse it to extract just the path
        if (pathOrUrl.startsWith("http://") || pathOrUrl.startsWith("https://")) {
            try {
                URI uri = URI.create(pathOrUrl);
                result = uri.getPath();
                if (result == null) {
                    result = "/";
                }
            } catch (IllegalArgumentException e) {
                // Keep original if parse fails
            }
        }

        // Remove query string if present
        int queryIndex = result.indexOf('?');
        if (queryIndex != -1) {
            result = result.substring(0, queryIndex);
        }

        return result;
    }

    /**
     * Recursively extracts all field paths from a JSON body.
     * Uses dot notation for nested fields (e.g., "user.address.city").
     *
     * @param body   The JSON body (as parsed by Gson)
     * @param prefix The current prefix for nested fields
     * @return A list of field paths
     */
    public static List<String> extractFieldsFromBody(Object body, String prefix) {
        if (body == null) {
            return Collections.emptyList();
        }

        List<String> fields = new ArrayList<>();

        if (body instanceof JsonObject) {
            JsonObject obj = (JsonObject) body;
            for (Map.Entry<String, JsonElement> entry : obj.entrySet()) {
                String key = entry.getKey();
                String fieldPath = (prefix != null && !prefix.isEmpty()) ? prefix + "." + key : key;
                fields.add(fieldPath);
                // Recursively extract nested fields
                fields.addAll(extractFieldsFromBody(entry.getValue(), fieldPath));
            }
        } else if (body instanceof JsonArray) {
            JsonArray arr = (JsonArray) body;
            if (!arr.isEmpty()) {
                // For arrays, extract fields from the first element as representative
                fields.addAll(extractFieldsFromBody(arr.get(0), prefix));
            }
        } else if (body instanceof Map) {
            // Handle Map (from Gson parsing with Object.class)
            @SuppressWarnings("unchecked")
            Map<String, Object> map = (Map<String, Object>) body;
            for (Map.Entry<String, Object> entry : map.entrySet()) {
                String key = entry.getKey();
                String fieldPath = (prefix != null && !prefix.isEmpty()) ? prefix + "." + key : key;
                fields.add(fieldPath);
                // Recursively extract nested fields
                fields.addAll(extractFieldsFromBody(entry.getValue(), fieldPath));
            }
        } else if (body instanceof List) {
            // Handle List (from Gson parsing with Object.class)
            @SuppressWarnings("unchecked")
            List<Object> list = (List<Object>) body;
            if (!list.isEmpty()) {
                fields.addAll(extractFieldsFromBody(list.get(0), prefix));
            }
        }

        return fields;
    }

    /**
     * Extracts all field paths from a JSON body without a prefix.
     *
     * @param body The JSON body (as parsed by Gson)
     * @return A list of field paths
     */
    public static List<String> extractFieldsFromBody(Object body) {
        return extractFieldsFromBody(body, null);
    }

    /**
     * Parses a JSON string body into an object for field extraction.
     *
     * @param bodyJson The JSON string
     * @return The parsed object, or null if parsing fails
     */
    public static Object parseJsonBody(String bodyJson) {
        if (bodyJson == null || bodyJson.isEmpty()) {
            return null;
        }
        try {
            return GSON.fromJson(bodyJson, Object.class);
        } catch (JsonSyntaxException e) {
            return null;
        }
    }

    /**
     * Merges two lists, removing duplicates.
     *
     * @param a First list
     * @param b Second list
     * @return A new list containing unique elements from both lists
     */
    public static List<String> mergeArrays(List<String> a, List<String> b) {
        Set<String> merged = new HashSet<>();
        if (a != null) {
            merged.addAll(a);
        }
        if (b != null) {
            merged.addAll(b);
        }
        return new ArrayList<>(merged);
    }

    /**
     * Converts captured interactions to endpoint usage,
     * deduplicating by method+path and merging usedFields.
     *
     * @param interactions The captured interactions
     * @return A list of EndpointUsage objects
     */
    public static List<EndpointUsage> mergeInteractionsToEndpoints(List<CapturedInteraction> interactions) {
        Map<String, EndpointUsage> endpointMap = new HashMap<>();

        for (CapturedInteraction interaction : interactions) {
            String method = interaction.getMethod();
            String path = normalizePathForEndpoint(interaction.getPath());
            String key = method + ":" + path;

            // Parse response body and extract fields
            Object body = parseJsonBody(interaction.getResponseBody());
            List<String> fields = extractFieldsFromBody(body);

            EndpointUsage existing = endpointMap.get(key);
            if (existing != null) {
                // Merge fields (union)
                List<String> mergedFields = mergeArrays(
                        new ArrayList<>(existing.getUsedFields()),
                        fields
                );
                endpointMap.put(key, new EndpointUsage(method, path, mergedFields));
            } else {
                endpointMap.put(key, new EndpointUsage(method, path, fields));
            }
        }

        // Convert to sorted list for deterministic output
        List<String> sortedKeys = new ArrayList<>(endpointMap.keySet());
        Collections.sort(sortedKeys);

        List<EndpointUsage> result = new ArrayList<>();
        for (String key : sortedKeys) {
            EndpointUsage ep = endpointMap.get(key);
            // Sort usedFields for deterministic output
            List<String> sortedFields = new ArrayList<>(ep.getUsedFields());
            Collections.sort(sortedFields);
            result.add(new EndpointUsage(ep.getMethod(), ep.getPath(), sortedFields));
        }

        return result;
    }

    /**
     * Result of building consumer from interactions.
     */
    public static class BuildResult {
        private final RegisterConsumerOptions options;
        private final String error;

        public BuildResult(RegisterConsumerOptions options, String error) {
            this.options = options;
            this.error = error;
        }

        public RegisterConsumerOptions getOptions() {
            return options;
        }

        public String getError() {
            return error;
        }

        public boolean hasError() {
            return error != null;
        }
    }

    /**
     * Builds consumer registration options from captured interactions.
     *
     * @param interactions The captured interactions
     * @param config       The auto-registration configuration
     * @return A BuildResult with either the options or an error
     */
    public static BuildResult buildConsumerFromInteractions(
            List<CapturedInteraction> interactions,
            AutoRegisterConfig config) {
        // Validate required fields
        if (config.getConsumerId() == null || config.getConsumerId().isEmpty()) {
            return new BuildResult(null, "consumerId is required");
        }
        if (config.getConsumerVersion() == null || config.getConsumerVersion().isEmpty()) {
            return new BuildResult(null, "consumerVersion is required");
        }
        if (config.getEnvironment() == null || config.getEnvironment().isEmpty()) {
            return new BuildResult(null, "environment is required");
        }
        if (config.getSchemaVersion() == null || config.getSchemaVersion().isEmpty()) {
            return new BuildResult(null, "schemaVersion is required");
        }

        // Validate interactions
        if (interactions == null || interactions.isEmpty()) {
            return new BuildResult(null, "no interactions to register");
        }

        // Extract schemaId from interactions or use provided override
        String schemaId = config.getSchemaId();
        if (schemaId == null || schemaId.isEmpty()) {
            SchemaIdResult result = extractSchemaIdFromInteractions(interactions);
            if (result.hasError()) {
                return new BuildResult(null, result.getError());
            }
            schemaId = result.getSchemaId();
        }

        // Merge interactions into endpoint usage
        List<EndpointUsage> usedEndpoints = mergeInteractionsToEndpoints(interactions);

        RegisterConsumerOptions options = RegisterConsumerOptions.builder()
                .consumerId(config.getConsumerId())
                .consumerVersion(config.getConsumerVersion())
                .schemaId(schemaId)
                .schemaVersion(config.getSchemaVersion())
                .environment(config.getEnvironment())
                .usedEndpoints(usedEndpoints)
                .build();

        return new BuildResult(options, null);
    }
}
