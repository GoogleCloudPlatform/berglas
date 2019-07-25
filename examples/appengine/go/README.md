# Berglas App Engine (Standard) Example - Go

This guide assumes you have followed the [setup instructions][setup] in the
README. Specifically, it is assumed that you have created a project, Cloud
Storage bucket, and Cloud KMS key.

[setup]: https://github.com/GoogleCloudPlatform/berglas#setup

1. Make sure you are in the `examples/appengine/go` folder before continuing!

1. Export the environment variables for your configuration:

    ```text
    export PROJECT_ID=my-project
    export BUCKET_ID=my-bucket
    export KMS_KEY=projects/${PROJECT_ID}/locations/global/keyRings/berglas/cryptoKeys/berglas-key
    ```

1. Create two secrets using the `berglas` CLI (see README for installation
instructions):

    ```text
    berglas create ${BUCKET_ID}/api-key "xxx-yyy-zzz" \
      --key ${KMS_KEY}
    ```

    ```text
    berglas create ${BUCKET_ID}/tls-key "=== BEGIN RSA PRIVATE KEY..." \
      --key ${KMS_KEY}
    ```

1. Get the App Engine service account email:

    ```text
    PROJECT_NUMBER=$(gcloud projects describe ${PROJECT_ID} --format 'value(projectNumber)')
    export SA_EMAIL=${PROJECT_NUMBER}-compute@developer.gserviceaccount.com
    ```

1. Grant the service account access to read the App Engine deployment's
environment variables:

    ```text
    gcloud projects add-iam-policy-binding ${PROJECT_ID} \
      --member serviceAccount:${SA_EMAIL} \
      --role roles/appengine.appViewer
    ```

1. Grant the service account access to the secrets:

    ```text
    berglas grant ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas grant ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```

1. Create environment:

    ```text
    echo -en "env_variables:\n\
  API_KEY: berglas://${BUCKET_ID}/api-key\n\
  TLS_KEY: berglas://${BUCKET_ID}/tls-key?destination=tempfile\n\
" > env.yaml
    ```

1. Deploy the app on GAE:

    ```text
    gcloud app deploy \
      --project ${PROJECT_ID}
    ```

1. Access the service:

    ```text
    curl $(gcloud app services browse berglas-example-go --no-launch-browser --project ${PROJECT_ID} --format 'value(url)')
    curl $(gcloud app describe --project "${PROJECT_ID}" --format 'value(defaultHostname)')
    ```

1. (Optional) Cleanup the deployment:

    ```text
    gcloud app services delete berglas-example-go \
      --quiet \
      --project ${PROJECT_ID}
    ```

1. (Optional) Revoke access to the secrets:

    ```text
    berglas revoke ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas revoke ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```
