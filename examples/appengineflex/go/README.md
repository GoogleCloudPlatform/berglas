# Berglas App Engine (Flex) Example - Go

This guide assumes you have followed the [setup instructions][setup] in the
README. Specifically, it is assumed that you have created a project, Cloud
Storage bucket, and Cloud KMS key.

[setup]: https://github.com/GoogleCloudPlatform/berglas#setup

1. Make sure you are in the `examples/appengineflex/go` folder before continuing!

1. Enable the App Engine Flex API (this only needs to be done once per project):

    ```text
    gcloud services enable --project ${PROJECT_ID} \
      appengineflex.googleapis.com
    ```

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
    export SA_EMAIL=service-${PROJECT_NUMBER}@gae-api-prod.google.com.iam.gserviceaccount.com
    ```

1. Grant the service account access to the secrets:

    ```text
    berglas grant ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas grant ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```

1. Vendor the dependencies:

    ```text
    go mod vendor
    ```

1. Build a container using Cloud Build and publish it to Container Registry:

    ```text
    gcloud builds submit \
      --project ${PROJECT_ID} \
      --tag gcr.io/${PROJECT_ID}/berglas-example-go:0.0.1 \
      .
    ```

1. Create environment:

    ```text
    cat > env.yaml <<EOF
    env_variables:
      API_KEY: berglas://${BUCKET_ID}/api-key
      TLS_KEY: berglas://${BUCKET_ID}/tls-key?destination=tempfile
    EOF
    ```

1. Deploy the container on GAE:

    ```text
    gcloud app deploy \
      --project ${PROJECT_ID} \
      --image-url gcr.io/${PROJECT_ID}/berglas-example-go:0.0.1 \
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

    ```text
    IMAGE=gcr.io/${PROJECT_ID}/berglas-example-go
    for DIGEST in $(gcloud container images list-tags ${IMAGE} --format='get(digest)'); do
      gcloud container images delete --quiet --force-delete-tags "${IMAGE}@${DIGEST}"
    done
    ```

1. (Optional) Revoke access to the secrets:

    ```text
    berglas revoke ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas revoke ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```
