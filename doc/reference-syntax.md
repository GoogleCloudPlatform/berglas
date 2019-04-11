# Berglas Reference Syntax

This document describes the syntax for referencing a Berglas entity. Most
commonly these references will live in environment variables, but the will parse
these strings at lower levels.

## Syntax

```text
berglas://[BUCKET]/[SECRET]?[OPTIONS]
```

- `BUCKET` - name of the Cloud Storage bucket where the secret is stored

- `SECRET` - name of the secret in the Cloud Storage bucket

- `OPTIONS` - options specified as URL query parameters

### Options

- `destination` - when specified as a URL query parameter, this controls how the
  secret is resolved:

    - `tempfile` - resolve the secret contents to a tempfile, replacing the
      environment variable with the path to the tempfile

    - `[PATH]` - resolve the secret contents and write them to the specified
      file path.

## Examples

Read a secret:

```text
berglas://my-bucket/my-secret
```

Read a secret into a tempfile:

```text
berglas://my-bucket/path/to/my-secret?destination=tempfile
```
