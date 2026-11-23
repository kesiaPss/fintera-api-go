## Purpose

Reject profile pictures whose pixel dimensions are so large that decoding them
would expand into an uncompressed bitmap far beyond the accepted file size,
before the full decode happens.

## ADDED Requirements

### Requirement: Profile images are size-checked by dimensions before decoding

Profile picture uploads SHALL be validated on their declared pixel dimensions
before the image is fully decoded. An image exceeding 4096 pixels on either
side or 16 million total pixels SHALL be rejected with a Spanish error, and
neither the stored WebP variant nor a thumbnail SHALL be written. Images
within the limits SHALL be processed exactly as before (re-encoded to WebP
with a thumbnail, per the released image pipeline).

#### Scenario: Pixel bomb rejected

- **WHEN** a user uploads an image file that is under the file-size limit but
  declares dimensions above the pixel budget
- **THEN** the request fails with a validation error
- **AND** no file is written to storage

#### Scenario: Normal avatar accepted

- **WHEN** a user uploads a valid JPEG, PNG, or WebP within the dimension limits
- **THEN** the stored WebP picture and its thumbnail are written and the
  response returns their paths as before
