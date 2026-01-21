---
title: Java SDK
sidebar_label: Java
sidebar_position: 5
description: CVT SDK for Java
---

# Java SDK

The Java SDK provides contract validation for Java applications with Spring integration.

## Installation

### Gradle

```gradle
dependencies {
    implementation 'com.cvt:cvt-sdk:1.0.0'
}
```

### Maven

```xml
<dependency>
    <groupId>com.cvt</groupId>
    <artifactId>cvt-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

## Quick Start

```java
import com.cvt.sdk.ContractValidator;
import com.cvt.sdk.ValidationResult;
import com.cvt.sdk.Request;
import com.cvt.sdk.Response;

public class Example {
    public static void main(String[] args) throws Exception {
        ContractValidator validator = new ContractValidator("localhost:9550");

        // Register a schema
        String schema = Files.readString(Path.of("openapi.json"));
        validator.registerSchema("user-api", schema);

        // Validate an interaction
        ValidationResult result = validator.validate(
            new Request("GET", "/users/123"),
            new Response(200, "{\"id\": \"123\", \"name\": \"John\"}")
        );

        System.out.println("Valid: " + result.isValid());

        validator.close();
    }
}
```

## API Reference

### ContractValidator

#### Constructor

```java
ContractValidator(String address)
ContractValidator(String address, ValidatorOptions options)
```

#### Builder Pattern

```java
ContractValidator validator = ContractValidator.builder()
    .address("localhost:9550")
    .tlsRootCerts(rootCerts)
    .apiKey("your-api-key")
    .timeout(Duration.ofSeconds(30))
    .build();
```

### Methods

#### registerSchema

```java
RegisterSchemaResponse registerSchema(String schemaId, String content)
RegisterSchemaResponse registerSchema(String schemaId, String content, String version)
RegisterSchemaResponse registerSchema(RegisterSchemaRequest request)
```

#### validate

```java
ValidationResult validate(Request request, Response response)
ValidationResult validate(String schemaId, Request request, Response response)
```

#### registerConsumer

```java
RegisterConsumerResponse registerConsumer(RegisterConsumerRequest request)
```

#### listConsumers

```java
List<ConsumerInfo> listConsumers(String schemaId)
List<ConsumerInfo> listConsumers(String schemaId, String environment)
```

#### deregisterConsumer

```java
void deregisterConsumer(String consumerId, String schemaId, String environment)
```

#### compareSchemas

```java
CompareSchemasResponse compareSchemas(String schemaId, String oldVersion, String newVersion)
```

#### canIDeploy

```java
CanIDeployResponse canIDeploy(String schemaId, String newVersion, String environment)
```

#### generateFixture

```java
GeneratedFixture generateFixture(GenerateFixtureRequest request)
```

#### close

```java
void close()
```

## HTTP Adapters

### OkHttp Interceptor

```java
import com.cvt.sdk.adapters.OkHttpInterceptor;

ContractValidator validator = new ContractValidator("localhost:9550");
validator.registerSchema("user-api", schema);

OkHttpClient client = new OkHttpClient.Builder()
    .addInterceptor(new OkHttpInterceptor(validator, "user-api"))
    .build();

// All requests are now validated
Request request = new Request.Builder()
    .url("http://user-service/users/123")
    .build();
client.newCall(request).execute();
```

### Apache HttpClient

```java
import com.cvt.sdk.adapters.ApacheHttpClientInterceptor;

CloseableHttpClient client = HttpClients.custom()
    .addInterceptorFirst(new ApacheHttpClientInterceptor(validator, "user-api"))
    .build();
```

## Producer Middleware

### Spring Interceptor

```java
import com.cvt.sdk.producer.ProducerConfig;
import com.cvt.sdk.producer.ValidationMode;
import com.cvt.sdk.producer.adapters.SpringInterceptor;

@Configuration
public class WebConfig implements WebMvcConfigurer {

    @Autowired
    private ContractValidator validator;

    @Override
    public void addInterceptors(InterceptorRegistry registry) {
        ProducerConfig config = ProducerConfig.builder()
            .schemaId("my-api")
            .validator(validator)
            .mode(ValidationMode.STRICT)  // STRICT | WARN | SHADOW
            .build();

        registry.addInterceptor(new SpringInterceptor(config))
            .addPathPatterns("/api/**")
            .excludePathPatterns("/health", "/metrics");
    }
}
```

### Servlet Filter

```java
import com.cvt.sdk.producer.adapters.ServletFilter;

@WebFilter(urlPatterns = "/api/*")
public class ValidationFilter extends ServletFilter {
    public ValidationFilter() {
        super(ProducerConfig.builder()
            .schemaId("my-api")
            .validator(validator)
            .mode(ValidationMode.STRICT)
            .build());
    }
}
```

### Spring Boot Auto-Configuration

```java
@Configuration
@EnableCvtProducer
public class CvtConfig {

    @Bean
    public ProducerConfig producerConfig(ContractValidator validator) {
        return ProducerConfig.builder()
            .schemaId("my-api")
            .validator(validator)
            .mode(ValidationMode.STRICT)
            .build();
    }
}
```

## Producer Test Kit

```java
import com.cvt.sdk.producer.ProducerTestKit;
import org.junit.jupiter.api.*;

public class UserHandlerTest {
    private ProducerTestKit testKit;

    @BeforeEach
    void setup() {
        testKit = ProducerTestKit.builder()
            .schemaId("user-api")
            .serverAddress("localhost:9550")
            .build();
    }

    @AfterEach
    void teardown() {
        testKit.close();
    }

    @Test
    void getUserReturnsValidResponse() {
        // Call your handler
        var response = userHandler.getUser("123");

        // Validate
        var result = testKit.validateResponse(
            "GET",
            "/users/123",
            TestResponseData.builder()
                .statusCode(200)
                .body(response)
                .build()
        );

        assertTrue(result.isValid());
        assertTrue(result.getErrors().isEmpty());
    }
}
```

## TLS Configuration

```java
// TLS
ContractValidator validator = ContractValidator.builder()
    .address("localhost:9550")
    .tlsRootCerts(Files.readAllBytes(Path.of("certs/ca.crt")))
    .build();

// mTLS
ContractValidator validator = ContractValidator.builder()
    .address("localhost:9550")
    .tlsRootCerts(Files.readAllBytes(Path.of("certs/ca.crt")))
    .tlsCertChain(Files.readAllBytes(Path.of("certs/client.crt")))
    .tlsPrivateKey(Files.readAllBytes(Path.of("certs/client.key")))
    .build();
```

## API Key Authentication

```java
ContractValidator validator = ContractValidator.builder()
    .address("localhost:9550")
    .apiKey("your-api-key")
    .build();
```

## Types

```java
public class Request {
    private String method;
    private String path;
    private Map<String, String> headers;
    private String body;
}

public class Response {
    private int statusCode;
    private Map<String, String> headers;
    private String body;
}

public class ValidationResult {
    private boolean valid;
    private List<String> errors;
    private String validatedAgainstVersion;
    private String validatedAgainstHash;
}

public class BreakingChange {
    private BreakingChangeType type;
    private String path;
    private String method;
    private String description;
}

public class ConsumerInfo {
    private String consumerId;
    private String consumerVersion;
    private String schemaId;
    private String schemaVersion;
    private String environment;
    private List<EndpointUsage> usedEndpoints;
}
```

## Error Handling

```java
try {
    validator.registerSchema("my-api", schema);
} catch (InvalidSchemaException e) {
    System.err.println("Schema is not valid OpenAPI: " + e.getMessage());
} catch (CvtConnectionException e) {
    System.err.println("Cannot connect to CVT server: " + e.getMessage());
}

try {
    ValidationResult result = validator.validate(request, response);
} catch (SchemaNotFoundException e) {
    System.err.println("Schema not registered: " + e.getMessage());
}
```

## Async Support

```java
CompletableFuture<ValidationResult> future = validator.validateAsync(request, response);

future.thenAccept(result -> {
    if (result.isValid()) {
        System.out.println("Valid!");
    }
});
```

## Try-with-resources

```java
try (ContractValidator validator = new ContractValidator("localhost:9550")) {
    validator.registerSchema("my-api", schema);
    ValidationResult result = validator.validate(request, response);
}
// Connection automatically closed
```

## Related Documentation

- **[Consumer Testing Guide](../../guides/consumer-testing.mdx)** - Testing your API integrations
- **[Producer Testing Guide](../../guides/producer-testing.md)** - Validating your APIs
- **[API Reference](../api.md)** - Full gRPC API documentation
