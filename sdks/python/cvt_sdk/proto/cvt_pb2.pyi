from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class BreakingChangeType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    BREAKING_CHANGE_UNSPECIFIED: _ClassVar[BreakingChangeType]
    ENDPOINT_REMOVED: _ClassVar[BreakingChangeType]
    REQUIRED_FIELD_ADDED: _ClassVar[BreakingChangeType]
    TYPE_CHANGED: _ClassVar[BreakingChangeType]
    REQUIRED_PARAMETER_ADDED: _ClassVar[BreakingChangeType]
    RESPONSE_SCHEMA_CHANGED: _ClassVar[BreakingChangeType]
    ENUM_VALUE_REMOVED: _ClassVar[BreakingChangeType]

class OutputType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    OUTPUT_FIXTURE: _ClassVar[OutputType]
    OUTPUT_REQUEST: _ClassVar[OutputType]
    OUTPUT_RESPONSE: _ClassVar[OutputType]
BREAKING_CHANGE_UNSPECIFIED: BreakingChangeType
ENDPOINT_REMOVED: BreakingChangeType
REQUIRED_FIELD_ADDED: BreakingChangeType
TYPE_CHANGED: BreakingChangeType
REQUIRED_PARAMETER_ADDED: BreakingChangeType
RESPONSE_SCHEMA_CHANGED: BreakingChangeType
ENUM_VALUE_REMOVED: BreakingChangeType
OUTPUT_FIXTURE: OutputType
OUTPUT_REQUEST: OutputType
OUTPUT_RESPONSE: OutputType

class SchemaOwnership(_message.Message):
    __slots__ = ("owner", "team", "contact_email", "read_only")
    OWNER_FIELD_NUMBER: _ClassVar[int]
    TEAM_FIELD_NUMBER: _ClassVar[int]
    CONTACT_EMAIL_FIELD_NUMBER: _ClassVar[int]
    READ_ONLY_FIELD_NUMBER: _ClassVar[int]
    owner: str
    team: str
    contact_email: str
    read_only: bool
    def __init__(self, owner: _Optional[str] = ..., team: _Optional[str] = ..., contact_email: _Optional[str] = ..., read_only: bool = ...) -> None: ...

class SchemaMetadata(_message.Message):
    __slots__ = ("schema_id", "schema_version", "schema_hash", "registered_at", "updated_at", "ownership", "openapi_version", "endpoint_count")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_HASH_FIELD_NUMBER: _ClassVar[int]
    REGISTERED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    OWNERSHIP_FIELD_NUMBER: _ClassVar[int]
    OPENAPI_VERSION_FIELD_NUMBER: _ClassVar[int]
    ENDPOINT_COUNT_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    schema_version: str
    schema_hash: str
    registered_at: int
    updated_at: int
    ownership: SchemaOwnership
    openapi_version: str
    endpoint_count: int
    def __init__(self, schema_id: _Optional[str] = ..., schema_version: _Optional[str] = ..., schema_hash: _Optional[str] = ..., registered_at: _Optional[int] = ..., updated_at: _Optional[int] = ..., ownership: _Optional[_Union[SchemaOwnership, _Mapping]] = ..., openapi_version: _Optional[str] = ..., endpoint_count: _Optional[int] = ...) -> None: ...

class RegisterSchemaRequest(_message.Message):
    __slots__ = ("schema_id", "schema_content", "schema_version", "ownership", "check_compatibility")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_CONTENT_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    OWNERSHIP_FIELD_NUMBER: _ClassVar[int]
    CHECK_COMPATIBILITY_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    schema_content: str
    schema_version: str
    ownership: SchemaOwnership
    check_compatibility: bool
    def __init__(self, schema_id: _Optional[str] = ..., schema_content: _Optional[str] = ..., schema_version: _Optional[str] = ..., ownership: _Optional[_Union[SchemaOwnership, _Mapping]] = ..., check_compatibility: bool = ...) -> None: ...

class RegisterSchemaResponse(_message.Message):
    __slots__ = ("success", "message", "metadata", "breaking_changes")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    BREAKING_CHANGES_FIELD_NUMBER: _ClassVar[int]
    success: bool
    message: str
    metadata: SchemaMetadata
    breaking_changes: _containers.RepeatedCompositeFieldContainer[BreakingChange]
    def __init__(self, success: bool = ..., message: _Optional[str] = ..., metadata: _Optional[_Union[SchemaMetadata, _Mapping]] = ..., breaking_changes: _Optional[_Iterable[_Union[BreakingChange, _Mapping]]] = ...) -> None: ...

class InteractionRequest(_message.Message):
    __slots__ = ("schema_id", "request", "response", "schema_version")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    RESPONSE_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    request: RequestData
    response: ResponseData
    schema_version: str
    def __init__(self, schema_id: _Optional[str] = ..., request: _Optional[_Union[RequestData, _Mapping]] = ..., response: _Optional[_Union[ResponseData, _Mapping]] = ..., schema_version: _Optional[str] = ...) -> None: ...

class RequestData(_message.Message):
    __slots__ = ("method", "path", "headers", "body")
    class HeadersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    METHOD_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    method: str
    path: str
    headers: _containers.ScalarMap[str, str]
    body: str
    def __init__(self, method: _Optional[str] = ..., path: _Optional[str] = ..., headers: _Optional[_Mapping[str, str]] = ..., body: _Optional[str] = ...) -> None: ...

class ResponseData(_message.Message):
    __slots__ = ("status_code", "headers", "body")
    class HeadersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    STATUS_CODE_FIELD_NUMBER: _ClassVar[int]
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    status_code: int
    headers: _containers.ScalarMap[str, str]
    body: str
    def __init__(self, status_code: _Optional[int] = ..., headers: _Optional[_Mapping[str, str]] = ..., body: _Optional[str] = ...) -> None: ...

class ValidationResult(_message.Message):
    __slots__ = ("valid", "errors", "validated_against_version", "validated_against_hash")
    VALID_FIELD_NUMBER: _ClassVar[int]
    ERRORS_FIELD_NUMBER: _ClassVar[int]
    VALIDATED_AGAINST_VERSION_FIELD_NUMBER: _ClassVar[int]
    VALIDATED_AGAINST_HASH_FIELD_NUMBER: _ClassVar[int]
    valid: bool
    errors: _containers.RepeatedScalarFieldContainer[str]
    validated_against_version: str
    validated_against_hash: str
    def __init__(self, valid: bool = ..., errors: _Optional[_Iterable[str]] = ..., validated_against_version: _Optional[str] = ..., validated_against_hash: _Optional[str] = ...) -> None: ...

class BreakingChange(_message.Message):
    __slots__ = ("type", "path", "method", "description", "old_value", "new_value")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    OLD_VALUE_FIELD_NUMBER: _ClassVar[int]
    NEW_VALUE_FIELD_NUMBER: _ClassVar[int]
    type: BreakingChangeType
    path: str
    method: str
    description: str
    old_value: str
    new_value: str
    def __init__(self, type: _Optional[_Union[BreakingChangeType, str]] = ..., path: _Optional[str] = ..., method: _Optional[str] = ..., description: _Optional[str] = ..., old_value: _Optional[str] = ..., new_value: _Optional[str] = ...) -> None: ...

class GetSchemaRequest(_message.Message):
    __slots__ = ("schema_id", "schema_version")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    schema_version: str
    def __init__(self, schema_id: _Optional[str] = ..., schema_version: _Optional[str] = ...) -> None: ...

class GetSchemaResponse(_message.Message):
    __slots__ = ("found", "metadata", "schema_content")
    FOUND_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_CONTENT_FIELD_NUMBER: _ClassVar[int]
    found: bool
    metadata: SchemaMetadata
    schema_content: str
    def __init__(self, found: bool = ..., metadata: _Optional[_Union[SchemaMetadata, _Mapping]] = ..., schema_content: _Optional[str] = ...) -> None: ...

class ListSchemasRequest(_message.Message):
    __slots__ = ("page_size", "page_token", "owner", "team")
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    TEAM_FIELD_NUMBER: _ClassVar[int]
    page_size: int
    page_token: str
    owner: str
    team: str
    def __init__(self, page_size: _Optional[int] = ..., page_token: _Optional[str] = ..., owner: _Optional[str] = ..., team: _Optional[str] = ...) -> None: ...

class ListSchemasResponse(_message.Message):
    __slots__ = ("schemas", "next_page_token", "total_count")
    SCHEMAS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    TOTAL_COUNT_FIELD_NUMBER: _ClassVar[int]
    schemas: _containers.RepeatedCompositeFieldContainer[SchemaMetadata]
    next_page_token: str
    total_count: int
    def __init__(self, schemas: _Optional[_Iterable[_Union[SchemaMetadata, _Mapping]]] = ..., next_page_token: _Optional[str] = ..., total_count: _Optional[int] = ...) -> None: ...

class CompareSchemasRequest(_message.Message):
    __slots__ = ("schema_id", "old_version", "new_version")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    OLD_VERSION_FIELD_NUMBER: _ClassVar[int]
    NEW_VERSION_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    old_version: str
    new_version: str
    def __init__(self, schema_id: _Optional[str] = ..., old_version: _Optional[str] = ..., new_version: _Optional[str] = ...) -> None: ...

class CompareSchemasResponse(_message.Message):
    __slots__ = ("compatible", "breaking_changes", "old_schema", "new_schema")
    COMPATIBLE_FIELD_NUMBER: _ClassVar[int]
    BREAKING_CHANGES_FIELD_NUMBER: _ClassVar[int]
    OLD_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    NEW_SCHEMA_FIELD_NUMBER: _ClassVar[int]
    compatible: bool
    breaking_changes: _containers.RepeatedCompositeFieldContainer[BreakingChange]
    old_schema: SchemaMetadata
    new_schema: SchemaMetadata
    def __init__(self, compatible: bool = ..., breaking_changes: _Optional[_Iterable[_Union[BreakingChange, _Mapping]]] = ..., old_schema: _Optional[_Union[SchemaMetadata, _Mapping]] = ..., new_schema: _Optional[_Union[SchemaMetadata, _Mapping]] = ...) -> None: ...

class GenerateFixtureRequest(_message.Message):
    __slots__ = ("schema_id", "method", "path", "status_code", "use_examples", "content_type", "output_type")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    STATUS_CODE_FIELD_NUMBER: _ClassVar[int]
    USE_EXAMPLES_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_TYPE_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    method: str
    path: str
    status_code: int
    use_examples: bool
    content_type: str
    output_type: OutputType
    def __init__(self, schema_id: _Optional[str] = ..., method: _Optional[str] = ..., path: _Optional[str] = ..., status_code: _Optional[int] = ..., use_examples: bool = ..., content_type: _Optional[str] = ..., output_type: _Optional[_Union[OutputType, str]] = ...) -> None: ...

class GenerateFixtureResponse(_message.Message):
    __slots__ = ("success", "message", "fixture", "request_body", "response")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    FIXTURE_FIELD_NUMBER: _ClassVar[int]
    REQUEST_BODY_FIELD_NUMBER: _ClassVar[int]
    RESPONSE_FIELD_NUMBER: _ClassVar[int]
    success: bool
    message: str
    fixture: GeneratedFixture
    request_body: str
    response: GeneratedResponse
    def __init__(self, success: bool = ..., message: _Optional[str] = ..., fixture: _Optional[_Union[GeneratedFixture, _Mapping]] = ..., request_body: _Optional[str] = ..., response: _Optional[_Union[GeneratedResponse, _Mapping]] = ...) -> None: ...

class GeneratedFixture(_message.Message):
    __slots__ = ("request", "response")
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    RESPONSE_FIELD_NUMBER: _ClassVar[int]
    request: GeneratedRequest
    response: GeneratedResponse
    def __init__(self, request: _Optional[_Union[GeneratedRequest, _Mapping]] = ..., response: _Optional[_Union[GeneratedResponse, _Mapping]] = ...) -> None: ...

class GeneratedRequest(_message.Message):
    __slots__ = ("method", "path", "headers", "body")
    class HeadersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    METHOD_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    method: str
    path: str
    headers: _containers.ScalarMap[str, str]
    body: str
    def __init__(self, method: _Optional[str] = ..., path: _Optional[str] = ..., headers: _Optional[_Mapping[str, str]] = ..., body: _Optional[str] = ...) -> None: ...

class GeneratedResponse(_message.Message):
    __slots__ = ("status_code", "headers", "body")
    class HeadersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    STATUS_CODE_FIELD_NUMBER: _ClassVar[int]
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    BODY_FIELD_NUMBER: _ClassVar[int]
    status_code: int
    headers: _containers.ScalarMap[str, str]
    body: str
    def __init__(self, status_code: _Optional[int] = ..., headers: _Optional[_Mapping[str, str]] = ..., body: _Optional[str] = ...) -> None: ...

class ListEndpointsRequest(_message.Message):
    __slots__ = ("schema_id",)
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    def __init__(self, schema_id: _Optional[str] = ...) -> None: ...

class ListEndpointsResponse(_message.Message):
    __slots__ = ("endpoints",)
    ENDPOINTS_FIELD_NUMBER: _ClassVar[int]
    endpoints: _containers.RepeatedCompositeFieldContainer[EndpointInfo]
    def __init__(self, endpoints: _Optional[_Iterable[_Union[EndpointInfo, _Mapping]]] = ...) -> None: ...

class EndpointInfo(_message.Message):
    __slots__ = ("method", "path", "operation_id", "summary")
    METHOD_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    OPERATION_ID_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    method: str
    path: str
    operation_id: str
    summary: str
    def __init__(self, method: _Optional[str] = ..., path: _Optional[str] = ..., operation_id: _Optional[str] = ..., summary: _Optional[str] = ...) -> None: ...

class ValidateProducerRequest(_message.Message):
    __slots__ = ("schema_id", "schema_version", "method", "path", "response", "request")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    METHOD_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    RESPONSE_FIELD_NUMBER: _ClassVar[int]
    REQUEST_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    schema_version: str
    method: str
    path: str
    response: ResponseData
    request: RequestData
    def __init__(self, schema_id: _Optional[str] = ..., schema_version: _Optional[str] = ..., method: _Optional[str] = ..., path: _Optional[str] = ..., response: _Optional[_Union[ResponseData, _Mapping]] = ..., request: _Optional[_Union[RequestData, _Mapping]] = ...) -> None: ...

class ConsumerInfo(_message.Message):
    __slots__ = ("consumer_id", "consumer_version", "schema_id", "schema_version", "environment", "registered_at", "last_validated_at", "used_endpoints")
    CONSUMER_ID_FIELD_NUMBER: _ClassVar[int]
    CONSUMER_VERSION_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
    REGISTERED_AT_FIELD_NUMBER: _ClassVar[int]
    LAST_VALIDATED_AT_FIELD_NUMBER: _ClassVar[int]
    USED_ENDPOINTS_FIELD_NUMBER: _ClassVar[int]
    consumer_id: str
    consumer_version: str
    schema_id: str
    schema_version: str
    environment: str
    registered_at: int
    last_validated_at: int
    used_endpoints: _containers.RepeatedCompositeFieldContainer[EndpointUsage]
    def __init__(self, consumer_id: _Optional[str] = ..., consumer_version: _Optional[str] = ..., schema_id: _Optional[str] = ..., schema_version: _Optional[str] = ..., environment: _Optional[str] = ..., registered_at: _Optional[int] = ..., last_validated_at: _Optional[int] = ..., used_endpoints: _Optional[_Iterable[_Union[EndpointUsage, _Mapping]]] = ...) -> None: ...

class EndpointUsage(_message.Message):
    __slots__ = ("method", "path", "used_fields")
    METHOD_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    USED_FIELDS_FIELD_NUMBER: _ClassVar[int]
    method: str
    path: str
    used_fields: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, method: _Optional[str] = ..., path: _Optional[str] = ..., used_fields: _Optional[_Iterable[str]] = ...) -> None: ...

class RegisterConsumerRequest(_message.Message):
    __slots__ = ("consumer_id", "consumer_version", "schema_id", "schema_version", "environment", "used_endpoints")
    CONSUMER_ID_FIELD_NUMBER: _ClassVar[int]
    CONSUMER_VERSION_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
    USED_ENDPOINTS_FIELD_NUMBER: _ClassVar[int]
    consumer_id: str
    consumer_version: str
    schema_id: str
    schema_version: str
    environment: str
    used_endpoints: _containers.RepeatedCompositeFieldContainer[EndpointUsage]
    def __init__(self, consumer_id: _Optional[str] = ..., consumer_version: _Optional[str] = ..., schema_id: _Optional[str] = ..., schema_version: _Optional[str] = ..., environment: _Optional[str] = ..., used_endpoints: _Optional[_Iterable[_Union[EndpointUsage, _Mapping]]] = ...) -> None: ...

class RegisterConsumerResponse(_message.Message):
    __slots__ = ("success", "message", "consumer")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CONSUMER_FIELD_NUMBER: _ClassVar[int]
    success: bool
    message: str
    consumer: ConsumerInfo
    def __init__(self, success: bool = ..., message: _Optional[str] = ..., consumer: _Optional[_Union[ConsumerInfo, _Mapping]] = ...) -> None: ...

class ListConsumersRequest(_message.Message):
    __slots__ = ("schema_id", "environment")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    environment: str
    def __init__(self, schema_id: _Optional[str] = ..., environment: _Optional[str] = ...) -> None: ...

class ListConsumersResponse(_message.Message):
    __slots__ = ("consumers",)
    CONSUMERS_FIELD_NUMBER: _ClassVar[int]
    consumers: _containers.RepeatedCompositeFieldContainer[ConsumerInfo]
    def __init__(self, consumers: _Optional[_Iterable[_Union[ConsumerInfo, _Mapping]]] = ...) -> None: ...

class DeregisterConsumerRequest(_message.Message):
    __slots__ = ("consumer_id", "schema_id", "environment")
    CONSUMER_ID_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
    consumer_id: str
    schema_id: str
    environment: str
    def __init__(self, consumer_id: _Optional[str] = ..., schema_id: _Optional[str] = ..., environment: _Optional[str] = ...) -> None: ...

class DeregisterConsumerResponse(_message.Message):
    __slots__ = ("success", "message")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    success: bool
    message: str
    def __init__(self, success: bool = ..., message: _Optional[str] = ...) -> None: ...

class CanIDeployRequest(_message.Message):
    __slots__ = ("schema_id", "new_version", "environment")
    SCHEMA_ID_FIELD_NUMBER: _ClassVar[int]
    NEW_VERSION_FIELD_NUMBER: _ClassVar[int]
    ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
    schema_id: str
    new_version: str
    environment: str
    def __init__(self, schema_id: _Optional[str] = ..., new_version: _Optional[str] = ..., environment: _Optional[str] = ...) -> None: ...

class CanIDeployResponse(_message.Message):
    __slots__ = ("safe_to_deploy", "summary", "breaking_changes", "affected_consumers")
    SAFE_TO_DEPLOY_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    BREAKING_CHANGES_FIELD_NUMBER: _ClassVar[int]
    AFFECTED_CONSUMERS_FIELD_NUMBER: _ClassVar[int]
    safe_to_deploy: bool
    summary: str
    breaking_changes: _containers.RepeatedCompositeFieldContainer[BreakingChange]
    affected_consumers: _containers.RepeatedCompositeFieldContainer[ConsumerImpact]
    def __init__(self, safe_to_deploy: bool = ..., summary: _Optional[str] = ..., breaking_changes: _Optional[_Iterable[_Union[BreakingChange, _Mapping]]] = ..., affected_consumers: _Optional[_Iterable[_Union[ConsumerImpact, _Mapping]]] = ...) -> None: ...

class ConsumerImpact(_message.Message):
    __slots__ = ("consumer_id", "consumer_version", "current_schema_version", "environment", "will_break", "relevant_changes")
    CONSUMER_ID_FIELD_NUMBER: _ClassVar[int]
    CONSUMER_VERSION_FIELD_NUMBER: _ClassVar[int]
    CURRENT_SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    ENVIRONMENT_FIELD_NUMBER: _ClassVar[int]
    WILL_BREAK_FIELD_NUMBER: _ClassVar[int]
    RELEVANT_CHANGES_FIELD_NUMBER: _ClassVar[int]
    consumer_id: str
    consumer_version: str
    current_schema_version: str
    environment: str
    will_break: bool
    relevant_changes: _containers.RepeatedCompositeFieldContainer[BreakingChange]
    def __init__(self, consumer_id: _Optional[str] = ..., consumer_version: _Optional[str] = ..., current_schema_version: _Optional[str] = ..., environment: _Optional[str] = ..., will_break: bool = ..., relevant_changes: _Optional[_Iterable[_Union[BreakingChange, _Mapping]]] = ...) -> None: ...
