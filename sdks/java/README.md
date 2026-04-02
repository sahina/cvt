# io.github.sahina:cvt-sdk

The Java SDK for **Contract Validator Toolkit (CVT)** — a consumer-driven contract validation platform for OpenAPI v2/v3 specifications.

CVT validates HTTP request/response interactions against registered OpenAPI schemas using a gRPC service. This SDK provides:

- Schema registration and validation against OpenAPI v2/v3 specs
- HTTP adapters (OkHttp) for automatic traffic validation
- Server-side middleware (Spring, Servlet) for producer validation
- Breaking change detection between schema versions
- Consumer registry and deployment safety checks (can-i-deploy)

For full documentation, visit the [CVT Documentation](https://sahina.github.io/cvt/).

## Installation

```xml
<dependency>
    <groupId>io.github.sahina</groupId>
    <artifactId>cvt-sdk</artifactId>
    <version>0.1.0</version> <!-- Replace with latest from Maven Central -->
</dependency>
```

Or with Gradle:

```gradle
dependencies {
    implementation 'io.github.sahina:cvt-sdk:0.1.0' // Replace with latest from Maven Central
}
```

## Usage

### 1. Initialize and Register Schema

You can register a schema from a local file or a URL.

```java
import io.github.sahina.sdk.ContractValidator;

ContractValidator validator = new ContractValidator("localhost:9550");

// Register from local file
validator.registerSchema("my-schema", "path/to/openapi.json");

// OR Register from URL
validator.registerSchema("petstore", "https://petstore.swagger.io/v2/swagger.json");
```

### 2. Validate Interactions

```java
import io.github.sahina.sdk.ValidationRequest;
import io.github.sahina.sdk.ValidationResponse;
import io.github.sahina.sdk.ValidationResult;

ValidationRequest request = ValidationRequest.builder()
    .method("POST")
    .path("/users")
    .body("{\"username\": \"alice\", \"email\": \"alice@example.com\"}")
    .build();

ValidationResponse response = ValidationResponse.builder()
    .statusCode(201)
    .build();

ValidationResult result = validator.validate(request, response);

if (result.isValid()) {
    System.out.println("Valid interaction");
} else {
    System.err.println("Validation errors: " + result.getErrors());
}
```

## HTTP Adapter (OkHttp)

The SDK includes an OkHttp adapter for automatic HTTP traffic validation:

```java
import io.github.sahina.sdk.adapters.OkHttpContractAdapter;
import io.github.sahina.sdk.adapters.AdapterConfig;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;

ContractValidator validator = new ContractValidator("localhost:9550");
validator.registerSchema("petstore", "./openapi.json");

OkHttpContractAdapter adapter = new OkHttpContractAdapter(validator)
    .withConfig(AdapterConfig.builder()
        .autoValidate(true)
        .build());

OkHttpClient client = new OkHttpClient.Builder()
    .addInterceptor(adapter)
    .build();

// All requests are now automatically validated
Request request = new Request.Builder()
    .url("http://petstore-service/pet/123")
    .build();
Response response = client.newCall(request).execute();
```

### Adapter Options

- `autoValidate`: Enable/disable automatic validation (default: true)
- `includePaths`: List of paths to include
- `excludePaths`: List of paths to exclude
- `onValidationFailure`: Custom error handler
- `getInteractions()`: Retrieve captured interactions
- `clearInteractions()`: Reset captured data

## Producer Validation (Server-Side Middleware)

Validate incoming requests and outgoing responses against your OpenAPI contract on the server side.

> **Full documentation:** See [Validation Modes](https://sahina.github.io/cvt/docs/guides/validation-modes) for detailed behavior, rollout strategy, and metrics information.

### Validation Modes

| Mode                    | Request Violation | Response Violation | Use Case               |
| ----------------------- | ----------------- | ------------------ | ---------------------- |
| `ValidationMode.STRICT` | Reject with 400   | Log error          | Production enforcement |
| `ValidationMode.WARN`   | Log, continue     | Log, continue      | Gradual rollout        |
| `ValidationMode.SHADOW` | Metrics only      | Metrics only       | Initial deployment     |

**Recommended rollout:** `SHADOW` -> `WARN` -> `STRICT`. See [Recommended Rollout Strategy](https://sahina.github.io/cvt/docs/guides/validation-modes#recommended-rollout-strategy).

### Spring Interceptor

```java
import io.github.sahina.sdk.producer.ProducerConfig;
import io.github.sahina.sdk.producer.ValidationMode;
import io.github.sahina.sdk.producer.adapters.SpringInterceptor;

ProducerConfig config = ProducerConfig.builder()
    .schemaId("my-api")
    .validator(validator)
    .mode(ValidationMode.STRICT)
    .excludePaths(List.of("/health", "/metrics"))
    .build();

@Configuration
public class WebConfig implements WebMvcConfigurer {
    @Override
    public void addInterceptors(InterceptorRegistry registry) {
        registry.addInterceptor(new SpringInterceptor(config))
            .addPathPatterns("/api/**");
    }
}
```

### Servlet Filter

```java
import io.github.sahina.sdk.producer.adapters.ServletFilter;

@Bean
public FilterRegistrationBean<ServletFilter> validationFilter() {
    FilterRegistrationBean<ServletFilter> registration = new FilterRegistrationBean<>();
    registration.setFilter(new ServletFilter(config));
    registration.addUrlPatterns("/api/*");
    return registration;
}
```

### Configuration Options

| Option              | Type             | Description                                        |
| ------------------- | ---------------- | -------------------------------------------------- |
| `schemaId`          | `String`         | Schema ID to validate against                      |
| `validator`         | `Validator`      | ContractValidator instance                         |
| `mode`              | `ValidationMode` | `STRICT`, `WARN`, or `SHADOW`                      |
| `validateRequest`   | `boolean`        | Enable request validation (default: true)          |
| `validateResponse`  | `boolean`        | Enable response validation (default: true)         |
| `excludePath`       | `String` (regex) | Paths to skip validation (builder adds patterns)   |
| `includePath`       | `String` (regex) | Only validate matching paths (builder adds patterns)|
| `onRequestFailure`  | `BiFunction`     | Called when request validation fails                |
| `onResponseFailure` | `Consumer`       | Called when response validation fails               |

## Breaking Change Detection

Detect breaking changes between OpenAPI schema versions before deployment:

```java
import io.github.sahina.sdk.ContractValidator;
import io.github.sahina.sdk.CompareResult;
import io.github.sahina.sdk.BreakingChange;

ContractValidator validator = new ContractValidator("localhost:9550");

// Register both schema versions
validator.registerSchemaWithVersion("my-api", "./openapi-v1.json", "1.0.0");
validator.registerSchemaWithVersion("my-api", "./openapi-v2.json", "2.0.0");

// Compare versions
CompareResult result = validator.compareSchemas("my-api", "1.0.0", "2.0.0");

if (!result.isCompatible()) {
    System.out.println("Breaking changes detected:");
    for (BreakingChange change : result.getBreakingChanges()) {
        System.out.printf("- [%s] %s%n", change.getType(), change.getDescription());
        if (change.getPath() != null) {
            System.out.printf("  Path: %s %s%n", change.getMethod(), change.getPath());
        }
    }
    System.exit(1); // Fail CI build
}
```

### Breaking Change Types

| Type                        | Description                                    |
| --------------------------- | ---------------------------------------------- |
| `ENDPOINT_REMOVED`          | An endpoint was removed                        |
| `REQUIRED_FIELD_ADDED`      | A required field was added to request          |
| `TYPE_CHANGED`              | A field's type was changed incompatibly        |
| `REQUIRED_PARAMETER_ADDED`  | A required query/path/header param was added   |
| `RESPONSE_SCHEMA_CHANGED`   | Response schema was changed incompatibly       |
| `ENUM_VALUE_REMOVED`        | An allowed enum value was removed              |

See [`examples/BreakingChanges.java`](https://github.com/sahina/cvt/tree/main/sdks/java/src/main/java/io/github/sahina/examples/BreakingChanges.java) for a complete example.

## Producer Testing

Test that your API handlers return responses matching your OpenAPI specification.

### ProducerTestKit

```java
import io.github.sahina.sdk.producer.ProducerTestKit;
import io.github.sahina.sdk.producer.TestResponseData;
import io.github.sahina.sdk.producer.TestValidationResult;

ProducerTestKit testKit = ProducerTestKit.builder()
    .schemaId("user-api")
    .serverAddress("localhost:9550")
    .build();

try {
    // Validate handler response
    TestValidationResult result = testKit.validateResponse(
        "GET",
        "/users/123",
        TestResponseData.builder()
            .statusCode(200)
            .body(Map.of("id", "123", "name", "Alice", "email", "alice@example.com"))
            .build()
    );

    assertTrue(result.isValid());
} finally {
    testKit.close();
}
```

### Consumer Registry

Track which services depend on your API:

```java
import io.github.sahina.sdk.RegisterConsumerOptions;
import io.github.sahina.sdk.EndpointUsage;
import io.github.sahina.sdk.ConsumerInfo;

// Register a consumer after successful contract tests
ConsumerInfo consumer = validator.registerConsumer(
    RegisterConsumerOptions.builder()
        .consumerId("order-service")
        .consumerVersion("2.1.0")
        .schemaId("user-api")
        .schemaVersion("1.0.0")
        .environment("prod")
        .usedEndpoints(List.of(
            new EndpointUsage("GET", "/users/{id}", List.of("id", "email"))))
        .build()
);

// List all consumers of a schema
List<ConsumerInfo> consumers = validator.listConsumers("user-api", "prod");

// Deregister a consumer
validator.deregisterConsumer("order-service", "user-api", "prod");
```

### Deployment Safety (can-i-deploy)

Check if a new schema version can be safely deployed:

```java
import io.github.sahina.sdk.CanIDeployResult;

CanIDeployResult result = validator.canIDeploy("user-api", "2.0.0", "prod");

if (!result.isSafeToDeploy()) {
    System.err.println("Cannot deploy: " + result.getSummary());
    for (var consumer : result.getAffectedConsumers()) {
        if (consumer.isWillBreak()) {
            System.err.println("- " + consumer.getConsumerId() + " will break");
        }
    }
    System.exit(1);
}
```

See [Producer Testing Guide](https://sahina.github.io/cvt/docs/guides/producer-testing) for complete documentation.

## Security Configuration

### TLS

```java
ContractValidator validator = ContractValidator.builder()
    .address("localhost:9550")
    .tlsEnabled(true)
    .rootCertPath("./certs/ca.crt")
    .build();
```

### API Key Authentication

```java
ContractValidator validator = ContractValidator.builder()
    .address("localhost:9550")
    .apiKey("your-api-key")
    .build();
```

## Prerequisites

Ensure the CVT gRPC server is running (default: `localhost:9550`).
