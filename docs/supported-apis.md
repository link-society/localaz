# Supported APIs

## Azure Blob Storage

| Operation                        | REST surface                                              |
| -------------------------------- | -------------------------------------------------------- |
| List Containers                  | `GET /{account}?comp=list`                               |
| Create Container                 | `PUT /{account}/{container}?restype=container`           |
| Get Container Properties         | `GET/HEAD /{account}/{container}?restype=container`      |
| Delete Container                 | `DELETE /{account}/{container}?restype=container`        |
| List Blobs (flat + hierarchical) | `GET /{account}/{container}?restype=container&comp=list` |
| Put Blob (block blob)            | `PUT /{account}/{container}/{blob}`                      |
| Put Block                        | `PUT /{account}/{container}/{blob}?comp=block`           |
| Put Block List                   | `PUT /{account}/{container}/{blob}?comp=blocklist`       |
| Get Blob (+ range requests)      | `GET /{account}/{container}/{blob}`                      |
| Get Blob Properties              | `HEAD /{account}/{container}/{blob}`                     |
| Delete Blob                      | `DELETE /{account}/{container}/{blob}`                   |
| Get Service Properties           | `GET /{account}?restype=service&comp=properties`         |

### Supported semantics

- Container and blob metadata (`x-ms-meta-*`).
- Content settings: content type, encoding, language, disposition, cache-control.
- Content-MD5 computation and round-tripping.
- Virtual-directory listing via a delimiter (`BlobPrefix` results).
- Block blob uploads via both the single-shot and staged-block paths, so both
  small and large SDK/CLI uploads work.
- Single-range `GET` requests (`Range` / `x-ms-range`).

### Not yet implemented

- Page blobs and append blobs.
- Shared Key signature verification (the header is accepted but not validated).
- Leases, snapshots, versioning, soft delete, tags.
- SAS token generation/validation.

## Roadmap

- Queue Storage
- Table Storage
- Service Bus (pub/sub)
- Optional Shared Key signature verification
