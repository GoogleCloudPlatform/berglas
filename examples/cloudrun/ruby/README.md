# Berglas Ruby Cloud Run Example

Berglas automatically downloads and decrypts referenced secrets at boot time on
Google Cloud Run.

1. Setup a berglas secrets store using the instructions in the README. Export
the environment variables for your configuration:

    ```text
    $ export PROJECT_ID=my-project
    $ export BUCKET=my-bucket
    $ export KMS_KEY=projects/my-project/locations/my-location/keyRings/my-keyring/cryptoKeys/my-key
    ```

1. Create two secrets using the `berglas` CLI (see README for installation
instructions):

    ```text
    $ berglas create $BUCKET/api-key xxx-yyy-zzz \
        --key $KMS_KEY
    ```

    ```text
    $ berglas create $BUCKET/tls-key "=== BEGIN RSA PRIVATE KEY..." \
        --key $KMS_KEY
    ```

1. Find the Cloud Run service account:

    ```text
    $ PROJECT_NUMBER=$(gcloud projects describe $PROJECT_ID --format 'value(projectNumber)')
    $ export SA_EMAIL=$PROJECT_NUMBER-compute@developer.gserviceaccount.com
    ```

1. Grant the service account access to read the Cloud Run deployment's
environment variables:

    ```text
    $ gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member serviceAccount:$SA_EMAIL \
        --role roles/run.viewer
    ```

1. Grant the service account access to the Cloud Storage bucket objects:

    ```text
    $ gsutil iam ch serviceAccount:${SA_EMAIL}:roles/storage.legacyObjectReader gs://${BUCKET}/api-key
    ```

    ```text
    $ gsutil iam ch serviceAccount:${SA_EMAIL}:roles/storage.legacyObjectReader gs://${BUCKET}/tls-key
    ```

1. Grant the service account access to use the KMS key:

    ```text
    $ gcloud kms keys add-iam-policy-binding $KMS_CRYPTO_KEY \
        --project $PROJECT_ID \
        --keyring $KMS_KEYRING \
        --location $KMS_LOCATION \
        --member serviceAccount:$SA_EMAIL \
        --role roles/cloudkms.cryptoKeyDecrypter
    ```

1. Build a container using Cloud Build and publish it to Container Registry:

    ```text
    $ gcloud builds submit \
        --project $PROJECT_ID \
        --tag gcr.io/$PROJECT_ID/berglas-example:ruby-0.0.1 \
        .
    ```

1. Deploy the container on Cloud Run:

    ```text
    $ gcloud beta run deploy berglas-example-ruby \
        --project $PROJECT_ID \
        --region us-central1 \
        --image gcr.io/$PROJECT_ID/berglas-example:ruby-0.0.1 \
        --memory 1G \
        --concurrency 10 \
        --set-env-vars "API_KEY=berglas://$BUCKET/api-key,TLS_KEY=berglas://$BUCKET/tls-key?destination=tempfile" \
        --allow-unauthenticated
    ```

1. Access the function:

    ```text
    $ curl $(gcloud beta run services describe berglas-example-ruby --project $PROJECT_ID --region us-central1 --format 'value(status.domain)')
    ```
