package io.github.sahina.sdk;

/**
 * Ownership information for a registered schema.
 */
public class SchemaOwnership {
    private final String owner;
    private final String team;
    private final String contactEmail;
    private final boolean readOnly;

    public SchemaOwnership(String owner, String team, String contactEmail, boolean readOnly) {
        this.owner = owner != null ? owner : "";
        this.team = team != null ? team : "";
        this.contactEmail = contactEmail != null ? contactEmail : "";
        this.readOnly = readOnly;
    }

    public String getOwner() {
        return owner;
    }

    public String getTeam() {
        return team;
    }

    public String getContactEmail() {
        return contactEmail;
    }

    public boolean isReadOnly() {
        return readOnly;
    }

    @Override
    public String toString() {
        return "SchemaOwnership{" +
                "owner='" + owner + '\'' +
                ", team='" + team + '\'' +
                ", contactEmail='" + contactEmail + '\'' +
                ", readOnly=" + readOnly +
                '}';
    }
}
