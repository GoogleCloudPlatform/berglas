# Berglas Cloud Functions Example

Berglas automatically downloads and decrypts referenced secrets at function
boot time on Cloud Functions.

1. Create a Google Cloud project or use an existing one:

    ```text
    $ export PROJECT_ID=your-project-id
    ```

1. Enable the necessary Google Cloud APIs. This only needs to be done once per
project:

    ```text
    $ gcloud services enable --project $PROJECT_ID \
        cloudkms.googleapis.com \
        cloudfunctions.googleapis.com \
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

1. Create a new service account which will be assigned to the Cloud Function
later:

    ```text
    $ export SA_NAME=berglas-example
    ```

    ```text
    $ gcloud iam service-accounts create $SA_NAME \
        --project $PROJECT_ID \
        --display-name "berglas Cloud Functions Example"
    ```

    Save the service account email because it will be used later:

    ```text
    $ export SA_EMAIL=$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com
    ```

1. Grant the service account access to read the function's environment
variables:

    ```text
    $ gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member serviceAccount:$SA_EMAIL \
        --role roles/cloudfunctions.viewer
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

1. Vendor dependencies:

   ```text
   $ go mod vendor
   ```

1. Deploy the function:

    ```text
    $ gcloud beta functions deploy berglas-example \
        --project $PROJECT_ID \
        --region us-central1 \
        --runtime go111 \
        --memory 1G \
        --max-instances 10 \
        --service-account $SA_EMAIL \
        --set-env-vars "API_KEY=berglas://$BUCKET/api-key,TLS_KEY=berglas://$BUCKET/tls-key?destination=tempfile" \
        --entry-point F \
        --trigger-http
    ```

1. (Optional) Make the function accessible:

    ```text
    $ gcloud alpha functions add-iam-policy-binding berglas-example \
        --project $PROJECT_ID \
        --role roles/cloudfunctions.invoker \
        --member allUsers
    ```

    This example makes the function accessible to everyone, which might not be
    desirable. You can grant finer-grained permissions, but that is not
    discussed in this tutorial.

1. Access the function:

    ```text
    $ curl $(gcloud beta functions describe berglas-example --project $PROJECT_ID --format 'value(httpsTrigger.url)')
    ```

1. (Optional) Delete the function:

   ```text
   $ gcloud functions delete berglas-example \
       --project $PROJECT_ID \
       --region us-central1
   ```
