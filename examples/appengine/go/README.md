# Berglas App Engine (Standard) Example - Go

This guide assumes you have followed the [setup instructions][setup] in the
README. Specifically, it is assumed that you have created a project, Cloud
Storage bucket, and Cloud KMS key.

[setup]: https://github.com/GoogleCloudPlatform/berglas#setup

1. Make sure you are in the `examples/appengine/go` folder before continuing!

1. Export the environment variables for your configuration:

    Using Secret Manager storage:

    ```text
    export PROJECT_ID=my-project
    ```

    Using Cloud Storage storage:

    ```text
    export PROJECT_ID=my-project
    export BUCKET_ID=my-bucket
    export KMS_KEY=projects/${PROJECT_ID}/locations/global/keyRings/berglas/cryptoKeys/berglas-key
    ```

1. Enable the required services:

    ```text
    gcloud services enable --project ${PROJECT_ID} \
      appengine.googleapis.com
    ```

1. Create two secrets using the `berglas` CLI (see README for installation
instructions):

    Using Secret Manager storage:

    ```text
    berglas create sm://${PROJECT_ID}/api-key "xxx-yyy-zzz"
    ```

    ```text
    berglas create sm://${PROJECT_ID}/tls-key "=== BEGIN RSA PRIVATE KEY..."
    ```

    Using Cloud Storage storage:

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
    export SA_EMAIL=${PROJECT_ID}@appspot.gserviceaccount.com
    ```

1. Grant the service account access to the secrets:

    Using Secret Manager storage:

    ```text
    berglas grant sm://${PROJECT_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas grant sm://${PROJECT_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```

    Using Google Cloud storage:

    ```text
    berglas grant ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas grant ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```

1. Create environment:

    Using Secret Manager storage:
    
    ```text
    cat > env.yaml <<EOF
    env_variables:
      GO111MODULE: on
      API_KEY: sm://${PROJECT_ID}/api-key 
      TLS_KEY: sm://${PROJECT_ID}/tls-key?destination=tempfile
    EOF
    ```

    Using Google Cloud storage:
    
    ```text
    cat > env.yaml <<EOF
    env_variables:
      GO111MODULE: on
      API_KEY: berglas://${BUCKET_ID}/api-key
      TLS_KEY: berglas://${BUCKET_ID}/tls-key?destination=tempfile
    EOF
    ```

1. Deploy the app on GAE:

    ```text
    gcloud app deploy \
      --project ${PROJECT_ID} \
      --quiet
    ```

1. Access the service:

    ```text
    curl $(gcloud app services browse berglas-example-go --no-launch-browser --project ${PROJECT_ID} --format 'value(url)')
    ```

1. (Optional) Cleanup the deployment:

    ```text
    gcloud app services delete berglas-example-go \
      --quiet \
      --project ${PROJECT_ID}
    ```

1. (Optional) Revoke access to the secrets:

    Using Secret Manager storage:

    ```text
    berglas revoke sm://${PROJECT_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas revoke sm://${PROJECT_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```

    Using Cloud Storage storage:

    ```text
    berglas revoke ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas revoke ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```
