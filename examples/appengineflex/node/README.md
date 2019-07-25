# Berglas App Engine (Flex) Example - Node

This guide assumes you have followed the [setup instructions][setup] in the
README. Specifically, it is assumed that you have created a project, Cloud
Storage bucket, and Cloud KMS key.

[setup]: https://github.com/GoogleCloudPlatform/berglas#setup

1. Make sure you are in the `examples/appengineflex/node` folder before continuing!

1. Export the environment variables for your configuration:

    ```text
    export PROJECT_ID=my-project
    export BUCKET_ID=my-bucket
    export KMS_KEY=projects/${PROJECT_ID}/locations/${KMS_LOCATION}/keyRings/${KMS_KEYRING}/cryptoKeys/${KMS_CRYPTO_KEY}
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
    export SA_EMAIL=${PROJECT_ID}@appspot.gserviceaccount.com
    ```

1. Grant the service account access to the secrets:

    ```text
    berglas grant ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas grant ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```

1. Build a container using Cloud Build and publish it to Container Registry:

    ```text
    gcloud builds submit \
      --project ${PROJECT_ID} \
      --tag gcr.io/${PROJECT_ID}/berglas-example-node:0.0.1 \
      .
    ```

1. Create GAE environment:

    ```text
    echo -en "env_variables:\n\
  API_KEY: berglas://${BUCKET_ID}/api-key\n\
  TLS_KEY: berglas://${BUCKET_ID}/tls-key?destination=tempfile\n\
" > env.yaml
    ```

1. Deploy the container on GAE:

    ```text
    gcloud app deploy \
      --project ${PROJECT_ID} \
      --image-url gcr.io/${PROJECT_ID}/berglas-example-node:0.0.1
    ```

1. Access the service:

    ```text
    curl $(gcloud app services browse berglas-example-node --no-launch-browser --project ${PROJECT_ID} --format 'value(url)')
    ```

1. (Optional) Cleanup the deployment:

    ```text
    gcloud app services delete berglas-example-node \
      --quiet \
      --project ${PROJECT_ID}

    IMAGE=gcr.io/${PROJECT_ID}/berglas-example-node
    for DIGEST in $(gcloud container images list-tags ${IMAGE} --format='get(digest)'); do
      gcloud container images delete --quiet --force-delete-tags "${IMAGE}@${DIGEST}"
    done
    ```

1. (Optional) Revoke access to the secrets:

    ```text
    berglas revoke ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas revoke ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```
