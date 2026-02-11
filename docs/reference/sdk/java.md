---
title: Java SDK
sidebar_label: Java
sidebar_position: 5
description: CVT SDK for Java
---

# Java SDK

## What is the Java SDK?

The Java SDK provides contract validation for Java applications with Spring integration. It includes HTTP client adapters for automatic validation with OkHttp, producer middleware for Spring and Servlet applications, and a test kit for schema compliance testing.

:::tip SDK Architecture
For information about SDK design patterns, adapter architecture, and cross-language consistency, see [SDK Architecture](../architecture/sdk-architecture.md).
:::

## Installation

### Maven Central (Recommended)

No authentication required.

#### Maven

```xml
<dependency>
    <groupId>io.github.sahina</groupId>
    <artifactId>cvt-sdk</artifactId>
    <version>0.1.0</version>
</dependency>
```

#### Gradle

```gradle
dependencies {
    implementation("io.github.sahina:cvt-sdk:0.1.0")
}
```

### GitHub Packages

Alternatively, install from GitHub Packages (requires a GitHub PAT with `read:packages` scope):

1. Add credentials to `~/.m2/settings.xml`:

    ```xml
    <settings>
      <servers>
        <server>
          <id>github-cvt</id>
          <username>YOUR_GITHUB_USERNAME</username>
          <password>YOUR_GITHUB_PAT</password>
        </server>
      </servers>
    </settings>
    ```

2. Add the repository and dependency to your `pom.xml`:

```xml
<repositories>
    <repository>
        <id>github-cvt</id>
        <url>https://maven.pkg.github.com/sahina/cvt</url>
    </repository>
</repositories>

<dependencies>
    <dependency>
        <groupId>io.github.sahina</groupId>
        <artifactId>cvt-sdk</artifactId>
        <version>0.1.0</version>
    </dependency>
</dependencies>
```

For Gradle, add to `build.gradle`:

```gradle
repositories {
    maven {
        url = uri("https://maven.pkg.github.com/sahina/cvt")
        credentials {
            username = project.findProperty("gpr.user") ?: System.getenv("GITHUB_USERNAME")
            password = project.findProperty("gpr.key") ?: System.getenv("GITHUB_PAT")
        }
    }
}

dependencies {
    implementation("io.github.sahina:cvt-sdk:LATEST")
}
```

Replace `LATEST` with the desired version. Available versions can be found on the project's [GitHub Packages](https://github.com/sahina/cvt/packages) page.

## Quick Start

```java
import io.github.sahina.sdk.ContractValidator;
import io.github.sahina.sdk.ValidationResult;
import io.github.sahina.sdk.ValidationRequest;
import io.github.sahina.sdk.ValidationResponse;

public class Example {
    public static void main(String[] args) throws Exception {
        ContractValidator validator = new ContractValidator("localhost:9550");

        // Register a schema from file
        validator.registerSchema("petstore", "./openapi.json");
        // Or register from URL:
        // validator.registerSchema("petstore", "https://petstore3.swagger.io/api/v3/openapi.json");

        // Validate an interaction
        ValidationResult result = validator.validate(
            new ValidationRequest("GET", "/pet/123", null, null),
            new ValidationResponse(200, null, "{\"id\": 123, \"name\": \"doggie\", \"status\": \"available\"}")
        );

        System.out.println("Valid: " + result.isValid());
        if (!result.isValid()) {
            System.out.println("Errors: " + result.getErrors());
        }

        validator.close();
    }
}
```

## API Reference

### ContractValidator

#### Constructor

```java
// Simple usage (insecure connection)
ContractValidator()  // Uses localhost:9550
ContractValidator(String address)

// With builder pattern (TLS and API key)
ContractValidator.builder()
    .address("localhost:9550")
    .tlsEnabled(true)
    .rootCertPath("./certs/ca.crt")
    .apiKey("your-api-key")
    .build()
```

**Simple usage:**

```java
ContractValidator validator = new ContractValidator("localhost:9550");
```

**With builder:**

```java
ContractValidator validator = ContractValidator.builder()
    .address("localhost:9550")
    .tlsEnabled(true)
    .rootCertPath("./certs/ca.crt")
    .apiKey("your-api-key")
    .build();
```

| Option         | Type      | Description                                |
| -------------- | --------- | ------------------------------------------ |
| `address`      | `String`  | Server address (default: `localhost:9550`) |
| `tlsEnabled`   | `boolean` | Enable TLS                                 |
| `rootCertPath` | `String`  | Path to CA certificate                     |
| `apiKey`       | `String`  | API key for authentication                 |

### Methods

#### registerSchema

Registers an OpenAPI schema from a file path or URL.

```java
void registerSchema(String schemaId, String schemaPath) throws IOException
```

```java
// From local file
validator.registerSchema("petstore", "./openapi.json");

// From URL
validator.registerSchema("petstore", "https://petstore3.swagger.io/api/v3/openapi.json");
```

#### registerSchemaWithVersion

Registers a schema with version information for comparison.

```java
void registerSchemaWithVersion(String schemaId, String schemaPath, String version) throws IOException
```

```java
validator.registerSchemaWithVersion("petstore", "./openapi.json", "1.0.0");
```

#### validate

Validates an HTTP request/response pair against the registered schema.

```java
ValidationResult validate(ValidationRequest request, ValidationResponse response)
```

```java
ValidationResult result = validator.validate(
    new ValidationRequest("GET", "/pet/123", null, null),
    new ValidationResponse(200, null, "{\"id\": 123, \"name\": \"doggie\"}")
);

if (!result.isValid()) {
    System.err.println("Errors: " + result.getErrors());
}
```

#### compareSchemas

Compares two schema versions for breaking changes.

```java
CompareResult compareSchemas(String schemaId, String oldVersion, String newVersion)
```

```java
CompareResult result = validator.compareSchemas("petstore", "1.0.0", "2.0.0");
if (!result.isCompatible()) {
    for (BreakingChange change : result.getBreakingChanges()) {
        System.out.println(change.getType() + ": " + change.getDescription());
    }
}
```

#### generateFixture

Generates test fixtures from the schema.

```java
GeneratedFixture generateFixture(String method, String path, GenerateOptions options)
```

```java
GeneratedFixture fixture = validator.generateFixture("GET", "/pet/{petId}", null);
System.out.println("Request: " + fixture.getRequest());
System.out.println("Response: " + fixture.getResponse());
```

#### generateResponse

Generates a response fixture only.

```java
GeneratedResponse generateResponse(String method, String path, GenerateOptions options)
```

#### generateRequestBody

Generates a request body fixture for an endpoint.

```java
Object generateRequestBody(String method, String path, GenerateOptions options)
```

```java
Object body = validator.generateRequestBody("POST", "/pet", null);
System.out.println("Request body: " + body);
```

#### listEndpoints

Lists all endpoints in the registered schema.

```java
List<EndpointInfo> listEndpoints()
```

```java
List<EndpointInfo> endpoints = validator.listEndpoints();
for (EndpointInfo ep : endpoints) {
    System.out.println(ep.getMethod() + " " + ep.getPath() + " - " + ep.getSummary());
}
```

#### registerConsumer

Registers a consumer with expected interactions.

```java
ConsumerInfo registerConsumer(RegisterConsumerOptions options)
```

```java
ConsumerInfo consumer = validator.registerConsumer(
    new RegisterConsumerOptions(
        "order-service",
        "2.1.0",
        "petstore",
        "1.0.0",
        "prod",
        List.of(new EndpointUsage("GET", "/pet/{petId}", List.of("id", "name", "status")))
    )
);
```

#### listConsumers

Lists all consumers for a schema.

```java
List<ConsumerInfo> listConsumers(String schemaId, String environment)
```

```java
List<ConsumerInfo> consumers = validator.listConsumers("petstore", "prod");
for (ConsumerInfo c : consumers) {
    System.out.println(c.getConsumerId() + " v" + c.getConsumerVersion());
}
```

#### deregisterConsumer

Removes a consumer registration.

```java
void deregisterConsumer(String consumerId, String schemaId, String environment)
```

#### canIDeploy

Checks if a schema version can be safely deployed.

```java
CanIDeployResult canIDeploy(String schemaId, String newVersion, String environment)
```

```java
CanIDeployResult result = validator.canIDeploy("petstore", "2.0.0", "prod");
if (!result.isSafeToDeploy()) {
    System.out.println("Unsafe: " + result.getSummary());
    for (ConsumerImpact c : result.getAffectedConsumers()) {
        System.out.println("- " + c.getConsumerId() + " will break");
    }
}
```

#### close

Closes the gRPC connection. Implements `AutoCloseable` for try-with-resources.

```java
void close()
```

## HTTP Adapters

### OkHttp Adapter

Automatically validate all OkHttp requests:

```java
import io.github.sahina.sdk.adapters.OkHttpContractAdapter;
import io.github.sahina.sdk.adapters.AdapterConfig;

ContractValidator validator = new ContractValidator("localhost:9550");
validator.registerSchema("petstore", "./openapi.json");

OkHttpContractAdapter adapter = new OkHttpContractAdapter(AdapterConfig.builder()
    .validator(validator)
    .autoValidate(true)
    .onValidationFailure(result -> {
        throw new RuntimeException("Contract violation: " + result.getErrors());
    })
    .build());

OkHttpClient client = new OkHttpClient.Builder()
    .addInterceptor(adapter)
    .build();

// All requests are now validated
Request request = new Request.Builder()
    .url("http://petstore-service/pet/123")
    .build();
Response response = client.newCall(request).execute();

// Check captured interactions
List<CapturedInteraction> interactions = adapter.getInteractions();
```

## Producer Middleware

### Spring Interceptor

```java
import io.github.sahina.sdk.producer.ProducerConfig;
import io.github.sahina.sdk.producer.ValidationMode;
import io.github.sahina.sdk.producer.adapters.SpringInterceptor;

@Configuration
public class WebConfig implements WebMvcConfigurer {

    @Autowired
    private ContractValidator validator;

    @Override
    public void addInterceptors(InterceptorRegistry registry) {
        ProducerConfig config = ProducerConfig.builder()
            .schemaId("petstore")
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
import io.github.sahina.sdk.producer.adapters.ServletFilter;

@WebFilter(urlPatterns = "/api/*")
public class ValidationFilter extends ServletFilter {
    public ValidationFilter(ContractValidator validator) {
        super(ProducerConfig.builder()
            .schemaId("petstore")
            .validator(validator)
            .mode(ValidationMode.STRICT)
            .build());
    }
}
```

## Producer Test Kit

Test your API responses against your schema without real consumers:

```java
import io.github.sahina.sdk.producer.ProducerTestKit;
import io.github.sahina.sdk.producer.TestResponseData;
import io.github.sahina.sdk.producer.TestValidationResult;
import org.junit.jupiter.api.*;

public class PetHandlerTest {
    private ProducerTestKit testKit;

    @BeforeEach
    void setup() {
        testKit = ProducerTestKit.builder()
            .schemaId("petstore")
            .serverAddress("localhost:9550")
            .build();
    }

    @AfterEach
    void teardown() {
        testKit.close();
    }

    @Test
    void getPetReturnsValidResponse() {
        // Call your handler
        Pet pet = petHandler.getPet(123);

        // Validate
        TestValidationResult result = testKit.validateResponse(
            "GET",
            "/pet/123",
            TestResponseData.builder()
                .statusCode(200)
                .body(pet)
                .build()
        );

        assertTrue(result.isValid());
        assertTrue(result.getErrors().isEmpty());
    }

    @Test
    void detectsInvalidResponse() {
        TestValidationResult result = testKit.validateResponse(
            "GET",
            "/pet/123",
            TestResponseData.builder()
                .statusCode(200)
                .body(Map.of("id", "not-a-number"))  // Invalid type
                .build()
        );

        assertFalse(result.isValid());
        assertFalse(result.getErrors().isEmpty());
    }
}
```

### ForEndpoint Helper

Test multiple scenarios for the same endpoint:

```java
ProducerTestKit.EndpointTester getPetEndpoint = testKit.forEndpoint("GET", "/pet/{petId}");

// Test valid response
TestValidationResult result = getPetEndpoint.validateResponse(
    TestResponseData.builder()
        .statusCode(200)
        .body(Map.of("id", 123, "name", "doggie", "status", "available"))
        .build(),
    Map.of("petId", "123")
);

// Test not found
result = getPetEndpoint.validateResponse(
    TestResponseData.builder()
        .statusCode(404)
        .body(Map.of("message", "Pet not found"))
        .build(),
    Map.of("petId", "999")
);
```

## Auto-Registration

Build consumer info from captured interactions:

```java
import io.github.sahina.sdk.AutoRegisterConfig;
import io.github.sahina.sdk.adapters.CapturedInteraction;

// From OkHttp adapter
OkHttpContractAdapter adapter = ...;
List<CapturedInteraction> interactions = adapter.getInteractions();

// Auto-register consumer from captured interactions
ConsumerInfo info = validator.registerConsumerFromInteractions(
    interactions,
    AutoRegisterConfig.builder()
        .consumerId("order-service")
        .consumerVersion("2.1.0")
        .environment("dev")
        .schemaVersion("1.0.0")
        .build()
);

System.out.println("Registered: " + info.getConsumerId());
```

Or preview before registering:

```java
AutoRegisterUtils.BuildResult result = validator.buildConsumerFromInteractions(
    interactions,
    AutoRegisterConfig.builder()
        .consumerId("order-service")
        .consumerVersion("2.1.0")
        .environment("dev")
        .schemaVersion("1.0.0")
        .build()
);

if (result.hasError()) {
    System.err.println("Error: " + result.getError());
} else {
    System.out.println("Would register: " + result.getOptions().getUsedEndpoints().size() + " endpoints");
}
```

## TLS Configuration

```java
ContractValidator validator = ContractValidator.builder()
    .address("localhost:9550")
    .tlsEnabled(true)
    .rootCertPath("./certs/ca.crt")
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
public class ValidationRequest {
    private String method;
    private String path;
    private Map<String, String> headers;
    private String body;
}

public class ValidationResponse {
    private int statusCode;
    private Map<String, String> headers;
    private String body;
}

public class ValidationResult {
    private boolean valid;
    private List<String> errors;
}

public class BreakingChange {
    private String type;
    private String path;
    private String method;
    private String description;
    private String oldValue;
    private String newValue;
}

public class CompareResult {
    private boolean compatible;
    private List<BreakingChange> breakingChanges;
}

public class ConsumerInfo {
    private String consumerId;
    private String consumerVersion;
    private String schemaId;
    private String schemaVersion;
    private String environment;
    private long registeredAt;
    private long lastValidatedAt;
    private List<EndpointUsage> usedEndpoints;
}

public class CanIDeployResult {
    private boolean safeToDeploy;
    private String summary;
    private List<BreakingChange> breakingChanges;
    private List<ConsumerImpact> affectedConsumers;
}
```

## Error Handling

```java
try {
    validator.registerSchema("petstore", "./openapi.json");
} catch (IOException e) {
    System.err.println("Failed to read or register schema: " + e.getMessage());
} catch (IllegalArgumentException e) {
    System.err.println("Schema registration failed: " + e.getMessage());
}

// Validation errors are returned in the result, not thrown
ValidationResult result = validator.validate(request, response);
if (!result.isValid()) {
    System.err.println("Validation errors: " + result.getErrors());
}
```

## Try-with-resources

The ContractValidator implements `AutoCloseable`:

```java
try (ContractValidator validator = new ContractValidator("localhost:9550")) {
    validator.registerSchema("petstore", "./openapi.json");
    ValidationResult result = validator.validate(request, response);
}
// Connection automatically closed
```

## Related Documentation

- **[Consumer Testing Guide](../../guides/consumer-testing.mdx)** - Testing your API integrations
- **[Producer Testing Guide](../../guides/producer-testing.mdx)** - Validating your APIs
- **[API Reference](../api.mdx)** - Full gRPC API documentation
