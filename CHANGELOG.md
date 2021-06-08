# Berglas Changelog

All notable changes to Berglas will be documented in this file. This file is maintained by humans and is therefore subject to error.

## [0.6.0]
### Breaking
- repo: rename "master" branch to "main"
- core: require Go 1.16+

### Changed
- core: updated all dependencies to their latest versions
- core: preserve file extension when using a tempfile
- core: truncate an existing file before writing
- core: stop receiving signals when terminating
- docs: document Certificate Authority (CA requirements
- docs: add Secret Manager example to AppEngine
- k8s: updated example to use Go 1.13
- k8s: updated example to use cloud-platform scope (required for Secret Manager)
- k8s: added `--allow-unauthenticated` command to documentation

## [0.5.1]
### Changed
- cli: deprecate `--generations` in favor of hash-based syntax

## [0.5.0] 2019-12-13
### Breaking
- core: drop `ResolveValue` since it is no longer needed

### Added
- core: support for Secret Manager
- cli: `migrate` command migrates Berglas secrets to Secret Manager

## [0.4.0] 2019-12-05
### Breaking
- core: drop runtime environment lookup and associated API methods (see #87 for
  details)

### Changed
- core: do not attempt to chmod on windows and plan9
- docs: various updates for products that have gone GA

## [0.3.0] 2019-11-13
### Breaking
- cli: remove local command short flags, use long-form flags instead, short
  flags will be reserved for global flags moving forward

### Added
- core: add structured logging support (cli default is "text" and sdk default is
  "json")
- core: `DetectRuntimeEnvironment` is now available on the client too
- cli: add --log-format and --log-level flags for controlling output

### Changed
- core: fix nil pointer when finalizing object writes

## [0.2.1] 2019-10-11
### Changed
- core: resolve value uses the passed value instead of looking it up from the
  environment
- core: retry when setting GCS IAM permissions
- core: retry when setting KMS IAM permissions
- core: allow accessing a specific version of a secret

## [0.2.0] 2019-08-01
### Breaking
- cli: drop `version` command in favor of `--version` flag
- core: create will now return an error against an existing secret - use update
  instead

### Added
- core: add new `Read` API for returning the plaintext secret and metadata about
  the storage object
- core: retry certain IAM functions due to eventual consistency
- cli: `edit` command for editing a secret in a local editor
- cli: `update` command for updating an existing secret

### Changed
- auto: [security] do not trust the environment variables
- cli: `list` command now outputs in a table with version and timestamp
- cli: standardized exit codes - see README for more information
- core: delete all storage versions when deleting a secret

## [0.1.4] 2019-07-26
### Breaking
- core: create now returns a respobse struct
- core: list returns a struct with a list member instead of a raw list

### Added
- core: support multiple secret versions through GCS generation
- core: support for Google App Engine (GAE) flex and standard environments

## [0.1.3] 2019-06-18
### Changed
- cli: also allow `berglas://` prefixes
- core: properly convert to seconds for KMS rotation period during bootstrap
- core: support multiple containers being returned from the v1alpha API
- doc: update to match auto-generated KMS keyring

## [0.1.2] 2019-05-17
### Changed
- core: update dependencies to latest version
- dist: mark published binaries as executable
- pkg/auto: Panic on error. The former behavior of logging but not throwing an
  error can be restored by setting the environment variable
  `BERGLAS_CONTINUE_ON_ERROR` to `true`.

## [0.1.1] 2019-04-25
### Added
- pkg/auto: Retry transient errors
- pkg/retry: Add package for handling retries

## [0.1.0] - 2019-04-22
### Added
- Initial release
