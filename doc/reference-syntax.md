# Berglas Reference Syntax

This document describes the syntax for referencing a Berglas entity. Most
commonly these references will live in environment variables, but Berglas will
parse these strings at the library level too.

## Syntax

### Storage

```text
berglas://[BUCKET]/[OBJECT]?[OPTIONS]#[GENERATION]
```

- `BUCKET` - name of the Cloud Storage bucket where the secret is stored

- `OBJECT` - name of the secret in the Cloud Storage bucket

- `OPTIONS` - options specified as URL query parameters (see below)

- `GENERATION` - secret generation to access specified as URL fragment. Defaults to latest.

### Secret Manager

```text
sm://[PROJECT]/[NAME]?[OPTIONS]#[VERSION]
```

- `PROJECT` - ID of the Google Cloud project for Secret Manager

- `NAME` - name of the secret in Secret Manager

- `OPTIONS` - options specified as URL query parameters (see below)

- `VERSION` - secret version to access specified as URL fragment. Defaults to "latest".


### Options

- `destination` - when specified as a URL query parameter, this controls how the
  secret is resolved:

    - `tempfile` - resolve the secret and write the contents to a tempfile,
      replacing the environment variable with the path to the tempfile

    - `[PATH]` - resolve the secret and write the contents to the specified file
      path.

- `path` - when this URL query parameter is supplied, the value of the
  secret must be a JSON object. Berglas will deserialize and then
  treat `path` as a [JMESPath](https://jmespath.org/) query to extract
  a single value from the object.

## Examples

Read a Cloud Storage secret:

```text
berglas://my-bucket/my-secret
```

Read a Secret Manager secret:

```text
sm://my-project/my-secret
```

Read a secret into a tempfile:

```text
berglas://my-bucket/path/to/my-secret?destination=tempfile
```

Read a specific generation of a secret:

```text
sm://my-project/my-secret#13
```

Read a path from a secret that is encoded as a JSON object:

```text
sm://my-project/my-secret?path=password
```
