## Purpose

Protect the server's memory from oversized HTTP request bodies by bounding how
much JSON and multipart upload content is buffered in memory per request,
while keeping every valid request's outcome unchanged.

## ADDED Requirements

### Requirement: Oversized JSON request bodies are rejected

The API SHALL reject JSON requests whose body exceeds the configured limit
(default 2 MiB) without processing or persisting the request. Endpoints that
read the request body directly SHALL respond with `413` and a Spanish JSON
error; endpoints using standard JSON binding SHALL respond with the binding
error status. Non-multipart request bodies are capped at read time; multipart
uploads are exempt (they keep their own per-endpoint limits).

#### Scenario: JSON body over the limit

- **WHEN** a client POSTs a JSON body larger than the configured limit
- **THEN** the request is rejected with a 4xx response
- **AND** no resource is created or modified

#### Scenario: JSON body under the limit

- **WHEN** a client POSTs a JSON body within the limit
- **THEN** the request is processed exactly as before this change

#### Scenario: Multipart uploads are not affected by the JSON limit

- **WHEN** a client uploads a `multipart/form-data` request to an upload
  endpoint
- **THEN** the JSON body limit is not applied to that request

### Requirement: The JSON body limit is configurable

The limit SHALL be configurable through the `MAX_JSON_BODY_BYTES` environment
variable with a default of 2 MiB.

#### Scenario: Custom limit via environment

- **WHEN** `MAX_JSON_BODY_BYTES` is set at startup
- **THEN** requests larger than that value are rejected and the default is not
  used

### Requirement: Multipart parsing buffers a bounded amount per request

Every multipart upload path (project drawings, project/lot photos, contract
documents, payment receipts, branding logo, profile picture, historical and
project CSV imports) SHALL keep at most 16 MiB of file content in memory per
request; content beyond that SHALL be handled outside memory. The accepted
total request size and the per-file size limits SHALL NOT change, and
successful uploads SHALL store the complete file and respond as before.

#### Scenario: Large multi-file gallery upload still succeeds

- **WHEN** a seller or admin uploads several photos whose combined size exceeds
  the memory bound but stays within the endpoint's body and per-file limits
- **THEN** every accepted file is stored in full and the response lists them
  as before

#### Scenario: Receipt upload unchanged

- **WHEN** a user uploads a receipt within the 10 MB per-file limit
- **THEN** the upload succeeds and the receipt is saved unchanged
