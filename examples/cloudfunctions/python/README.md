# Berglas Cloud Functions Example - Python

This guide assumes you have followed the [setup instructions][setup] in the
README. Specifically, it is assumed that you have created a project, Cloud
Storage bucket, and Cloud KMS key.

[setup]: https://github.com/GoogleCloudPlatform/berglas#setup

1. Make sure you are in the `examples/cloudfunctions/python` folder before
continuing!

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

1. Create a service account which will be assigned to the Cloud Function later:

    ```text
    gcloud iam service-accounts create berglas-service-account \
      --project ${PROJECT_ID} \
      --display-name "berglas Cloud Functions Example"
    ```

    Save the service account email because it will be used later:

    ```text
    export SA_EMAIL=berglas-service-account@${PROJECT_ID}.iam.gserviceaccount.com
    ```

1. Grant the service account access to read the Cloud Function's environment
variables:

    ```text
    gcloud projects add-iam-policy-binding ${PROJECT_ID} \
      --member serviceAccount:${SA_EMAIL} \
      --role roles/cloudfunctions.viewer
    ```

1. Grant the service account access to the secrets:

    ```text
    berglas grant ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas grant ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```

1. Deploy the Cloud Function:

    ```text
    gcloud beta functions deploy berglas-example-python \
      --project ${PROJECT_ID} \
      --region us-central1 \
      --runtime python37 \
      --memory 1G \
      --max-instances 10 \
      --service-account ${SA_EMAIL} \
      --set-env-vars "API_KEY=berglas://${BUCKET_ID}/api-key,TLS_KEY=berglas://${BUCKET_ID}/tls-key?destination=tempfile" \
      --entry-point handler \
      --trigger-http
    ```

1. Make the Cloud Function accessible:

    ```text
    gcloud alpha functions add-iam-policy-binding berglas-example-python \
      --project ${PROJECT_ID} \
      --role roles/cloudfunctions.invoker \
      --member allUsers
    ```

    This example makes the function accessible to everyone, which might not be
    desirable. You can grant finer-grained permissions, but that is not
    discussed in this tutorial.

1. Access the function:

    ```text
    curl $(gcloud beta functions describe berglas-example-python --project ${PROJECT_ID} --format 'value(httpsTrigger.url)')
    ```

1. (Optional) Delete the function:

   ```text
   gcloud functions delete berglas-example-python \
     --project ${PROJECT_ID} \
     --region us-central1
   ```

1. (Optional) Revoke access to the secrets:

    ```text
    berglas revoke ${BUCKET_ID}/api-key --member serviceAccount:${SA_EMAIL}
    berglas revoke ${BUCKET_ID}/tls-key --member serviceAccount:${SA_EMAIL}
    ```
