# Berglas Cloud Run Example

Berglas automatically downloads and decrypts referenced secrets at boot time on
Google Cloud Run.

1. Create a Google Cloud project or use an existing one:

    ```text
    $ export PROJECT_ID=your-project-id
    ```

1. Enable the necessary Google Cloud APIs. This only needs to be done once per
project:

    ```text
    $ gcloud services enable --project $PROJECT_ID \
        cloudbuild.googleapis.com \
        cloudkms.googleapis.com \
        run.googleapis.com \
        storage-api.googleapis.com \
        storage-component.googleapis.com
    ```

1. Create a Cloud Storage bucket or use an existing one:

    ```text
    $ export BUCKET=your-bucket-id-here
    ```

    ```text
    $ gsutil mb -p $PROJECT_ID gs://$BUCKET
    ```

    For more details on how to secure the bucket, see the README.

1. Create a Cloud KMS key or use an existing one:

    ```text
    $ export KMS_LOCATION=us-east4
    $ export KMS_KEYRING=my-keyring
    $ export KMS_CRYPTO_KEY=my-key
    ```

    ```text
    $ gcloud kms keyrings create $KMS_KEYRING \
        --project $PROJECT_ID \
        --location $KMS_LOCATION
    ```

    ```text
    $ gcloud kms keys create $KMS_CRYPTO_KEY \
        --project $PROJECT_ID \
        --location $KMS_LOCATION \
        --keyring $KMS_KEYRING \
        --purpose encryption
    ```

1. Create two secrets using the `berglas` CLI (see README for installation
instructions):

    ```text
    $ berglas create $BUCKET/api-key xxx-yyy-zzz \
        --key projects/$PROJECT_ID/locations/$KMS_LOCATION/keyRings/$KMS_KEYRING/cryptoKeys/$KMS_CRYPTO_KEY
    ```

    ```text
    $ berglas create $BUCKET/tls-key "=== BEGIN RSA PRIVATE KEY..." \
        --key projects/$PROJECT_ID/locations/$KMS_LOCATION/keyRings/$KMS_KEYRING/cryptoKeys/$KMS_CRYPTO_KEY
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
        --tag gcr.io/$PROJECT_ID/berglas-example:0.0.1 \
        .
    ```

1. Deploy the container on Cloud Run:

    ```text
    $ gcloud beta run deploy berglas-example \
        --project $PROJECT_ID \
        --region us-central1 \
        --image gcr.io/$PROJECT_ID/berglas-example:0.0.1 \
        --memory 1G \
        --concurrency 10 \
        --set-env-vars "API_KEY=berglas://$BUCKET/api-key,TLS_KEY=berglas://$BUCKET/tls-key?destination=tempfile" \
        --allow-unauthenticated
    ```

1. Access the function:

    ```text
    $ curl $(gcloud beta run services describe berglas-example --project $PROJECT_ID --region us-central1 --format 'value(status.domain)')
    ```
