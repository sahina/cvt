package io.github.sahina.sdk;

/**
 * Metadata describing a schema resolved via {@link ContractValidator#useSchema}.
 */
public class SchemaInfo {
    private final String schemaId;
    private final String schemaVersion;
    private final String schemaHash;
    private final long registeredAt;
    private final long updatedAt;
    private final String openapiVersion;
    private final int endpointCount;
    private final SchemaOwnership ownership;

    public SchemaInfo(
            String schemaId,
            String schemaVersion,
            String schemaHash,
            long registeredAt,
            long updatedAt,
            String openapiVersion,
            int endpointCount,
            SchemaOwnership ownership) {
        this.schemaId = schemaId;
        this.schemaVersion = schemaVersion;
        this.schemaHash = schemaHash != null ? schemaHash : "";
        this.registeredAt = registeredAt;
        this.updatedAt = updatedAt;
        this.openapiVersion = openapiVersion != null ? openapiVersion : "";
        this.endpointCount = endpointCount;
        this.ownership = ownership;
    }

    public String getSchemaId() {
        return schemaId;
    }

    public String getSchemaVersion() {
        return schemaVersion;
    }

    public String getSchemaHash() {
        return schemaHash;
    }

    public long getRegisteredAt() {
        return registeredAt;
    }

    public long getUpdatedAt() {
        return updatedAt;
    }

    public String getOpenapiVersion() {
        return openapiVersion;
    }

    public int getEndpointCount() {
        return endpointCount;
    }

    public SchemaOwnership getOwnership() {
        return ownership;
    }

    @Override
    public String toString() {
        return "SchemaInfo{" +
                "schemaId='" + schemaId + '\'' +
                ", schemaVersion='" + schemaVersion + '\'' +
                ", schemaHash='" + schemaHash + '\'' +
                ", registeredAt=" + registeredAt +
                ", updatedAt=" + updatedAt +
                ", openapiVersion='" + openapiVersion + '\'' +
                ", endpointCount=" + endpointCount +
                ", ownership=" + ownership +
                '}';
    }
}
