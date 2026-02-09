# Contract Validator Toolkit (CVT) - Java SDK

The **CVT Java SDK** allows you to validate HTTP interactions (requests and responses) against OpenAPI schemas using the CVT gRPC service.

> **Status**: Fully Implemented

## Installation

**Note**: This package is currently for internal/development use.

To use locally, first install to your local Maven repository:

```bash
cd sdks/java
mvn install
```

Then add `mavenLocal()` to your repositories and the dependency:

**Gradle:**

```gradle
repositories {
    mavenLocal()
    mavenCentral()
}

dependencies {
    implementation 'com.cvt:cvt-sdk:0.1.0'
}
```

**Maven:**

```xml
<dependency>
    <groupId>com.cvt</groupId>
    <artifactId>cvt-sdk</artifactId>
    <version>0.1.0</version>
</dependency>
```

### Install from GitHub Packages

1. Create a GitHub [Personal Access Token](https://github.com/settings/tokens) with `read:packages` scope.

2. Add the repository and credentials to your `~/.m2/settings.xml`:

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

3. Add the GitHub Packages repository to your `pom.xml`:

```xml
<repositories>
    <repository>
        <id>github-cvt</id>
        <url>https://maven.pkg.github.com/sahina/cvt</url>
    </repository>
</repositories>

<dependencies>
    <dependency>
        <groupId>com.cvt</groupId>
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
            password = project.findProperty("gpr.key") ?: System.getenv("GITHUB_TOKEN")
        }
    }
}

dependencies {
    implementation 'com.cvt:cvt-sdk:0.1.0'
}
```

## Usage

### Initialize and Register Schema

```java
import com.cvt.ContractValidator;

ContractValidator validator = new ContractValidator("localhost:9550");

// Register from local file
validator.registerSchema("my-schema", "path/to/openapi.json");

// Register from URL
validator.registerSchema("petstore", "https://petstore.swagger.io/v2/swagger.json");
```

### Validate Interactions

```java
import com.cvt.ValidationRequest;
import com.cvt.ValidationResponse;
import com.cvt.ValidationResult;

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
    System.out.println("✅ Valid interaction");
} else {
    System.err.println("❌ Validation errors: " + result.getErrors());
}
```

## Producer Validation (Server-Side Middleware)

Validate incoming requests and outgoing responses against your OpenAPI contract on the server side.

> **Full documentation:** See [Validation Modes](../../docs/guides/validation-modes.mdx) for detailed behavior, rollout strategy, and metrics information.

### Validation Modes

| Mode                    | Request Violation | Response Violation | Use Case               |
| ----------------------- | ----------------- | ------------------ | ---------------------- |
| `ValidationMode.STRICT` | Reject with 400   | Log error          | Production enforcement |
| `ValidationMode.WARN`   | Log, continue     | Log, continue      | Gradual rollout        |
| `ValidationMode.SHADOW` | Metrics only      | Metrics only       | Initial deployment     |

**Recommended rollout:** `SHADOW` → `WARN` → `STRICT`. See [Recommended Rollout Strategy](../../docs/guides/validation-modes.mdx#recommended-rollout-strategy).

### Spring Interceptor

```java
import com.cvt.sdk.producer.ProducerConfig;
import com.cvt.sdk.producer.ValidationMode;
import com.cvt.sdk.producer.adapters.SpringInterceptor;

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
import com.cvt.sdk.producer.adapters.ServletFilter;

@Bean
public FilterRegistrationBean<ServletFilter> validationFilter() {
    FilterRegistrationBean<ServletFilter> registration = new FilterRegistrationBean<>();
    registration.setFilter(new ServletFilter(config));
    registration.addUrlPatterns("/api/*");
    return registration;
}
```

### Configuration Options

| Option              | Type             | Description                                |
| ------------------- | ---------------- | ------------------------------------------ |
| `schemaId`          | `String`         | Schema ID to validate against              |
| `validator`         | `Validator`      | ContractValidator instance                 |
| `mode`              | `ValidationMode` | `STRICT`, `WARN`, or `SHADOW`              |
| `excludePaths`      | `List<String>`   | Paths to skip validation (e.g., `/health`) |
| `includePaths`      | `List<String>`   | Only validate matching paths               |
| `validateResponse`  | `boolean`        | Enable response validation (default: true) |
| `onValidationError` | `Consumer`       | Custom error handler callback              |

## Breaking Change Detection

Detect breaking changes between OpenAPI schema versions before deployment:

```java
import com.cvt.sdk.ContractValidator;
import com.cvt.sdk.CompareResult;
import com.cvt.sdk.BreakingChange;

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

| Type                   | Description                           |
| ---------------------- | ------------------------------------- |
| `ENDPOINT_REMOVED`     | An endpoint was removed               |
| `REQUIRED_FIELD_ADDED` | A required field was added to request |
| `FIELD_TYPE_CHANGED`   | A field's type was changed            |
| `ENUM_VALUE_REMOVED`   | An allowed enum value was removed     |

See `examples/BreakingChanges.java` for a complete example.

## Producer Testing

Test that your API handlers return responses matching your OpenAPI specification.

### ProducerTestKit

```java
import com.cvt.sdk.producer.ProducerTestKit;
import com.cvt.sdk.producer.TestResponseData;
import com.cvt.sdk.producer.TestValidationResult;

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
import com.cvt.sdk.RegisterConsumerOptions;
import com.cvt.sdk.EndpointUsage;
import com.cvt.sdk.ConsumerInfo;

// Register a consumer after successful contract tests
ConsumerInfo consumer = validator.registerConsumer(
    RegisterConsumerOptions.builder()
        .consumerId("order-service")
        .consumerVersion("2.1.0")
        .schemaId("user-api")
        .schemaVersion("1.0.0")
        .environment("prod")
        .usedEndpoints(List.of(
            new EndpointUsage("GET", "/users/{id}", List.of("id", "email"))
        ))
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
import com.cvt.sdk.CanIDeployResult;

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

See [Producer Testing Guide](../../docs/guides/producer-testing.mdx) for complete documentation.

## Prerequisites

Ensure the CVT gRPC server is running (default: `localhost:9550`).

## Testing

The Java SDK includes tests covering:

- Client initialization and configuration
- Schema registration
- Validation requests and responses
- Error handling

### Running Tests

```bash
# Run all tests
mvn test

# Run tests with coverage
mvn test jacoco:report

# View coverage report
open target/site/jacoco/index.html

# Run specific test class
mvn test -Dtest=ContractValidatorTest
```

### Test Structure

```shell
src/test/java/com/cvt/
└── ContractValidatorTest.java  # Main SDK test suite
```

### Writing Tests

Example test using JUnit 5:

```java
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.AfterEach;
import static org.junit.jupiter.api.Assertions.*;

class ContractValidatorTest {
    private ContractValidator validator;

    @BeforeEach
    void setUp() {
        validator = new ContractValidator("localhost:9550");
    }

    @AfterEach
    void tearDown() {
        validator.close();
    }

    @Test
    void shouldValidateCorrectInteraction() {
        validator.registerSchema("test", "src/test/resources/openapi.json");

        ValidationResult result = validator.validate(
            ValidationRequest.builder()
                .method("GET")
                .path("/users")
                .build(),
            ValidationResponse.builder()
                .statusCode(200)
                .body("[]")
                .build()
        );

        assertTrue(result.isValid());
    }
}
```

### Coverage

The SDK targets 60%+ test coverage.

## Development

```bash
# Build the SDK
mvn package -DskipTests

# Run linter/verify
mvn verify -DskipTests

# Generate javadoc
mvn javadoc:javadoc

# Install to local Maven
mvn install
```

## Contributing

Contributions are welcome!

## License

MIT License
