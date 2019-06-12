# Kubernetes Mutating Webhook

To consume Berglas secrets from Kubernetes, you can deploy a
`MutatingWebhookConfiguration`. This mutation will inspect all pods admitted
into the cluster and adjust their manifest to pull secrets from Berglas.

The webhook must be deployed and accessible by the Kubernetes API server. You
can deploy the webhook via knative, Cloud Run, Cloud Functions, Heroku, Cloud
Foundry, etc, so long as it offers a TLS endpoint accessible by the Kubernetes
API server.


## Deployment

One of the easiest ways to deploy the mutation webhook is using Cloud Functions.
To deploy on Cloud Functions:

1. Enable the Cloud Functions API (this only needs to be done once per project):

    ```text
    gcloud services enable --project ${PROJECT_ID} \
      cloudfunctions.googleapis.com
    ```

1. Set environment variables (replace with your values):

    ```text
    export PROJECT_ID=my-project
    ```

1. Ensure you are in the `kubernetes/` directory

1. Deploy the mutation webhook:

    ```text
    gcloud functions deploy berglas-secrets-webhook \
      --project ${PROJECT_ID} \
      --runtime go111 \
      --entry-point F \
      --trigger-http
    ```

1. Extract the Cloud Function URL:

    ```text
    ENDPOINT=$(gcloud functions describe berglas-secrets-webhook --project ${PROJECT_ID} --format 'value(httpsTrigger.url)')
    ```

1. Register the webhook with this URL:

    ```text
    sed "s|REPLACE_WITH_YOUR_URL|$ENDPOINT|" deploy/webhook.yaml | kubectl apply -f -
    ```

1. (Optional) Verify the webhook is running:

    ```text
    kubectl get mutatingwebhookconfiguration

    NAME
    berglas-secrets-webhook
    ```


## Permissions

Either the pods or the Kubernetes cluster needs to be able to authenticate to
Google Cloud using IAM with Default Application Credentials. See the
authentication section of the main README for more details.

On Google Cloud, it is strongly recommended that you have a dedicated service account for the GKE cluster. For example:

1. Export your configuration variables:

    ```text
    PROJECT_ID=berglas-test
    BUCKET_ID=berglas-test-secrets
    KMS_KEY=projects/${PROJECT_ID}/locations/global/keyRings/berglas/cryptoKeys/berglas-key
    ```

1. Create a service account:

    ```text
    gcloud iam service-accounts create berglas-k8s \
      --project ${PROJECT_ID} \
      --display-name "Berglas K8S account"
    ```

    ```text
    export SA_EMAIL=berglas-k8s@${PROJECT_ID}.iam.gserviceaccount.com
    ```

1. Grant the service account the required GKE permissions:

    ```text
    gcloud projects add-iam-policy-binding ${PROJECT_ID} \
      --member "serviceAccount:${SA_EMAIL}" \
      --role roles/logging.logWriter
    ```

    ```text
    gcloud projects add-iam-policy-binding ${PROJECT_ID} \
      --member "serviceAccount:${SA_EMAIL}" \
      --role roles/monitoring.metricWriter
    ```

    ```text
    gcloud projects add-iam-policy-binding ${PROJECT_ID} \
      --member "serviceAccount:${SA_EMAIL}" \
      --role roles/monitoring.viewer
    ```

1. Grant Berglas permissions:

    ```text
    gsutil iam ch serviceAccount:${SA_EMAIL}:legacyObjectReader gs://${BUCKET_ID}/api-key
    ```

    ```text
    gsutil iam ch serviceAccount:${SA_EMAIL}:legacyObjectReader gs://${BUCKET_ID}/tls-key
    ```

    ```text
    gcloud kms keys add-iam-policy-binding ${KMS_KEY} \
      --member serviceAccount:${SA_EMAIL} \
      --role roles/cloudkms.cryptoKeyDecrypter
    ```

1. Create the GKE cluster with the attached service account:

    ```text
    gcloud container clusters create berglas-k8s-test \
      --project ${PROJECT_ID} \
      --region us-east1 \
      --num-nodes 1 \
      --machine-type n1-standard-2 \
      --service-account ${SA_EMAIL} \
      --no-issue-client-certificate \
      --no-enable-basic-auth \
      --enable-autoupgrade \
      --metadata disable-legacy-endpoints=true
    ```


## Example

After deploying the webhook, test your configuration by deploying a sample application.


1. Update `deploy/sample.yaml` to refer to your secret:

1. Deploy it:

    ```text
    kubectl apply -f deploy/sample.yaml
    ```


## Limitations

The mutator requires that containers specify a `command` in their manifest. If a
container requests Berglas secrets and does not specify a `command`, the mutator
will log an error and not mutate the spec.
