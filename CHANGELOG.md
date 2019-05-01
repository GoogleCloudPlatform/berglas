# Berglas Changelog

All notable changes to Berglas will be documented in this file. This file is maintained by humans and is therefore subject to error.

## [0.1.2] Unreleased
### Changed
- pkg/auto: Panic on error. The former behavior of logging but not throwing an
  error can be restored by setting the environment variable
  `BERGLAS_CONTINUE_ON_ERROR` to `true`.
- dist: mark published binaries are executable

## [0.1.1] 2019-04-25
### Added
- pkg/auto: Retry transient errors
- pkg/retry: Add package for handling retries

## [0.1.0] - 2019-04-22
### Added
- Initial release
