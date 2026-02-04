# ContractValidator SDK Examples

Example code demonstrating the ContractValidator Java SDK.

## Quick Start

1. **Start the server** (from repository root):

   ```bash
   make up
   ```

2. **Run an example** (from `sdks/java` directory):

   ```bash
   mvn compile exec:java -Dexec.mainClass="com.cvt.examples.BasicUsage"
   # or
   mvn compile exec:java -Dexec.mainClass="com.cvt.examples.AdvancedUsage"
   ```

   Alternatively, compile and run directly:

   ```bash
   mvn package
   java -cp "target/cvt-sdk-*.jar:target/classes" com.cvt.examples.BasicUsage
   ```

## Examples

### BasicUsage.java

Introduction to the SDK covering:

- Client initialization and configuration
- Schema registration from local files
- Basic request/response validation
- Error handling

### AdvancedUsage.java

Advanced scenarios including:

- Nested object validation (Category, Tags)
- Batch validation (multiple requests)
- Path parameter validation
- Enum validation
- Detailed error messages for invalid requests

### BreakingChanges.java

Breaking change detection between schema versions:

- Register multiple schema versions
- Compare versions for breaking changes
- Detect removed endpoints, added required fields, type changes
- CI/CD integration patterns

```bash
mvn compile exec:java -Dexec.mainClass="com.cvt.examples.BreakingChanges"
```

## Troubleshooting

**Server connection errors?** Ensure the server is running: `make up` from repository root.

**Compilation errors?** Build the project first: `mvn package` from the `sdks/java` directory.

**Class not found?** Ensure you're using the correct classpath when running examples.
