# Berglas Cloud Functions Example - Go

This guide assumes you have followed the [setup instructions][setup] in the
README. Specifically, it is assumed that you have created a project, Cloud
Storage bucket, and Cloud KMS key.

[setup]: https://github.com/GoogleCloudPlatform/berglas#setup

1. Make sure you are in the `examples/cloudfunctions/go` folder before
continuing!

1. Export the environment variables for your configuration:

    ```text
    export PROJECT_ID=my-project
    export BUCKET=my-bucket
    export KMS_KEY=projects/$PROJECT_ID/locations/$KMS_LOCATION/keyRings/$KMS_KEYRING/cryptoKeys/$KMS_CRYPTO_KEY
    ```

1. Create two secrets using the `berglas` CLI (see README for installation
instructions):

    ```text
    berglas create $BUCKET_ID/api-key "xxx-yyy-zzz" \
      --key $KMS_KEY
    ```

    ```text
    berglas create $BUCKET_ID/tls-key "=== BEGIN RSA PRIVATE KEY..." \
      --key $KMS_KEY
    ```

1. Create a service account which will be assigned to the Cloud Function later:

    ```text
    $ gcloud iam service-accounts create berglas-service-account \
        --project $PROJECT_ID \
        --display-name "berglas Cloud Functions Example"
    ```

    Save the service account email because it will be used later:

    ```text
    export SA_EMAIL=berglas-service-account@$PROJECT_ID.iam.gserviceaccount.com
    ```

1. Grant the service account access to read the Cloud Function's environment
variables:

    ```text
    gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member serviceAccount:$SA_EMAIL \
      --role roles/cloudfunctions.viewer
    ```

1. Grant the service account access to the Cloud Storage bucket objects:

    ```text
    gsutil iam ch serviceAccount:$SA_EMAIL:legacyObjectReader gs://${BUCKET}/api-key
    gsutil iam ch serviceAccount:$SA_EMAIL:legacyObjectReader gs://${BUCKET}/tls-key
    ```

1. Grant the service account access to use the KMS key:

    ```text
    gcloud kms keys add-iam-policy-binding $KMS_KEY \
      --member serviceAccount:$SA_EMAIL \
      --role roles/cloudkms.cryptoKeyDecrypter
    ```

1. Deploy the Cloud Function:

    ```text
    gcloud beta functions deploy berglas-example-go \
      --project $PROJECT_ID \
      --region us-central1 \
      --runtime go111 \
      --memory 1G \
      --max-instances 10 \
      --service-account $SA_EMAIL \
      --set-env-vars "API_KEY=berglas://$BUCKET_ID/api-key,TLS_KEY=berglas://$BUCKET_ID/tls-key?destination=tempfile" \
      --entry-point F \
      --trigger-http
    ```

1. Make the Cloud Function accessible:

    ```text
    gcloud alpha functions add-iam-policy-binding berglas-example-go \
      --project $PROJECT_ID \
      --role roles/cloudfunctions.invoker \
      --member allUsers
    ```

    This example makes the function accessible to everyone, which might not be
    desirable. You can grant finer-grained permissions, but that is not
    discussed in this tutorial.

1. Access the function:

    ```text
    curl $(gcloud beta functions describe berglas-example-go --project $PROJECT_ID --format 'value(httpsTrigger.url)')
    ```

1. (Optional) Delete the function:

   ```text
   gcloud functions delete berglas-example-go \
     --project $PROJECT_ID \
     --region us-central1
   ```
