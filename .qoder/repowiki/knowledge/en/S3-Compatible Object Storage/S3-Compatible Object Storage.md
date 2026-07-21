---
kind: external_dependency
name: S3-Compatible Object Storage
slug: s3-object-storage
category: external_dependency
category_hints:
    - vendor_identity
scope:
    - '**'
---

Object storage used for archiving raw API responses from weather providers. Stores complete JSON payloads with structured keys (forecasts/{provider}/{location}/{date}/{snapshot_id}.json) for audit trail and reproducibility. Raw data retained for 2 years then archived per retention policy.