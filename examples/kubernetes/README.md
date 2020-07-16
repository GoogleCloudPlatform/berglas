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
      --allow-unauthenticated \
      --runtime go113 \
      --entry-point F \
      --trigger-http
    ```

    Note: This function does **not** require authentication. It does **not**
    need permission to access secrets. The function purely mutates YAML
    configurations before sending them to the scheduler.

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


## Setup and Usage

Either the pods or the Kubernetes cluster needs to be able to authenticate to
Google Cloud using IAM with Default Application Credentials. See the
authentication section of the main README for more details.

On Google Cloud, it is strongly recommended that you use [Workload
Identity][workload-identity] or have a dedicated service account for the GKE
cluster. For example:

1. Create a service account:

    ```text
    gcloud iam service-accounts create berglas-accessor \
      --project ${PROJECT_ID} \
      --display-name "Berglas secret accessor account"
    ```

    ```text
    export SA_EMAIL=berglas-accessor@${PROJECT_ID}.iam.gserviceaccount.com
    ```

1. Grant the Google Cloud service account permissions to access the secrets you
   require. For example, to grant access to the secret named "my-secret":

    Using Secret Manager storage:

    ```text
    berglas grant sm://${PROJECT_ID}/my-secret --member serviceAccount:$SA_EMAIL
    ```

    Using Cloud Storage storage:

    ```text
    berglas grant ${BUCKET_ID}/my-secret --member serviceAccount:$SA_EMAIL
    ```

1. Create the GKE cluster:

    ```text
    gcloud container clusters create berglas-k8s-test \
      --project ${PROJECT_ID} \
      --region us-east1 \
      --num-nodes 1 \
      --machine-type n1-standard-2 \
      --no-issue-client-certificate \
      --no-enable-basic-auth \
      --enable-autoupgrade \
      --scopes cloud-platform \
      --metadata disable-legacy-endpoints=true
    ```

1. Create a Kubernetes service account:

    ```text
    kubectl create serviceaccount "envserver"
    ```

1. Grant the Kubernetes service account permissions to act as the Google Cloud service account:

    ```text
    gcloud iam service-accounts add-iam-policy-binding \
      --role "roles/iam.workloadIdentityUser" \
      --member "serviceAccount:${PROJECT_ID}.svc.id.goog[default/envserver]" \
      berglas-accessor@${PROJECT_ID}.iam.gserviceaccount.com
    ```

1. Annotate the Kubernetes service account with the name of the Google Cloud service account:

    ```text
    kubectl annotate serviceaccount "envserver" \
      iam.gke.io/gcp-service-account=berglas-accessor@${PROJECT_ID}.iam.gserviceaccount.com
    ```

1. Update `deploy/sample.yaml` to refer to your secret. If you're using Secret
   Manager storage, use the `sm://` prefix. If you're using the Cloud Storage
   storage, use the `berglas://` prefix. See more in the Berglas [reference
   syntax][reference-syntax].

1. Deploy it:

    ```text
    kubectl apply -f deploy/sample.yaml
    ```


## Limitations

The mutator requires that containers specify a `command` in their manifest. If a
container requests Berglas secrets and does not specify a `command`, the mutator
will log an error and not mutate the spec.


[workload-identity]: https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity
[syntax-syntax]: doc/reference-syntax.md
